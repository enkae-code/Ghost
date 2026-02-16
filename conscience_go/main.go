// Author: Enkae (enkae.dev@pm.me)
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"ghost/kernel/internal/adapter"
	"ghost/kernel/internal/gateway"
	pb "ghost/kernel/internal/protocol"
	"ghost/kernel/internal/service"

	_ "modernc.org/sqlite"
)

func main() {
	// Flags
	grpcPort := flag.Int("grpc-port", 50051, "gRPC server port")
	httpPort := flag.Int("http-port", 8080, "HTTP gateway port")
	ghostPort := flag.Int("ghost-port", 5005, "Ghost TCP Gateway port")
	flag.Parse()

	// 1. Initialize Logger
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	slog.Info("Ghost Kernel Initializing...")

	// 2. Ensure data directory exists
	if err := os.MkdirAll("data", 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// 3. Database Setup
	db, err := sql.Open("sqlite", "data/kernel.db")
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}
	defer db.Close()

	// 4. Initialize Adapters
	actionRepo, err := adapter.NewActionRepository(db)
	if err != nil {
		log.Fatalf("Failed to init ActionRepository: %v", err)
	}
	intentRepo, err := adapter.NewIntentHistoryRepository(db)
	if err != nil {
		log.Fatalf("Failed to init IntentHistoryRepository: %v", err)
	}
	memoryRepo, err := adapter.NewSQLiteRepository("data/kernel.db")
	if err != nil {
		log.Fatalf("Failed to init MemoryRepository: %v", err)
	}
	stateRepo, err := adapter.NewStateRepository(db)
	if err != nil {
		log.Fatalf("Failed to init StateRepository: %v", err)
	}

	// 5. Initialize Logic (The "Brain")
	ghostService := service.NewGhostService(actionRepo, intentRepo, memoryRepo, stateRepo)

	// Load Auth Token for Gateway
	tokenPath := "../ghost.token"
	authToken := ""
	if data, err := os.ReadFile(tokenPath); err == nil {
		authToken = strings.TrimSpace(string(data))
	} else {
		// Try current dir
		if data, err := os.ReadFile("ghost.token"); err == nil {
			authToken = strings.TrimSpace(string(data))
		} else {
			slog.Warn("ghost.token not found, using default/empty token (insecure)")
		}
	}

	// Initialize Legacy Bridge & Gateway
	legacyBridge := service.NewLegacyBridge(ghostService, memoryRepo)
	gatewayServer := gateway.NewServer("", *ghostPort, authToken)
	gatewayServer.SetApprovalHandler(legacyBridge)
	gatewayServer.SetMemoryHandler(legacyBridge)

	// Start Ghost TCP Gateway (Legacy/Brain Support)
	go func() {
		slog.Info("Ghost TCP Gateway starting", "port", *ghostPort)
		if err := gatewayServer.Start(context.Background()); err != nil {
			slog.Error("Ghost Gateway failed", "error", err)
		}
	}()

	// 6. Start gRPC Server
	grpcAddr := fmt.Sprintf("127.0.0.1:%d", *grpcPort)
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterNervousSystemServer(grpcServer, ghostService)

	// Run gRPC in goroutine
	go func() {
		slog.Info("gRPC Server listening", "addr", grpcAddr)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	// 7. Start HTTP Gateway (REST Proxy + Dashboard)
	go func() {
		ctx := context.Background()
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		// API Mux (gRPC Gateway)
		apiMux := runtime.NewServeMux()
		opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

		// Register the gateway to talk to the local gRPC server
		endpoint := fmt.Sprintf("127.0.0.1:%d", *grpcPort)
		err := pb.RegisterNervousSystemHandlerFromEndpoint(ctx, apiMux, endpoint, opts)
		if err != nil {
			log.Fatalf("Failed to register gateway: %v", err)
		}

		// Create a root mux to route API and Static files
		rootMux := http.NewServeMux()

		// 1. Handle API requests via gRPC Gateway
		// Note: The gRPC gateway mux matches patterns defined in proto (e.g. /v1/...)
		rootMux.Handle("/v1/", apiMux)

		// 2. Serve static frontend (build output from apps/landing or apps/dashboard)
		staticDir := "./static"
		cleanStaticDir := filepath.Clean(staticDir)
		fs := http.FileServer(http.Dir(staticDir))

		rootMux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Clean path to prevent directory traversal (URL paths use /)
			cleanURLPath := path.Clean(r.URL.Path)
			// Remove leading slash to ensure filepath.Join treats it as relative
			relPath := strings.TrimPrefix(cleanURLPath, "/")
			// Convert to OS-specific path separators and join
			fullPath := filepath.Join(staticDir, filepath.FromSlash(relPath))

			// Security: Ensure path is within static directory
			// Note: This prevents ".." attacks that filepath.Clean might miss if combined with Join incorrectly
			if !strings.HasPrefix(fullPath, cleanStaticDir) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			// Check if file exists
			info, err := os.Stat(fullPath)
			if err != nil {
				if os.IsNotExist(err) {
					// If file doesn't exist, serve index.html for client-side routing
					http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
					return
				}
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			// If it's a directory, serve index.html (SPA routing) unless strictly asking for a file
			// This prevents directory listing if index.html is missing in subdir
			if info.IsDir() {
				// Optionally check for index.html in subdir, but for SPA usually root index.html handles all
				http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
				return
			}

			// Otherwise serve the file (or let FileServer handle permission/dir logic)
			fs.ServeHTTP(w, r)
		}))

		httpAddr := fmt.Sprintf("127.0.0.1:%d", *httpPort)
		slog.Info("HTTP Gateway listening", "addr", httpAddr)

		// CORS middleware for frontend dashboard
		corsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			allowedOrigins := []string{
				fmt.Sprintf("http://localhost:%d", *httpPort),
				fmt.Sprintf("http://127.0.0.1:%d", *httpPort),
			}

			origin := r.Header.Get("Origin")
			for _, allowed := range allowedOrigins {
				if origin == allowed {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					break
				}
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			rootMux.ServeHTTP(w, r)
		})

		server := &http.Server{
			Addr:              httpAddr,
			Handler:           corsHandler,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      15 * time.Second,
			IdleTimeout:       60 * time.Second,
		}

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to serve HTTP: %v", err)
		}
	}()

	// 8. Wait for Shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	slog.Info("Shutting down...")
	grpcServer.GracefulStop()
}
