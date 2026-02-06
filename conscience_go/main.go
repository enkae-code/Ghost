// Author: Enkae (enkae.dev@pm.me)
package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"ghost/kernel/internal/adapter"
	"ghost/kernel/internal/domain"

	_ "modernc.org/sqlite"
)

// PermissionRequest represents an action that needs approval
type PermissionRequest struct {
	ID             string                 `json:"id"`
	Intent         string                 `json:"intent"`
	TraceID        string                 `json:"trace_id,omitempty"`
	Actions        []Action               `json:"actions"`
	ExpectedWindow string                 `json:"expected_window,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// Action represents a single executable action
type Action struct {
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload"`
}

// PermissionResponse indicates whether the action is approved
type PermissionResponse struct {
	ID         string `json:"id"`
	Approved   bool   `json:"approved"`
	Reason     string `json:"reason,omitempty"`
	ErrorCode  string `json:"error_code,omitempty"`
	TrustScore int    `json:"trust_score,omitempty"`
}

// FocusState represents the current focused window from Sentinel
type FocusState struct {
	WindowName string `json:"name"`
	mu         sync.RWMutex
}

var currentFocus = &FocusState{}
var intentHistoryRepo *adapter.IntentHistoryRepository
var memoryRepo *adapter.SQLiteRepository
var appConfig *Config
var authToken string

// Config represents the application configuration
type Config struct {
	System struct {
		Version     string `json:"version"`
		Environment string `json:"environment"`
		LogLevel    string `json:"log_level"`
		LogFile     string `json:"log_file"`
	} `json:"system"`
	Network struct {
		KernelHost string `json:"kernel_host"`
		KernelPort int    `json:"kernel_port"`
	} `json:"network"`
	Security struct {
		SafeMode        bool     `json:"safe_mode"`
		BlockedKeywords []string `json:"blocked_keywords"`
	} `json:"security"`
}

func main() {
	// Load configuration
	var err error
	appConfig, err = loadConfig()
	if err != nil {
		log.Fatalf("[KERNEL] Failed to load config: %v", err)
	}

	// Setup structured logging
	setupLogging(appConfig.System.LogLevel, appConfig.System.LogFile)

	slog.Info("Ghost Kernel initializing",
		"version", appConfig.System.Version,
		"environment", appConfig.System.Environment)
	fmt.Println("[KERNEL] Ghost Kernel initializing...")
	fmt.Println("[KERNEL] Role: Permission Gate & Focus Verification")

	// Load or generate authentication token
	authToken, err = loadOrGenerateToken()
	if err != nil {
		log.Fatalf("[KERNEL] Failed to initialize auth token: %v", err)
	}
	slog.Info("Authentication enabled", "token_file", "ghost.token")
	fmt.Println("[KERNEL] 🔐 Authentication enabled")

	// Initialize SQLite database
	fmt.Println("[KERNEL] 💾 Initializing SQLite database...")
	db, err := sql.Open("sqlite", "data/kernel.db")
	if err != nil {
		log.Fatalf("[KERNEL] Failed to open database: %v", err)
	}
	defer db.Close()

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		log.Fatalf("[KERNEL] Failed to enable WAL mode: %v", err)
	}

	// Initialize IntentHistory repository
	intentHistoryRepo, err = adapter.NewIntentHistoryRepository(db)
	if err != nil {
		log.Fatalf("[KERNEL] Failed to initialize intent history repository: %v", err)
	}

	// Initialize Memory repository (SQLite)
	memoryRepo, err = adapter.NewSQLiteRepository("data/kernel.db")
	if err != nil {
		log.Fatalf("[KERNEL] Failed to initialize memory repository: %v", err)
	}

	fmt.Println("[KERNEL] ✓ Database initialized.")

	listenAddr := fmt.Sprintf("%s:%d", appConfig.Network.KernelHost, appConfig.Network.KernelPort)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("[KERNEL] Failed to bind to %s: %v", listenAddr, err)
	}
	defer listener.Close()

	slog.Info("Kernel listening", "address", listenAddr)
	fmt.Printf("[KERNEL] 🟢 Listening on %s\n", listenAddr)
	fmt.Println("[KERNEL] Awaiting permission requests from Ghost Brain...")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("[KERNEL] Connection error: %v", err)
			continue
		}

		// Handle each connection in a goroutine for concurrency
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	encoder := json.NewEncoder(conn)

	// Authentication handshake - first message must be auth token
	authenticated := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// If not authenticated, expect auth handshake
		if !authenticated {
			var authMsg struct {
				AuthToken string `json:"auth_token"`
			}
			if err := json.Unmarshal([]byte(line), &authMsg); err != nil {
				slog.Warn("Invalid auth handshake", "error", err.Error())
				fmt.Println("[KERNEL] ❌ Invalid auth handshake, closing connection")
				return
			}

			if authMsg.AuthToken != authToken {
				slog.Warn("Authentication failed", "remote_addr", conn.RemoteAddr().String())
				fmt.Println("[KERNEL] ❌ Authentication failed, closing connection")
				return
			}

			authenticated = true
			slog.Info("Client authenticated", "remote_addr", conn.RemoteAddr().String())
			fmt.Println("[KERNEL] ✓ Client authenticated")
			continue
		}

		// First, try to determine message type
		var messageType struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(line), &messageType); err == nil {
			if messageType.Type == "focus_update" {
				// Handle focus update from Sentinel
				var focusUpdate struct {
					Type       string `json:"type"`
					WindowName string `json:"window_name"`
				}
				if err := json.Unmarshal([]byte(line), &focusUpdate); err != nil {
					log.Printf("[KERNEL] Invalid focus update JSON: %v", err)
					continue
				}

				// Update focus state
				currentFocus.mu.Lock()
				currentFocus.WindowName = focusUpdate.WindowName
				currentFocus.mu.Unlock()
				fmt.Printf("[KERNEL] 🎯 Focus updated: %s\n", focusUpdate.WindowName)
				continue
			} else if messageType.Type == "reflex_query" {
				// Handle muscle memory reflex query
				var reflexQuery struct {
					Type   string `json:"type"`
					Intent string `json:"intent"`
				}
				if err := json.Unmarshal([]byte(line), &reflexQuery); err != nil {
					log.Printf("[KERNEL] Invalid reflex query JSON: %v", err)
					continue
				}

				fmt.Printf("[KERNEL] 💪 Reflex query: %s\n", reflexQuery.Intent)

				// Query for cached plan
				ctx := context.Background()
				cachedPlan, trustScore, err := intentHistoryRepo.GetReflex(ctx, reflexQuery.Intent)

				response := map[string]interface{}{
					"type": "reflex_response",
				}

				if err != nil {
					log.Printf("[KERNEL] Reflex query error: %v", err)
					response["found"] = false
				} else if cachedPlan != "" {
					response["found"] = true
					response["cached_plan"] = cachedPlan
					response["trust_score"] = trustScore
					fmt.Printf("[KERNEL] ⚡ Reflex found (Trust Score: %d)\n", trustScore)
				} else {
					response["found"] = false
					fmt.Printf("[KERNEL] No reflex found for intent\n")
				}

				// Send response
				if err := encoder.Encode(response); err != nil {
					log.Printf("[KERNEL] Failed to send reflex response: %v", err)
				}
				continue
			} else if messageType.Type == "invalidate_reflex" {
				// Handle muscle memory invalidation (bad plan)
				var invalidateReq struct {
					Type   string `json:"type"`
					Intent string `json:"intent"`
				}
				if err := json.Unmarshal([]byte(line), &invalidateReq); err != nil {
					log.Printf("[KERNEL] Invalid invalidate request JSON: %v", err)
					continue
				}

				fmt.Printf("[KERNEL] 🗑️  Invalidating reflex: %s\n", invalidateReq.Intent)

				// Invalidate the cached plan
				ctx := context.Background()
				if err := intentHistoryRepo.InvalidateReflex(ctx, invalidateReq.Intent); err != nil {
					log.Printf("[KERNEL] Failed to invalidate reflex: %v", err)
				} else {
					fmt.Printf("[KERNEL] ✓ Reflex invalidated (will re-learn on next success)\n")
				}
				continue
			} else if messageType.Type == "memory_store" {
				// Handle memory storage request
				var storeReq struct {
					Type    string    `json:"type"`
					Key     string    `json:"key"`
					Value   string    `json:"value"`
					Context string    `json:"context"`
					Vector  []float32 `json:"vector"`
				}
				if err := json.Unmarshal([]byte(line), &storeReq); err != nil {
					log.Printf("[KERNEL] Invalid memory store JSON: %v", err)
					continue
				}

				// Create artifact
				// We use Key as Classification (e.g. "has_resume")
				// Value as Content (e.g. "False")
				// Context as Summary
				artifact := domain.NewArtifact(domain.ArtifactTypeText, storeReq.Value, domain.BoundingBox{})
				artifact.Classification = storeReq.Key
				artifact.Summary = storeReq.Context

				ctx := context.Background()
				if err := memoryRepo.Save(ctx, artifact); err != nil {
					log.Printf("[KERNEL] Failed to save memory: %v", err)
					encoder.Encode(map[string]interface{}{"success": false, "error": err.Error()})
					continue
				}

				// Update embedding if present
				if len(storeReq.Vector) > 0 {
					vectorJSON, _ := json.Marshal(storeReq.Vector)
					if err := memoryRepo.UpdateArtifact(ctx, artifact.ID, storeReq.Key, storeReq.Context, string(vectorJSON)); err != nil {
						log.Printf("[KERNEL] Failed to update memory embedding: %v", err)
					}
				}

				fmt.Printf("[KERNEL] 💾 Memory stored: %s = %s\n", storeReq.Key, storeReq.Value)
				encoder.Encode(map[string]interface{}{"success": true})
				continue

			} else if messageType.Type == "memory_search" {
				// Handle memory search request
				var searchReq struct {
					Type   string    `json:"type"`
					Vector []float32 `json:"vector"`
					Limit  int       `json:"limit"`
				}
				if err := json.Unmarshal([]byte(line), &searchReq); err != nil {
					log.Printf("[KERNEL] Invalid memory search JSON: %v", err)
					continue
				}

				ctx := context.Background()
				results, err := memoryRepo.SearchArtifacts(ctx, searchReq.Vector, searchReq.Limit)
				if err != nil {
					log.Printf("[KERNEL] Memory search error: %v", err)
					encoder.Encode(map[string]interface{}{"artifacts": []domain.Artifact{}})
					continue
				}

				fmt.Printf("[KERNEL] 🔍 Memory search: Found %d artifacts\n", len(results))
				encoder.Encode(map[string]interface{}{"artifacts": results})
				continue
			}
		}

		// Otherwise, parse as permission request
		var req PermissionRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			log.Printf("[KERNEL] Invalid request JSON: %v", err)
			continue
		}

		slog.Info("Permission request received",
			"request_id", req.ID,
			"trace_id", req.TraceID,
			"intent", req.Intent,
			"action_count", len(req.Actions))
		fmt.Printf("[KERNEL] [TraceID: %s] Permission request: %s\n", req.TraceID, req.ID)
		fmt.Printf("[KERNEL]    Intent: %s\n", req.Intent)
		fmt.Printf("[KERNEL]    Actions: %d\n", len(req.Actions))

		// Decision logic: Check focus state
		response := evaluatePermission(&req)

		// Send response back to Python
		if err := encoder.Encode(response); err != nil {
			log.Printf("[KERNEL] Failed to send response: %v", err)
		}

		if response.Approved {
			slog.Info("Permission approved",
				"request_id", req.ID,
				"trace_id", req.TraceID,
				"trust_score", response.TrustScore)
			fmt.Printf("[KERNEL] ✅ [TraceID: %s] APPROVED: %s\n", req.TraceID, req.ID)

			// Record successful approval in intent history with cached plan
			currentFocus.mu.RLock()
			focusedWindow := currentFocus.WindowName
			currentFocus.mu.RUnlock()

			if intentHistoryRepo != nil && focusedWindow != "" {
				ctx := context.Background()

				// Serialize the plan (intent + actions) for muscle memory
				planData := map[string]interface{}{
					"intent":  req.Intent,
					"actions": req.Actions,
				}
				planJSON, err := json.Marshal(planData)
				if err != nil {
					log.Printf("[KERNEL] Warning: Failed to marshal plan: %v", err)
				}

				if err := intentHistoryRepo.RecordSuccess(ctx, req.Intent, focusedWindow, string(planJSON)); err != nil {
					log.Printf("[KERNEL] Warning: Failed to record intent history: %v", err)
				} else {
					fmt.Printf("[KERNEL] 💾 Recorded: Intent '%s' + Window '%s' (Plan cached)\n", req.Intent, focusedWindow)
				}
			}
		} else {
			slog.Warn("Permission blocked",
				"request_id", req.ID,
				"trace_id", req.TraceID,
				"reason", response.Reason,
				"error_code", response.ErrorCode)
			fmt.Printf("[KERNEL] ❌ [TraceID: %s] BLOCKED: %s - %s\n", req.TraceID, req.ID, response.Reason)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("[KERNEL] Scanner error: %v", err)
	}
}

func evaluatePermission(req *PermissionRequest) PermissionResponse {
	// Focus State Verification
	currentFocus.mu.RLock()
	focusedWindow := currentFocus.WindowName
	currentFocus.mu.RUnlock()

	// If expected window is specified, verify it matches current focus
	if req.ExpectedWindow != "" {
		if !strings.Contains(strings.ToLower(focusedWindow), strings.ToLower(req.ExpectedWindow)) {
			return PermissionResponse{
				ID:        req.ID,
				Approved:  false,
				Reason:    fmt.Sprintf("Focus mismatch: Expected '%s', but focused on '%s'", req.ExpectedWindow, focusedWindow),
				ErrorCode: "FOCUS_MISMATCH",
			}
		}
	}

	// Risk Assessment: Dangerous actions require explicit approval
	for _, action := range req.Actions {
		if isDangerousAction(action) {
			return PermissionResponse{
				ID:       req.ID,
				Approved: false,
				Reason:   fmt.Sprintf("Dangerous action detected: %s", action.Type),
			}
		}
	}

	// Trust Score Check: Query intent history
	ctx := context.Background()
	trustScore := 0
	if intentHistoryRepo != nil && focusedWindow != "" {
		score, err := intentHistoryRepo.GetTrustScore(ctx, req.Intent, focusedWindow)
		if err != nil {
			log.Printf("[KERNEL] Warning: Failed to query trust score: %v", err)
		} else {
			trustScore = score
			if trustScore > 0 {
				fmt.Printf("[KERNEL] 📊 Trust Score: %d (Intent '%s' + Window '%s' succeeded %d times before)\n",
					trustScore, req.Intent, focusedWindow, trustScore)
			}
		}
	}

	// Default: Approve if no red flags
	// Note: Trust score is currently informational only, but could be used for auto-approval in the future
	return PermissionResponse{
		ID:         req.ID,
		Approved:   true,
		TrustScore: trustScore,
	}
}

func isDangerousAction(action Action) bool {
	// Use dynamic blocked keywords from config
	if appConfig == nil || !appConfig.Security.SafeMode {
		return false
	}

	dangerousPatterns := appConfig.Security.BlockedKeywords

	actionLower := strings.ToLower(action.Type)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(actionLower, strings.ToLower(pattern)) {
			slog.Warn("Dangerous action detected",
				"action_type", action.Type,
				"matched_keyword", pattern)
			return true
		}
	}

	// Check payload for dangerous content, but SKIP for SPEAK actions
	// SPEAK actions are conversational responses and may contain benign words
	// that happen to include blocked keyword substrings (e.g., "confirm" contains "rm ")
	if action.Type != "SPEAK" {
		// Check multiple payload fields for dangerous content
		fieldsToCheck := []string{"text", "content", "path", "find", "replace"}

		for _, field := range fieldsToCheck {
			if val, ok := action.Payload[field].(string); ok {
				valLower := strings.ToLower(val)
				for _, pattern := range dangerousPatterns {
					if strings.Contains(valLower, strings.ToLower(pattern)) {
						slog.Warn("Dangerous content detected",
							"field", field,
							"content_preview", val[:min(50, len(val))],
							"matched_keyword", pattern)
						return true
					}
				}
			}
		}
	}

	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// UpdateFocusState updates the current focused window (to be called by Sentinel)
func UpdateFocusState(windowName string) {
	currentFocus.mu.Lock()
	defer currentFocus.mu.Unlock()
	currentFocus.WindowName = windowName
	fmt.Printf("[KERNEL] 🎯 Focus updated: %s\n", windowName)
}

// loadConfig loads configuration from config.json
func loadConfig() (*Config, error) {
	// Look for config.json in parent directory (../../config.json from src/kernel/)
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	// Try multiple possible config locations
	configPaths := []string{
		filepath.Join(filepath.Dir(exePath), "config.json"),
		filepath.Join(filepath.Dir(exePath), "..", "..", "config.json"),
		"../../config.json",
		"config.json",
	}

	var config Config
	for _, configPath := range configPaths {
		data, err := os.ReadFile(configPath)
		if err != nil {
			continue // Try next path
		}

		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse config at %s: %w", configPath, err)
		}

		fmt.Printf("[KERNEL] ✓ Loaded config from: %s\n", configPath)
		return &config, nil
	}

	// If no config found, return safe defaults
	fmt.Println("[KERNEL] ⚠️  config.json not found, using safe defaults")
	return &Config{
		System: struct {
			Version     string `json:"version"`
			Environment string `json:"environment"`
			LogLevel    string `json:"log_level"`
			LogFile     string `json:"log_file"`
		}{Version: "3.0.0", Environment: "development", LogLevel: "INFO", LogFile: "kernel.log"},
		Network: struct {
			KernelHost string `json:"kernel_host"`
			KernelPort int    `json:"kernel_port"`
		}{KernelHost: "localhost", KernelPort: 5005},
		Security: struct {
			SafeMode        bool     `json:"safe_mode"`
			BlockedKeywords []string `json:"blocked_keywords"`
		}{SafeMode: true, BlockedKeywords: []string{"delete", "rm ", "format ", "shutdown"}},
	}, nil
}

// setupLogging configures structured JSON logging
func setupLogging(level string, logFile string) {
	var logLevel slog.Level
	switch strings.ToUpper(level) {
	case "DEBUG":
		logLevel = slog.LevelDebug
	case "INFO":
		logLevel = slog.LevelInfo
	case "WARN", "WARNING":
		logLevel = slog.LevelWarn
	case "ERROR":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	// Create file handler for structured logs
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("[KERNEL] Warning: Failed to open log file %s: %v", logFile, err)
		file = os.Stdout
	}

	handler := slog.NewJSONHandler(file, &slog.HandlerOptions{
		Level: logLevel,
	})

	slog.SetDefault(slog.New(handler))
}

// loadOrGenerateToken loads or generates the authentication token
func loadOrGenerateToken() (string, error) {
	// Search order: prioritize project root to unify with Python Brain
	candidatePaths := []string{
		"../../ghost.token", // Project Root (Development Mode: running from src/kernel/)
		"ghost.token",       // Current Directory (Production/Binary Mode)
	}

	// Try to read existing token from candidate paths
	for _, tokenFile := range candidatePaths {
		data, err := os.ReadFile(tokenFile)
		if err == nil {
			token := strings.TrimSpace(string(data))
			if len(token) == 64 { // 32 bytes = 64 hex chars
				fmt.Printf("[KERNEL] 🔑 Loaded auth token from %s\n", tokenFile)
				return token, nil
			}
		}
	}

	// No valid token found - generate new one
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}

	token := hex.EncodeToString(tokenBytes)

	// Write to project root if that directory exists, otherwise fallback to current directory
	targetPath := "../../ghost.token"
	if _, err := os.Stat("../../"); err != nil {
		// Project root not accessible, use current directory
		targetPath = "ghost.token"
	}

	if err := os.WriteFile(targetPath, []byte(token), 0600); err != nil {
		return "", fmt.Errorf("failed to write token file: %w", err)
	}

	fmt.Printf("[KERNEL] 🔐 Generated new auth token: %s\n", targetPath)
	return token, nil
}
