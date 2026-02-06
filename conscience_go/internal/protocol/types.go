// Author: Enkae (enkae.dev@pm.me)
// Package protocol defines JSON-RPC 2.0 message types for the Ghost Gateway.
// Based on Clawd Bot's protocol architecture, translated to Go for VA Tactical.
package protocol

import (
	"encoding/json"
	"time"
)

// ProtocolVersion is the current gateway protocol version
const ProtocolVersion = "1.0.0"

// JSON-RPC 2.0 Frame Types
// -------------------------

// RequestFrame represents an incoming JSON-RPC request
type RequestFrame struct {
	JSONRPC string          `json:"jsonrpc"` // Must be "2.0"
	ID      string          `json:"id"`      // Request identifier
	Method  string          `json:"method"`  // Method name
	Params  json.RawMessage `json:"params"`  // Method parameters
}

// ResponseFrame represents an outgoing JSON-RPC response
type ResponseFrame struct {
	JSONRPC string          `json:"jsonrpc"`          // Must be "2.0"
	ID      string          `json:"id"`               // Request identifier
	Result  json.RawMessage `json:"result,omitempty"` // Success result
	Error   *ErrorShape     `json:"error,omitempty"`  // Error (mutually exclusive with Result)
}

// EventFrame represents a server-pushed event (no ID)
type EventFrame struct {
	JSONRPC string          `json:"jsonrpc"` // Must be "2.0"
	Method  string          `json:"method"`  // Event type
	Params  json.RawMessage `json:"params"`  // Event data
}

// ErrorShape defines the JSON-RPC error format
type ErrorShape struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Error Codes (JSON-RPC + Ghost-specific)
// ----------------------------------------

const (
	// Standard JSON-RPC errors
	ErrCodeParseError     = -32700
	ErrCodeInvalidRequest = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInvalidParams  = -32602
	ErrCodeInternalError  = -32603

	// Ghost-specific errors (application range: -32000 to -32099)
	ErrCodeAuthFailed       = -32001
	ErrCodePermissionDenied = -32002
	ErrCodeFocusMismatch    = -32003
	ErrCodeRiskBlocked      = -32004
	ErrCodeTimeout          = -32005
	ErrCodeVoiceWakeError   = -32006
	ErrCodeMemoryError      = -32007
)

// Authentication
// --------------

// ConnectParams is the initial handshake message
type ConnectParams struct {
	Token           string `json:"token"`            // Auth token
	ClientID        string `json:"client_id"`        // Unique client identifier
	ClientType      string `json:"client_type"`      // "brain", "sentinel", "ears", "external"
	ProtocolVersion string `json:"protocol_version"` // Must match server version
}

// ConnectResult is returned on successful authentication
type ConnectResult struct {
	SessionID     string    `json:"session_id"`
	ServerVersion string    `json:"server_version"`
	ExpiresAt     time.Time `json:"expires_at"`
	Capabilities  []string  `json:"capabilities"` // Available methods for this client
}

// Voice Wake (VA Tactical - P0)
// -----------------------------

// WakeParams activates voice wake detection
type WakeParams struct {
	Keyword       string `json:"keyword"`         // Wake phrase, e.g., "hey ghost"
	AudioStreamID string `json:"audio_stream_id"` // Mic source identifier
	PatienceMs    int    `json:"patience_ms"`     // Timeout before giving up (TBI adaptive: 3000-15000)
}

// WakeResult confirms wake activation
type WakeResult struct {
	Active   bool   `json:"active"`
	StreamID string `json:"stream_id"`
}

// TalkModeParams starts/stops continuous listening
type TalkModeParams struct {
	Enabled    bool   `json:"enabled"`
	SessionID  string `json:"session_id"`
	PatienceMs int    `json:"patience_ms"` // Adaptive for TBI users
	AutoExtend bool   `json:"auto_extend"` // Auto-extend on slow speech detection
}

// TalkModeResult confirms talk mode state
type TalkModeResult struct {
	Active    bool      `json:"active"`
	StartedAt time.Time `json:"started_at"`
}

// Execution Approvals (Conscience Kernel)
// ----------------------------------------

// ExecApprovalRequestParams requests permission for an action
type ExecApprovalRequestParams struct {
	RequestID      string          `json:"request_id"`
	Intent         string          `json:"intent"`
	Actions        json.RawMessage `json:"actions"`
	ExpectedWindow string          `json:"expected_window,omitempty"`
	RiskLevel      int             `json:"risk_level"` // 1-10 scale for VA Conscience Kernel
	TraceID        string          `json:"trace_id,omitempty"`
}

// ExecApprovalResolveParams resolves a pending approval
type ExecApprovalResolveParams struct {
	RequestID string `json:"request_id"`
	Approved  bool   `json:"approved"`
	Reason    string `json:"reason,omitempty"`
	UserID    string `json:"user_id,omitempty"` // Who approved (for audit)
}

// ExecApprovalResult is returned after approval decision
type ExecApprovalResult struct {
	RequestID  string `json:"request_id"`
	Approved   bool   `json:"approved"`
	Reason     string `json:"reason,omitempty"`
	TrustScore int    `json:"trust_score"`
	ErrorCode  string `json:"error_code,omitempty"`
}

// Memory Operations
// -----------------

// MemoryStoreParams stores a key-value fact
type MemoryStoreParams struct {
	Key     string    `json:"key"`
	Value   string    `json:"value"`
	Context string    `json:"context"`
	Vector  []float32 `json:"vector,omitempty"` // Embedding for RAG
	TTLDays int       `json:"ttl_days,omitempty"`
}

// MemoryStoreResult confirms storage
type MemoryStoreResult struct {
	Success    bool   `json:"success"`
	ArtifactID string `json:"artifact_id,omitempty"`
}

// MemorySearchParams searches for similar memories
type MemorySearchParams struct {
	Query  string    `json:"query,omitempty"`  // Text query (converted to vector)
	Vector []float32 `json:"vector,omitempty"` // Direct vector search
	Limit  int       `json:"limit"`            // Max results
}

// MemorySearchResult returns matching artifacts
type MemorySearchResult struct {
	Artifacts []MemoryArtifact `json:"artifacts"`
}

// MemoryArtifact is a memory entry with similarity score
type MemoryArtifact struct {
	ID              string    `json:"id"`
	Key             string    `json:"key"`
	Value           string    `json:"value"`
	Context         string    `json:"context"`
	SimilarityScore float64   `json:"similarity_score,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// Focus State
// -----------

// FocusUpdateParams updates the current window focus
type FocusUpdateParams struct {
	WindowName  string    `json:"window_name"`
	ProcessName string    `json:"process_name,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// Sessions (Long-term Context for TBI Veterans)
// ----------------------------------------------

// SessionSnapshotParams requests a session state snapshot
type SessionSnapshotParams struct {
	SessionID      string `json:"session_id"`
	IncludeHistory bool   `json:"include_history"`
}

// SessionSnapshot represents current conversation state
type SessionSnapshot struct {
	SessionID      string          `json:"session_id"`
	ConversationID string          `json:"conversation_id"`
	Messages       json.RawMessage `json:"messages"`
	ActiveIntents  json.RawMessage `json:"active_intents,omitempty"`
	UserFacts      json.RawMessage `json:"user_facts,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	LastActiveAt   time.Time       `json:"last_active_at"`
}

// Event Types (Server-pushed)
// ---------------------------

// WakeDetectedEvent is pushed when wake word is detected
type WakeDetectedEvent struct {
	Timestamp     time.Time `json:"timestamp"`
	Keyword       string    `json:"keyword"`
	Confidence    float64   `json:"confidence"`
	AudioStreamID string    `json:"audio_stream_id"`
}

// FocusChangedEvent is pushed when window focus changes
type FocusChangedEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	WindowName  string    `json:"window_name"`
	ProcessName string    `json:"process_name"`
}

// ApprovalPendingEvent is pushed when action needs approval
type ApprovalPendingEvent struct {
	RequestID string    `json:"request_id"`
	Intent    string    `json:"intent"`
	RiskLevel int       `json:"risk_level"`
	Timestamp time.Time `json:"timestamp"`
}

// TickEvent is a heartbeat event for connection health
type TickEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Uptime    int64     `json:"uptime_seconds"`
}

// Session Updates (Streaming Text)
// ---------------------------------

// SessionUpdateParams is used for streaming text updates during a session
type SessionUpdateParams struct {
	SessionID  string    `json:"session_id"`
	MessageID  string    `json:"message_id"`
	Delta      string    `json:"delta"`       // Incremental text chunk
	IsComplete bool      `json:"is_complete"` // True when message is fully streamed
	Role       string    `json:"role"`        // "user", "assistant", "system"
	Timestamp  time.Time `json:"timestamp"`
}

// SessionUpdateEvent is pushed during streaming responses
type SessionUpdateEvent struct {
	SessionID  string    `json:"session_id"`
	MessageID  string    `json:"message_id"`
	Delta      string    `json:"delta"`
	IsComplete bool      `json:"is_complete"`
	Role       string    `json:"role"`
	Timestamp  time.Time `json:"timestamp"`
}

// Action Validation (Conscience Kernel)
// --------------------------------------

// RiskLevel defines the severity of an action
type RiskLevel int

const (
	RiskLevelNone     RiskLevel = 0
	RiskLevelLow      RiskLevel = 1  // Safe actions: read, navigate
	RiskLevelMedium   RiskLevel = 3  // Moderate: type text, click buttons
	RiskLevelHigh     RiskLevel = 7  // Dangerous: delete, submit forms
	RiskLevelCritical RiskLevel = 10 // Critical: financial, credentials
)

// LegacyAction represents a single executable action from the Brain (JSON-RPC legacy)
// Note: For gRPC, use the proto-generated Action type in ghost.pb.go
type LegacyAction struct {
	Type           string          `json:"type"`              // "CLICK", "TYPE", "KEY", "HOTKEY", "SCROLL", "OPEN_URL", etc.
	Target         string          `json:"target,omitempty"`  // UI element or target
	Payload        json.RawMessage `json:"payload,omitempty"` // Action-specific data
	RiskLevel      RiskLevel       `json:"risk_level"`
	RequiresWindow string          `json:"requires_window,omitempty"` // Expected window for focus check
}

// ActionValidationResult is returned by ValidateAction
type ActionValidationResult struct {
	Valid      bool      `json:"valid"`
	Blocked    bool      `json:"blocked"`
	Reason     string    `json:"reason,omitempty"`
	RiskLevel  RiskLevel `json:"risk_level"`
	Override   bool      `json:"override"`    // True if Override key was provided
	TrustScore int       `json:"trust_score"` // Historical trust from intent history
}

// ActionValidationRequest is sent to the Conscience Kernel
type ActionValidationRequest struct {
	RequestID      string         `json:"request_id"`
	Intent         string         `json:"intent"`
	Actions        []LegacyAction `json:"actions"`
	ExpectedWindow string         `json:"expected_window,omitempty"`
	Override       bool           `json:"override"` // If true, bypass RiskLevel checks
	TraceID        string         `json:"trace_id,omitempty"`
}

// Client Registry Types
// ----------------------

// ClientInfo represents a registered client in the gateway
type ClientInfo struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Capabilities []string  `json:"capabilities"`
	ConnectedAt  time.Time `json:"connected_at"`
	LastSeen     time.Time `json:"last_seen"`
	Status       string    `json:"status"` // "connected", "idle", "busy"
}

// ClientRegistrySnapshot returns all connected clients
type ClientRegistrySnapshot struct {
	Clients   []ClientInfo `json:"clients"`
	Timestamp time.Time    `json:"timestamp"`
}
