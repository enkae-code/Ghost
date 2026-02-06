// Package gateway provides the WebSocket JSON-RPC 2.0 server for Ghost.
// This is the unified control plane for VA Tactical, handling:
// - Voice wake activation
// - Execution approvals (Conscience Kernel)
// - Memory operations
// - Session management
package gateway

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"ghost/kernel/internal/protocol"

	"github.com/google/uuid"
)

// Server is the Ghost Gateway server
type Server struct {
	host           string
	port           int
	authToken      string
	clients        map[string]*Client
	clientsMu      sync.RWMutex
	startTime      time.Time
	handlers       map[string]MethodHandler
	eventBroadcast chan protocol.EventFrame

	// Dependencies
	approvalHandler ApprovalHandler
	memoryHandler   MemoryHandler
}

// Client represents a connected client
type Client struct {
	ID            string
	Type          string // "brain", "sentinel", "ears", "external"
	Conn          net.Conn
	Encoder       *json.Encoder
	Authenticated bool
	ConnectedAt   time.Time
	Capabilities  []string
}

// MethodHandler processes a JSON-RPC method call
type MethodHandler func(ctx context.Context, client *Client, params json.RawMessage) (json.RawMessage, *protocol.ErrorShape)

// ApprovalHandler interface for permission decisions
type ApprovalHandler interface {
	RequestApproval(ctx context.Context, req *protocol.ExecApprovalRequestParams) (*protocol.ExecApprovalResult, error)
	ResolveApproval(ctx context.Context, req *protocol.ExecApprovalResolveParams) error
}

// MemoryHandler interface for memory operations
type MemoryHandler interface {
	Store(ctx context.Context, req *protocol.MemoryStoreParams) (*protocol.MemoryStoreResult, error)
	Search(ctx context.Context, req *protocol.MemorySearchParams) (*protocol.MemorySearchResult, error)
}

// NewServer creates a new Gateway server
func NewServer(host string, port int, authToken string) *Server {
	s := &Server{
		host:           host,
		port:           port,
		authToken:      authToken,
		clients:        make(map[string]*Client),
		startTime:      time.Now(),
		handlers:       make(map[string]MethodHandler),
		eventBroadcast: make(chan protocol.EventFrame, 100),
	}

	// Register method handlers
	s.registerHandlers()

	return s
}

// SetApprovalHandler sets the approval decision handler
func (s *Server) SetApprovalHandler(h ApprovalHandler) {
	s.approvalHandler = h
}

// SetMemoryHandler sets the memory operations handler
func (s *Server) SetMemoryHandler(h MemoryHandler) {
	s.memoryHandler = h
}

// registerHandlers registers all JSON-RPC method handlers
func (s *Server) registerHandlers() {
	s.handlers["connect"] = s.handleConnect
	s.handlers["wake"] = s.handleWake
	s.handlers["talk_mode"] = s.handleTalkMode
	s.handlers["exec.request"] = s.handleExecRequest
	s.handlers["exec.resolve"] = s.handleExecResolve
	s.handlers["memory.store"] = s.handleMemoryStore
	s.handlers["memory.search"] = s.handleMemorySearch
	s.handlers["focus.update"] = s.handleFocusUpdate
	s.handlers["session.snapshot"] = s.handleSessionSnapshot
	s.handlers["session.update"] = s.handleSessionUpdate
	s.handlers["registry.snapshot"] = s.handleRegistrySnapshot
}

// Start begins listening for connections
func (s *Server) Start(ctx context.Context) error {
	listenAddr := fmt.Sprintf("%s:%d", s.host, s.port)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("failed to bind to %s: %w", listenAddr, err)
	}
	defer listener.Close()

	slog.Info("Ghost Gateway listening", "address", listenAddr, "protocol", protocol.ProtocolVersion)
	fmt.Printf("[GATEWAY] 🌐 WebSocket Gateway listening on %s (Protocol v%s)\n", listenAddr, protocol.ProtocolVersion)

	// Start event broadcaster
	go s.broadcastLoop(ctx)

	// Start heartbeat ticker
	go s.heartbeatLoop(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			conn, err := listener.Accept()
			if err != nil {
				slog.Error("Connection accept error", "error", err)
				continue
			}
			go s.handleConnection(ctx, conn)
		}
	}
}

// handleConnection processes a single client connection
func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	client := &Client{
		ID:          uuid.New().String(),
		Conn:        conn,
		Encoder:     json.NewEncoder(conn),
		ConnectedAt: time.Now(),
	}

	scanner := bufio.NewScanner(conn)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Parse incoming frame
		var frame protocol.RequestFrame
		if err := json.Unmarshal([]byte(line), &frame); err != nil {
			s.sendError(client, "", protocol.ErrCodeParseError, "Invalid JSON", nil)
			continue
		}

		// Validate JSON-RPC version
		if frame.JSONRPC != "2.0" {
			s.sendError(client, frame.ID, protocol.ErrCodeInvalidRequest, "Invalid JSON-RPC version", nil)
			continue
		}

		// Handle the method
		s.dispatchMethod(ctx, client, &frame)
	}

	// Cleanup on disconnect
	s.clientsMu.Lock()
	delete(s.clients, client.ID)
	s.clientsMu.Unlock()

	if client.Authenticated {
		slog.Info("Client disconnected", "client_id", client.ID, "type", client.Type)
	}
}

// dispatchMethod routes a request to the appropriate handler
func (s *Server) dispatchMethod(ctx context.Context, client *Client, frame *protocol.RequestFrame) {
	// Connect is always allowed
	if frame.Method == "connect" {
		result, errShape := s.handleConnect(ctx, client, frame.Params)
		if errShape != nil {
			s.sendError(client, frame.ID, errShape.Code, errShape.Message, errShape.Data)
		} else {
			s.sendResult(client, frame.ID, result)
		}
		return
	}

	// All other methods require authentication
	if !client.Authenticated {
		s.sendError(client, frame.ID, protocol.ErrCodeAuthFailed, "Authentication required", nil)
		return
	}

	// Find handler
	handler, exists := s.handlers[frame.Method]
	if !exists {
		s.sendError(client, frame.ID, protocol.ErrCodeMethodNotFound, fmt.Sprintf("Method not found: %s", frame.Method), nil)
		return
	}

	// Execute handler
	result, errShape := handler(ctx, client, frame.Params)
	if errShape != nil {
		s.sendError(client, frame.ID, errShape.Code, errShape.Message, errShape.Data)
	} else {
		s.sendResult(client, frame.ID, result)
	}
}

// sendResult sends a success response
func (s *Server) sendResult(client *Client, id string, result json.RawMessage) {
	response := protocol.ResponseFrame{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	if err := client.Encoder.Encode(response); err != nil {
		slog.Error("Failed to send response", "client_id", client.ID, "error", err)
	}
}

// sendError sends an error response
func (s *Server) sendError(client *Client, id string, code int, message string, data json.RawMessage) {
	response := protocol.ResponseFrame{
		JSONRPC: "2.0",
		ID:      id,
		Error: &protocol.ErrorShape{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	if err := client.Encoder.Encode(response); err != nil {
		slog.Error("Failed to send error", "client_id", client.ID, "error", err)
	}
}

// broadcastEvent sends an event to all authenticated clients
func (s *Server) broadcastEvent(event protocol.EventFrame) {
	s.eventBroadcast <- event
}

// broadcastLoop processes the event broadcast channel
func (s *Server) broadcastLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-s.eventBroadcast:
			s.clientsMu.RLock()
			for _, client := range s.clients {
				if client.Authenticated {
					if err := client.Encoder.Encode(event); err != nil {
						slog.Warn("Failed to broadcast event", "client_id", client.ID, "error", err)
					}
				}
			}
			s.clientsMu.RUnlock()
		}
	}
}

// heartbeatLoop sends periodic tick events
func (s *Server) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			event := protocol.EventFrame{
				JSONRPC: "2.0",
				Method:  "tick",
			}
			tickData := protocol.TickEvent{
				Timestamp: time.Now(),
				Uptime:    int64(time.Since(s.startTime).Seconds()),
			}
			if data, err := json.Marshal(tickData); err == nil {
				event.Params = data
				s.broadcastEvent(event)
			}
		}
	}
}

// --- Method Handlers ---

func (s *Server) handleConnect(ctx context.Context, client *Client, params json.RawMessage) (json.RawMessage, *protocol.ErrorShape) {
	var req protocol.ConnectParams
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &protocol.ErrorShape{Code: protocol.ErrCodeInvalidParams, Message: "Invalid connect params"}
	}

	// Validate token
	if req.Token != s.authToken {
		slog.Warn("Authentication failed", "client_id", client.ID, "remote_addr", client.Conn.RemoteAddr())
		return nil, &protocol.ErrorShape{Code: protocol.ErrCodeAuthFailed, Message: "Invalid authentication token"}
	}

	// Set client info
	client.Authenticated = true
	client.Type = req.ClientType
	client.Capabilities = s.getCapabilitiesForType(req.ClientType)

	// Register client
	s.clientsMu.Lock()
	s.clients[client.ID] = client
	s.clientsMu.Unlock()

	slog.Info("Client authenticated", "client_id", client.ID, "type", client.Type)
	fmt.Printf("[GATEWAY] ✓ Client authenticated: %s (%s)\n", client.ID[:8], client.Type)

	result := protocol.ConnectResult{
		SessionID:     client.ID,
		ServerVersion: protocol.ProtocolVersion,
		ExpiresAt:     time.Now().Add(24 * time.Hour),
		Capabilities:  client.Capabilities,
	}

	data, _ := json.Marshal(result)
	return data, nil
}

func (s *Server) handleWake(ctx context.Context, client *Client, params json.RawMessage) (json.RawMessage, *protocol.ErrorShape) {
	var req protocol.WakeParams
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &protocol.ErrorShape{Code: protocol.ErrCodeInvalidParams, Message: "Invalid wake params"}
	}

	slog.Info("Voice wake activated", "keyword", req.Keyword, "patience_ms", req.PatienceMs)
	fmt.Printf("[GATEWAY] 🎤 Voice wake activated: '%s' (patience: %dms)\n", req.Keyword, req.PatienceMs)

	// TODO: Connect to Rust voice_wake module
	result := protocol.WakeResult{
		Active:   true,
		StreamID: req.AudioStreamID,
	}

	data, _ := json.Marshal(result)
	return data, nil
}

func (s *Server) handleTalkMode(ctx context.Context, client *Client, params json.RawMessage) (json.RawMessage, *protocol.ErrorShape) {
	var req protocol.TalkModeParams
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &protocol.ErrorShape{Code: protocol.ErrCodeInvalidParams, Message: "Invalid talk_mode params"}
	}

	slog.Info("Talk mode changed", "enabled", req.Enabled, "patience_ms", req.PatienceMs, "auto_extend", req.AutoExtend)
	fmt.Printf("[GATEWAY] 🗣️ Talk mode: %v (patience: %dms, auto_extend: %v)\n", req.Enabled, req.PatienceMs, req.AutoExtend)

	result := protocol.TalkModeResult{
		Active:    req.Enabled,
		StartedAt: time.Now(),
	}

	data, _ := json.Marshal(result)
	return data, nil
}

func (s *Server) handleExecRequest(ctx context.Context, client *Client, params json.RawMessage) (json.RawMessage, *protocol.ErrorShape) {
	var req protocol.ExecApprovalRequestParams
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &protocol.ErrorShape{Code: protocol.ErrCodeInvalidParams, Message: "Invalid exec.request params"}
	}

	slog.Info("Execution approval requested", "request_id", req.RequestID, "intent", req.Intent, "risk_level", req.RiskLevel)

	if s.approvalHandler == nil {
		return nil, &protocol.ErrorShape{Code: protocol.ErrCodeInternalError, Message: "No approval handler configured"}
	}

	result, err := s.approvalHandler.RequestApproval(ctx, &req)
	if err != nil {
		return nil, &protocol.ErrorShape{Code: protocol.ErrCodeInternalError, Message: err.Error()}
	}

	data, _ := json.Marshal(result)
	return data, nil
}

func (s *Server) handleExecResolve(ctx context.Context, client *Client, params json.RawMessage) (json.RawMessage, *protocol.ErrorShape) {
	var req protocol.ExecApprovalResolveParams
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &protocol.ErrorShape{Code: protocol.ErrCodeInvalidParams, Message: "Invalid exec.resolve params"}
	}

	slog.Info("Execution approval resolved", "request_id", req.RequestID, "approved", req.Approved)

	if s.approvalHandler == nil {
		return nil, &protocol.ErrorShape{Code: protocol.ErrCodeInternalError, Message: "No approval handler configured"}
	}

	if err := s.approvalHandler.ResolveApproval(ctx, &req); err != nil {
		return nil, &protocol.ErrorShape{Code: protocol.ErrCodeInternalError, Message: err.Error()}
	}

	data, _ := json.Marshal(map[string]bool{"success": true})
	return data, nil
}

func (s *Server) handleMemoryStore(ctx context.Context, client *Client, params json.RawMessage) (json.RawMessage, *protocol.ErrorShape) {
	var req protocol.MemoryStoreParams
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &protocol.ErrorShape{Code: protocol.ErrCodeInvalidParams, Message: "Invalid memory.store params"}
	}

	slog.Info("Memory store requested", "key", req.Key)

	if s.memoryHandler == nil {
		return nil, &protocol.ErrorShape{Code: protocol.ErrCodeInternalError, Message: "No memory handler configured"}
	}

	result, err := s.memoryHandler.Store(ctx, &req)
	if err != nil {
		return nil, &protocol.ErrorShape{Code: protocol.ErrCodeMemoryError, Message: err.Error()}
	}

	data, _ := json.Marshal(result)
	return data, nil
}

func (s *Server) handleMemorySearch(ctx context.Context, client *Client, params json.RawMessage) (json.RawMessage, *protocol.ErrorShape) {
	var req protocol.MemorySearchParams
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &protocol.ErrorShape{Code: protocol.ErrCodeInvalidParams, Message: "Invalid memory.search params"}
	}

	slog.Info("Memory search requested", "query", req.Query, "limit", req.Limit)

	if s.memoryHandler == nil {
		return nil, &protocol.ErrorShape{Code: protocol.ErrCodeInternalError, Message: "No memory handler configured"}
	}

	result, err := s.memoryHandler.Search(ctx, &req)
	if err != nil {
		return nil, &protocol.ErrorShape{Code: protocol.ErrCodeMemoryError, Message: err.Error()}
	}

	data, _ := json.Marshal(result)
	return data, nil
}

func (s *Server) handleFocusUpdate(ctx context.Context, client *Client, params json.RawMessage) (json.RawMessage, *protocol.ErrorShape) {
	var req protocol.FocusUpdateParams
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &protocol.ErrorShape{Code: protocol.ErrCodeInvalidParams, Message: "Invalid focus.update params"}
	}

	slog.Info("Focus updated via gateway", "window_name", req.WindowName)
	fmt.Printf("[GATEWAY] 🎯 Focus: %s\n", req.WindowName)

	// Broadcast focus change event
	event := protocol.EventFrame{
		JSONRPC: "2.0",
		Method:  "focus.changed",
	}
	eventData := protocol.FocusChangedEvent{
		Timestamp:   req.Timestamp,
		WindowName:  req.WindowName,
		ProcessName: req.ProcessName,
	}
	if data, err := json.Marshal(eventData); err == nil {
		event.Params = data
		s.broadcastEvent(event)
	}

	data, _ := json.Marshal(map[string]bool{"success": true})
	return data, nil
}

func (s *Server) handleSessionSnapshot(ctx context.Context, client *Client, params json.RawMessage) (json.RawMessage, *protocol.ErrorShape) {
	var req protocol.SessionSnapshotParams
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &protocol.ErrorShape{Code: protocol.ErrCodeInvalidParams, Message: "Invalid session.snapshot params"}
	}

	// TODO: Implement session storage retrieval
	snapshot := protocol.SessionSnapshot{
		SessionID:      req.SessionID,
		ConversationID: uuid.New().String(),
		CreatedAt:      time.Now(),
		LastActiveAt:   time.Now(),
	}

	data, _ := json.Marshal(snapshot)
	return data, nil
}

// handleSessionUpdate handles streaming text updates during a session
func (s *Server) handleSessionUpdate(ctx context.Context, client *Client, params json.RawMessage) (json.RawMessage, *protocol.ErrorShape) {
	var req protocol.SessionUpdateParams
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &protocol.ErrorShape{Code: protocol.ErrCodeInvalidParams, Message: "Invalid session.update params"}
	}

	slog.Debug("Session update received", "session_id", req.SessionID, "message_id", req.MessageID, "is_complete", req.IsComplete)

	// Broadcast the session update event to all connected clients
	event := protocol.EventFrame{
		JSONRPC: "2.0",
		Method:  "session.update",
	}
	eventData := protocol.SessionUpdateEvent{
		SessionID:  req.SessionID,
		MessageID:  req.MessageID,
		Delta:      req.Delta,
		IsComplete: req.IsComplete,
		Role:       req.Role,
		Timestamp:  req.Timestamp,
	}
	if data, err := json.Marshal(eventData); err == nil {
		event.Params = data
		s.broadcastEvent(event)
	}

	data, _ := json.Marshal(map[string]bool{"success": true})
	return data, nil
}

// handleRegistrySnapshot returns a snapshot of all connected clients
func (s *Server) handleRegistrySnapshot(ctx context.Context, client *Client, params json.RawMessage) (json.RawMessage, *protocol.ErrorShape) {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	clients := make([]protocol.ClientInfo, 0, len(s.clients))
	for _, c := range s.clients {
		if c.Authenticated {
			clients = append(clients, protocol.ClientInfo{
				ID:           c.ID,
				Type:         c.Type,
				Capabilities: c.Capabilities,
				ConnectedAt:  c.ConnectedAt,
				LastSeen:     time.Now(), // Simplification: all connected clients are "seen"
				Status:       "connected",
			})
		}
	}

	snapshot := protocol.ClientRegistrySnapshot{
		Clients:   clients,
		Timestamp: time.Now(),
	}

	slog.Info("Client registry snapshot requested", "client_count", len(clients))
	fmt.Printf("[GATEWAY] 📋 Registry snapshot: %d clients\n", len(clients))

	data, _ := json.Marshal(snapshot)
	return data, nil
}

// getCapabilitiesForType returns allowed methods based on client type
func (s *Server) getCapabilitiesForType(clientType string) []string {
	switch clientType {
	case "brain":
		return []string{"exec.request", "memory.store", "memory.search", "session.snapshot", "session.update", "registry.snapshot"}
	case "sentinel":
		return []string{"focus.update"}
	case "ears":
		return []string{"wake", "talk_mode"}
	case "external":
		return []string{"wake", "talk_mode", "session.snapshot"} // Limited for mobile/external clients
	default:
		return []string{}
	}
}
