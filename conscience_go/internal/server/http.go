// Author: Enkae (enkae.dev@pm.me)
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"ghost/kernel/internal/adapter"
	"ghost/kernel/internal/domain"
)

// Server represents the HTTP API server
type Server struct {
	repo       *adapter.SQLiteRepository
	cmdRepo    *adapter.CommandRepository
	actionRepo *adapter.ActionRepository
	goalRepo   *adapter.GoalRepository
	stateRepo  *adapter.StateRepository
	mux        *http.ServeMux
}

// NewServer creates a new HTTP server instance
func NewServer(repo *adapter.SQLiteRepository, cmdRepo *adapter.CommandRepository, actionRepo *adapter.ActionRepository, goalRepo *adapter.GoalRepository, stateRepo *adapter.StateRepository) *Server {
	s := &Server{
		repo:       repo,
		cmdRepo:    cmdRepo,
		actionRepo: actionRepo,
		goalRepo:   goalRepo,
		stateRepo:  stateRepo,
		mux:        http.NewServeMux(),
	}

	s.registerRoutes()
	return s
}

// registerRoutes sets up all HTTP endpoints
func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/api/artifacts/", s.handleArtifactByID) // Handle both GET /api/artifacts and POST /api/artifacts/{id}/enrich
	s.mux.HandleFunc("/api/search", s.handleSearch) // Semantic search endpoint
	s.mux.HandleFunc("/api/commands/pending", s.handlePendingCommands) // Command queue for Sentinel
	s.mux.HandleFunc("/api/commands", s.handleCommands) // Create new commands
	s.mux.HandleFunc("/api/stream", s.handleStream)

	// Permission Kernel endpoints
	s.mux.HandleFunc("/api/propose", s.handlePropose) // Cortex proposes actions
	s.mux.HandleFunc("/api/approvals", s.handleApprovals) // UI polls for pending approvals
	s.mux.HandleFunc("/api/approve/", s.handleApprove) // User approves/rejects actions
	s.mux.HandleFunc("/api/reply/", s.handleReply) // User replies to clarification requests
	s.mux.HandleFunc("/api/modes", s.handleUserModes) // Get/Set automation modes
	s.mux.HandleFunc("/api/actions/approved", s.handleApprovedActions) // Effector queue
	s.mux.HandleFunc("/api/actions/", s.handleActionStatus) // Update action status

	// Agentic Planner endpoints
	s.mux.HandleFunc("/api/goal", s.handleGoal) // POST to inject goal, GET to poll for active goal

	// RAG endpoints (Omniscient Operator)
	s.mux.HandleFunc("/api/search/vector", s.handleVectorSearch) // POST with vector, returns similar artifacts

	// Consciousness Switch endpoints (Global State Manager)
	s.mux.HandleFunc("/api/state", s.handleState) // GET current state, POST to update state
}

// handleHealth returns a simple health check response
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "Engram Online",
	})
}

// handleArtifactByID routes requests to either list artifacts or enrich a specific artifact
func (s *Server) handleArtifactByID(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	
	// GET /api/artifacts - list all artifacts
	if path == "/api/artifacts" || path == "/api/artifacts/" {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleArtifactsList(w, r)
		return
	}
	
	// POST /api/artifacts/{id}/enrich - enrich artifact
	if r.Method == http.MethodPost && len(path) > len("/api/artifacts/") {
		s.handleArtifactEnrich(w, r)
		return
	}
	
	http.Error(w, "Not found", http.StatusNotFound)
}

// handleArtifactsList returns the last 50 artifacts from the database
func (s *Server) handleArtifactsList(w http.ResponseWriter, r *http.Request) {
	artifacts, err := s.repo.GetLastArtifacts(context.Background(), 50)
	if err != nil {
		log.Printf("[ERROR] Failed to fetch artifacts: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	if err := json.NewEncoder(w).Encode(artifacts); err != nil {
		log.Printf("[ERROR] Failed to encode artifacts: %v", err)
	}
}

// EnrichmentRequest represents the payload for enriching an artifact
type EnrichmentRequest struct {
	Classification string   `json:"classification"`
	Summary        string   `json:"summary"`
	Embedding     []float32 `json:"embedding"`
}

// handleArtifactEnrich handles POST /api/artifacts/{id}/enrich
func (s *Server) handleArtifactEnrich(w http.ResponseWriter, r *http.Request) {
	// Only handle paths that end with /enrich
	if r.URL.Path == "/api/artifacts/" || r.URL.Path == "/api/artifacts" {
		// Let handleArtifacts handle this
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract artifact ID from path: /api/artifacts/{id}/enrich
	path := r.URL.Path
	if len(path) < len("/api/artifacts/") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	// Remove prefix and suffix to get ID
	pathWithoutPrefix := path[len("/api/artifacts/"):]
	artifactID := pathWithoutPrefix
	if len(pathWithoutPrefix) > len("/enrich") && pathWithoutPrefix[len(pathWithoutPrefix)-len("/enrich"):] == "/enrich" {
		artifactID = pathWithoutPrefix[:len(pathWithoutPrefix)-len("/enrich")]
	} else {
		http.Error(w, "Path must end with /enrich", http.StatusBadRequest)
		return
	}

	if artifactID == "" {
		http.Error(w, "Artifact ID is required", http.StatusBadRequest)
		return
	}

	// Parse request body
	var req EnrichmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[ERROR] Failed to decode enrichment request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Classification == "" && req.Summary == "" {
		http.Error(w, "At least one of classification or summary is required", http.StatusBadRequest)
		return
	}

	// Convert embedding to JSON string
	var embeddingJSON string
	if len(req.Embedding) > 0 {
		if embeddingBytes, err := json.Marshal(req.Embedding); err == nil {
			embeddingJSON = string(embeddingBytes)
		}
	}

	// Update artifact in database
	if err := s.repo.UpdateArtifact(context.Background(), artifactID, req.Classification, req.Summary, embeddingJSON); err != nil {
		log.Printf("[ERROR] Failed to enrich artifact %s: %v", artifactID, err)
		http.Error(w, "Failed to update artifact", http.StatusInternalServerError)
		return
	}

	log.Printf("[HIPPOCAMPUS] Artifact %s enriched: %s | %s | Vector: %d dims", artifactID[:8], req.Classification, req.Summary, len(req.Embedding))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Artifact enriched successfully",
	})
}

// handleSearch performs semantic search over artifacts
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get search query from URL parameter
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Query parameter 'q' is required", http.StatusBadRequest)
		return
	}

	// For now, we'll implement a simple text search
	// TODO: Integrate with Python for vectorization and semantic search
	artifacts, err := s.repo.GetLastArtifacts(context.Background(), 50)
	if err != nil {
		log.Printf("[ERROR] Failed to fetch artifacts for search: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Simple text-based filtering (placeholder for semantic search)
	var filteredArtifacts []domain.Artifact
	for _, artifact := range artifacts {
		// Simple text matching for now
		if strings.Contains(strings.ToLower(artifact.Content), strings.ToLower(query)) ||
		   strings.Contains(strings.ToLower(artifact.Classification), strings.ToLower(query)) ||
		   strings.Contains(strings.ToLower(artifact.Summary), strings.ToLower(query)) {
			filteredArtifacts = append(filteredArtifacts, artifact)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	if err := json.NewEncoder(w).Encode(filteredArtifacts); err != nil {
		log.Printf("[ERROR] Failed to encode search results: %v", err)
	}
}

// handlePendingCommands returns all pending commands for the Sentinel to execute
func (s *Server) handlePendingCommands(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	commands, err := s.cmdRepo.GetPendingCommands(context.Background())
	if err != nil {
		log.Printf("[ERROR] Failed to fetch pending commands: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	if err := json.NewEncoder(w).Encode(commands); err != nil {
		log.Printf("[ERROR] Failed to encode commands: %v", err)
	}
}

// handleCommands handles creating new commands and updating command status
func (s *Server) handleCommands(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.createCommand(w, r)
	case http.MethodPatch:
		s.updateCommandStatus(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// createCommand creates a new command
func (s *Server) createCommand(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action  string `json:"action"`
		Target  string `json:"target"`
		Payload string `json:"payload"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[ERROR] Failed to decode command request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate action
	var action domain.CommandAction
	switch req.Action {
	case "TYPE":
		action = domain.CommandActionType
	case "CLICK":
		action = domain.CommandActionClick
	case "FOCUS_WINDOW":
		action = domain.CommandActionFocusWindow
	case "OPEN_APP":
		action = domain.CommandActionOpenApp
	default:
		http.Error(w, "Invalid action type", http.StatusBadRequest)
		return
	}

	cmd := domain.NewCommand(action, req.Target, req.Payload)

	if err := s.cmdRepo.SaveCommand(context.Background(), cmd); err != nil {
		log.Printf("[ERROR] Failed to save command: %v", err)
		http.Error(w, "Failed to create command", http.StatusInternalServerError)
		return
	}

	log.Printf("[COMMAND] Created: %s | Action: %s | Payload: %s", cmd.ID[:8], cmd.Action, cmd.Payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(cmd)
}

// updateCommandStatus updates the status of a command
func (s *Server) updateCommandStatus(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[ERROR] Failed to decode status update request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var status domain.CommandStatus
	switch req.Status {
	case "executing":
		status = domain.CommandStatusExecuting
	case "completed":
		status = domain.CommandStatusCompleted
	case "failed":
		status = domain.CommandStatusFailed
	default:
		http.Error(w, "Invalid status", http.StatusBadRequest)
		return
	}

	if err := s.cmdRepo.UpdateCommandStatus(context.Background(), req.ID, status); err != nil {
		log.Printf("[ERROR] Failed to update command status: %v", err)
		http.Error(w, "Failed to update command", http.StatusInternalServerError)
		return
	}

	log.Printf("[COMMAND] Updated %s to %s", req.ID[:8], status)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Command status updated",
	})
}

// handleStream is a placeholder for Server-Sent Events or WebSocket streaming
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	fmt.Fprintf(w, "data: {\"status\": \"Stream endpoint placeholder - SSE/WebSocket coming soon\"}\n\n")

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// Start launches the HTTP server on the specified address
func (s *Server) Start(addr string) error {
	log.Printf("HTTP server starting on %s", addr)

	server := &http.Server{
		Addr:              addr,
		Handler:           s.mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return server.ListenAndServe()
}

// ========================================
// PERMISSION KERNEL ENDPOINTS
// ========================================

// ProposeRequest represents an action proposal from the Cortex
type ProposeRequest struct {
	Intent    string          `json:"intent"`
	RiskScore int             `json:"risk_score"`
	Payload   json.RawMessage `json:"payload"`
	Domain    string          `json:"domain"`
}

// handlePropose handles POST /api/propose - Cortex submits action proposals
func (s *Server) handlePropose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ProposeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[KERNEL] Failed to decode propose request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Intent == "" {
		http.Error(w, "Intent is required", http.StatusBadRequest)
		return
	}

	if req.RiskScore < 0 || req.RiskScore > 100 {
		http.Error(w, "Risk score must be between 0 and 100", http.StatusBadRequest)
		return
	}

	// Create action proposal
	action := domain.NewActionProposal(req.Intent, req.RiskScore, req.Payload, req.Domain)

	// Get user mode for this domain
	userMode, err := s.actionRepo.GetUserMode(context.Background(), req.Domain)
	if err != nil {
		log.Printf("[KERNEL] Failed to get user mode: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Apply Permission Kernel logic
	if action.ShouldAutoApprove(userMode) {
		// Auto-approve low-risk actions in AUTO mode
		action.Status = domain.ActionProposalStatusExecuting
		log.Printf("[KERNEL] ✓ AUTO-APPROVED: %s | Risk: %d | Domain: %s", action.Intent, action.RiskScore, action.Domain)
	} else {
		// Hold for user approval
		action.Status = domain.ActionProposalStatusWaitingForUser
		log.Printf("[KERNEL] ⏸ WAITING FOR USER: %s | Risk: %d | Mode: %s", action.Intent, action.RiskScore, userMode.Mode)
	}

	// Save to database
	if err := s.actionRepo.SaveActionProposal(context.Background(), action); err != nil {
		log.Printf("[KERNEL] Failed to save action proposal: %v", err)
		http.Error(w, "Failed to save action", http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(action)
}

// handleApprovals handles GET /api/approvals - UI polls for pending approvals
func (s *Server) handleApprovals(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	actions, err := s.actionRepo.GetPendingApprovals(context.Background())
	if err != nil {
		log.Printf("[KERNEL] Failed to fetch pending approvals: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(actions)
}

// ApprovalRequest represents user's approval/rejection decision
type ApprovalRequest struct {
	Approved bool `json:"approved"`
}

// handleApprove handles POST /api/approve/{id} - User approves or rejects
func (s *Server) handleApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract action ID from path
	path := r.URL.Path
	if len(path) < len("/api/approve/") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	actionID := path[len("/api/approve/"):]
	if actionID == "" {
		http.Error(w, "Action ID is required", http.StatusBadRequest)
		return
	}

	// Parse approval decision
	var req ApprovalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[KERNEL] Failed to decode approval request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Update status based on user decision
	var newStatus domain.ActionProposalStatus
	if req.Approved {
		newStatus = domain.ActionProposalStatusApproved
		log.Printf("[KERNEL] ✓ USER APPROVED: %s", actionID[:8])
	} else {
		newStatus = domain.ActionProposalStatusRejected
		log.Printf("[KERNEL] ✗ USER REJECTED: %s", actionID[:8])
	}

	if err := s.actionRepo.UpdateActionStatus(context.Background(), actionID, newStatus); err != nil {
		log.Printf("[KERNEL] Failed to update action status: %v", err)
		http.Error(w, "Failed to update action", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Action decision recorded",
	})
}

// ReplyRequest represents user's response to a clarification request
type ReplyRequest struct {
	Message string `json:"message"`
}

// handleReply handles POST /api/reply/{id} - User replies to clarification
func (s *Server) handleReply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract action ID from path
	path := r.URL.Path
	if len(path) < len("/api/reply/") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	actionID := path[len("/api/reply/"):]
	if actionID == "" {
		http.Error(w, "Action ID is required", http.StatusBadRequest)
		return
	}

	// Parse user's reply
	var req ReplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[GHOST_CHAT] Failed to decode reply request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	// Store user response
	if err := s.actionRepo.UpdateUserResponse(context.Background(), actionID, req.Message); err != nil {
		log.Printf("[GHOST_CHAT] Failed to update user response: %v", err)
		http.Error(w, "Failed to save response", http.StatusInternalServerError)
		return
	}

	// Update status to PENDING so agent can resume
	if err := s.actionRepo.UpdateActionStatus(context.Background(), actionID, domain.ActionProposalStatusPending); err != nil {
		log.Printf("[GHOST_CHAT] Failed to update status: %v", err)
		http.Error(w, "Failed to update status", http.StatusInternalServerError)
		return
	}

	log.Printf("[GHOST_CHAT] 💬 User replied to %s: \"%s\"", actionID[:8], req.Message)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Reply received, agent will resume",
	})
}

// handleUserModes handles GET/POST /api/modes - Manage automation modes
func (s *Server) handleUserModes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getUserMode(w, r)
	case http.MethodPost:
		s.setUserMode(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getUserMode retrieves the current user mode for a domain
func (s *Server) getUserMode(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Query().Get("domain")
	if domain == "" {
		domain = "*" // Default to global mode
	}

	userMode, err := s.actionRepo.GetUserMode(context.Background(), domain)
	if err != nil {
		log.Printf("[KERNEL] Failed to get user mode: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(userMode)
}

// ModeRequest represents a request to change automation mode
type ModeRequest struct {
	Domain string `json:"domain"`
	Mode   string `json:"mode"`
}

// setUserMode sets the automation mode for a domain
func (s *Server) setUserMode(w http.ResponseWriter, r *http.Request) {
	var req ModeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[KERNEL] Failed to decode mode request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Domain == "" {
		req.Domain = "*" // Default to global mode
	}

	var mode domain.ModeType
	switch req.Mode {
	case "AUTO":
		mode = domain.ModeTypeAuto
	case "MANUAL":
		mode = domain.ModeTypeManual
	default:
		http.Error(w, "Invalid mode (must be AUTO or MANUAL)", http.StatusBadRequest)
		return
	}

	if err := s.actionRepo.SetUserMode(context.Background(), req.Domain, mode); err != nil {
		log.Printf("[KERNEL] Failed to set user mode: %v", err)
		http.Error(w, "Failed to set mode", http.StatusInternalServerError)
		return
	}

	log.Printf("[KERNEL] Mode changed: %s -> %s", req.Domain, mode)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Mode updated successfully",
	})
}

// handleApprovedActions handles GET /api/actions/approved - Effector Queue
// Returns all approved actions ready for execution by the Sentinel
func (s *Server) handleApprovedActions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	actions, err := s.actionRepo.GetApprovedActions(context.Background())
	if err != nil {
		log.Printf("[EFFECTOR] Failed to fetch approved actions: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(actions)
}

// ActionStatusRequest represents a request to update action status
type ActionStatusRequest struct {
	Status string `json:"status"`
}

// handleActionStatus handles POST /api/actions/{id}/complete, /fail, or GET /api/actions/{id}
func (s *Server) handleActionStatus(w http.ResponseWriter, r *http.Request) {
	// Skip if this is the /api/actions/approved route
	if r.URL.Path == "/api/actions/approved" {
		return
	}

	// GET /api/actions/{id} - Return action status (for polling)
	if r.Method == http.MethodGet {
		s.handleActionLookup(w, r)
		return
	}

	if r.Method != http.MethodPost && r.Method != http.MethodPatch {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract action ID from path
	path := r.URL.Path
	if len(path) < len("/api/actions/") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	// Parse path: /api/actions/{id}/complete or /api/actions/{id}/fail
	pathWithoutPrefix := path[len("/api/actions/"):]
	parts := strings.Split(pathWithoutPrefix, "/")

	if len(parts) < 2 {
		http.Error(w, "Invalid path format. Use /api/actions/{id}/complete or /fail", http.StatusBadRequest)
		return
	}

	actionID := parts[0]
	statusAction := parts[1]

	if actionID == "" {
		http.Error(w, "Action ID is required", http.StatusBadRequest)
		return
	}

	var newStatus domain.ActionProposalStatus

	switch statusAction {
	case "complete":
		newStatus = domain.ActionProposalStatusCompleted
		log.Printf("[EFFECTOR] ✓ Action %s marked as COMPLETED", actionID[:8])
	case "fail":
		newStatus = domain.ActionProposalStatusFailed
		log.Printf("[EFFECTOR] ✗ Action %s marked as FAILED", actionID[:8])
	case "executing":
		newStatus = domain.ActionProposalStatusExecuting
		log.Printf("[EFFECTOR] ⚡ Action %s marked as EXECUTING", actionID[:8])
	default:
		http.Error(w, "Invalid status action. Use 'complete', 'fail', or 'executing'", http.StatusBadRequest)
		return
	}

	if err := s.actionRepo.UpdateActionStatus(context.Background(), actionID, newStatus); err != nil {
		log.Printf("[ERROR] Failed to update action status: %v", err)
		http.Error(w, "Failed to update action status", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Action status updated to %s", newStatus),
	})
}

// GoalRequest represents a natural language goal from the user
type GoalRequest struct {
	Goal string `json:"goal"`
}

// handleGoal handles POST /api/goal (inject goal) and GET /api/goal (poll for active goal)
func (s *Server) handleGoal(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleInjectGoal(w, r)
	case http.MethodGet:
		s.handlePollGoal(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleInjectGoal handles POST /api/goal - User injects a natural language goal
func (s *Server) handleInjectGoal(w http.ResponseWriter, r *http.Request) {
	var req GoalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[PLANNER] Failed to decode goal request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Goal == "" {
		http.Error(w, "Goal text is required", http.StatusBadRequest)
		return
	}

	// Create new goal
	goal := domain.NewGoal(req.Goal)

	// Save to database
	if err := s.goalRepo.SaveGoal(context.Background(), goal); err != nil {
		log.Printf("[PLANNER] Failed to save goal: %v", err)
		http.Error(w, "Failed to save goal", http.StatusInternalServerError)
		return
	}

	log.Printf("[PLANNER] 🎯 Goal injected: %s | ID: %s", goal.GoalText, goal.ID[:8])

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(goal)
}

// handlePollGoal handles GET /api/goal - Python polls for active goals
func (s *Server) handlePollGoal(w http.ResponseWriter, r *http.Request) {
	goal, err := s.goalRepo.GetActiveGoal(context.Background())
	if err != nil {
		log.Printf("[PLANNER] Failed to fetch active goal: %v", err)
		http.Error(w, "Failed to fetch goal", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if goal == nil {
		// No active goal - return empty object
		json.NewEncoder(w).Encode(map[string]interface{}{})
	} else {
		json.NewEncoder(w).Encode(goal)
	}
}

// ========================================
// RAG ENDPOINTS (OMNISCIENT OPERATOR)
// ========================================

// VectorSearchRequest represents a vector search query
type VectorSearchRequest struct {
	Vector []float32 `json:"vector"`
	Limit  int       `json:"limit"`
}

// handleVectorSearch handles POST /api/search/vector - Semantic memory search
func (s *Server) handleVectorSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req VectorSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[RAG] Failed to decode vector search request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.Vector) == 0 {
		http.Error(w, "Vector is required", http.StatusBadRequest)
		return
	}

	// Default limit
	if req.Limit <= 0 {
		req.Limit = 10
	}

	// Search artifacts using cosine similarity
	artifacts, err := s.repo.SearchArtifacts(context.Background(), req.Vector, req.Limit)
	if err != nil {
		log.Printf("[RAG] Failed to search artifacts: %v", err)
		http.Error(w, "Search failed", http.StatusInternalServerError)
		return
	}

	log.Printf("[RAG] Vector search returned %d results (requested: %d)", len(artifacts), req.Limit)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(artifacts)
}

// handleActionLookup handles GET /api/actions/{id} - Poll for action status
func (s *Server) handleActionLookup(w http.ResponseWriter, r *http.Request) {
	// Extract action ID from path
	path := r.URL.Path
	if len(path) < len("/api/actions/") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	// Get ID (everything after /api/actions/)
	pathWithoutPrefix := path[len("/api/actions/"):]

	// Handle nested paths like /api/actions/{id}/complete
	parts := strings.Split(pathWithoutPrefix, "/")
	actionID := parts[0]

	if actionID == "" || actionID == "approved" {
		http.Error(w, "Action ID is required", http.StatusBadRequest)
		return
	}

	// Retrieve action from database
	action, err := s.actionRepo.GetActionByID(context.Background(), actionID)
	if err != nil {
		log.Printf("[RAG] Failed to fetch action %s: %v", actionID[:8], err)
		http.Error(w, "Action not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(action)
}

// ========================================
// CONSCIOUSNESS SWITCH ENDPOINTS
// ========================================

// StateRequest represents a request to change application state
type StateRequest struct {
	State string `json:"state"`
}

// handleState handles GET /api/state (get current) and POST /api/state (set new state)
func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetState(w, r)
	case http.MethodPost:
		s.handleSetState(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetState returns the current application state
func (s *Server) handleGetState(w http.ResponseWriter, r *http.Request) {
	state, err := s.stateRepo.GetState(context.Background())
	if err != nil {
		log.Printf("[STATE] Failed to get state: %v", err)
		http.Error(w, "Failed to get state", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"state": string(state),
	})
}

// handleSetState updates the application state
func (s *Server) handleSetState(w http.ResponseWriter, r *http.Request) {
	var req StateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[STATE] Failed to decode state request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate and convert to AppState
	newState := domain.AppState(req.State)
	if !newState.IsValid() {
		http.Error(w, "Invalid state. Must be ACTIVE, SHADOW, or PAUSED", http.StatusBadRequest)
		return
	}

	// Update state
	if err := s.stateRepo.SetState(context.Background(), newState); err != nil {
		log.Printf("[STATE] Failed to set state: %v", err)
		http.Error(w, "Failed to update state", http.StatusInternalServerError)
		return
	}

	// Log state change with visual indicator
	var emoji string
	switch newState {
	case domain.AppStateActive:
		emoji = "🟢"
	case domain.AppStateShadow:
		emoji = "🟡"
	case domain.AppStatePaused:
		emoji = "🔴"
	}

	log.Printf("[STATE] %s Consciousness switched to: %s", emoji, newState)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("State updated to %s", newState),
		"state":   string(newState),
	})
}
