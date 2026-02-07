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
	"syscall"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"ghost/kernel/internal/adapter"
	pb "ghost/kernel/internal/protocol"
	"ghost/kernel/internal/service"

	_ "modernc.org/sqlite"
)

func main() {
	// Flags
	grpcPort := flag.Int("grpc-port", 50051, "gRPC server port")
	httpPort := flag.Int("http-port", 8080, "HTTP gateway port")
	flag.Parse()

	// 1. Initialize Logger
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	slog.Info("Ghost Kernel Initializing...")

	// 2. Database Setup
	db, err := sql.Open("sqlite", "data/kernel.db")
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}
	defer db.Close()

	// 3. Ensure data directory exists
	if err := os.MkdirAll("data", 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

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

	// 6. Start gRPC Server
	lis, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *grpcPort))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterNervousSystemServer(grpcServer, ghostService)

	// Run gRPC in goroutine
	go func() {
		slog.Info("gRPC Server listening", "port", *grpcPort)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	// 7. Start HTTP Gateway (REST Proxy)
	go func() {
		ctx := context.Background()
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		mux := runtime.NewServeMux()
		opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

		// Register the gateway to talk to the local gRPC server
		endpoint := fmt.Sprintf("localhost:%d", *grpcPort)
		err := pb.RegisterNervousSystemHandlerFromEndpoint(ctx, mux, endpoint, opts)
		if err != nil {
			log.Fatalf("Failed to register gateway: %v", err)
		}

		slog.Info("HTTP Gateway listening", "port", *httpPort)
		if err := http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", *httpPort), mux); err != nil {
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
