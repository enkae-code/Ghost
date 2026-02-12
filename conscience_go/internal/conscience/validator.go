// Author: Enkae (enkae.dev@pm.me)
// Package conscience implements the Ghost Conscience Kernel - the safety layer
// that validates all actions before they are sent to the Body (Sentinel).
//
// Based on Clawd Bot's exec-approval-manager.ts, translated to Go.
//
// Rules:
// 1. ALL Action requests must pass through ValidateAction() before routing to Body
// 2. If RiskLevel > High (7+), reject automatically unless Override key is present
// 3. Actions are logged for audit and trust score calculation
package conscience

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ghost/kernel/internal/protocol"

	"github.com/google/uuid"
)

const (
	// MalformedJSONFallback is used as Target when request unmarshaling fails
	MalformedJSONFallback = "MALFORMED_JSON_FALLBACK"
)

// BlockedKeywords are patterns that trigger automatic rejection
var BlockedKeywords = []string{
	"password", "credential", "secret", "api_key", "token",
	"credit_card", "ssn", "social_security",
	"delete_all", "drop_table", "rm -rf",
	"format", "fdisk",
}

// DangerousActionTypes require extra scrutiny
var DangerousActionTypes = map[string]protocol.RiskLevel{
	"DELETE":      protocol.RiskLevelHigh,
	"SUBMIT":      protocol.RiskLevelMedium,
	"TYPE":        protocol.RiskLevelLow,
	"CLICK":       protocol.RiskLevelLow,
	"KEY":         protocol.RiskLevelLow,
	"HOTKEY":      protocol.RiskLevelMedium, // Could be Ctrl+A Ctrl+V etc.
	"OPEN_URL":    protocol.RiskLevelMedium,
	"SCROLL":      protocol.RiskLevelNone,
	"SCREENSHOT":  protocol.RiskLevelNone,
	"WAIT":        protocol.RiskLevelNone,
	"FOCUS":       protocol.RiskLevelNone,
	"FILE_WRITE":  protocol.RiskLevelHigh,
	"FILE_DELETE": protocol.RiskLevelCritical,
	"EXECUTE":     protocol.RiskLevelCritical,
	// Add mapping for Brain action types
	"WRITE":       protocol.RiskLevelHigh,   // Maps to FILE_WRITE
	"EDIT":        protocol.RiskLevelHigh,   // Maps to file edit
	"READ":        protocol.RiskLevelMedium, // Maps to file read
	"LIST":        protocol.RiskLevelLow,    // Maps to file list
	"SEARCH":      protocol.RiskLevelLow,    // Maps to file search
	"SCAN":        protocol.RiskLevelNone,   // Visual scan
	"SPEAK":       protocol.RiskLevelNone,   // Audio output
	"MEMORIZE":    protocol.RiskLevelNone,   // Memory operation
}

// AllowedActionTypes is the strict allowlist of actions
var AllowedActionTypes = map[string]bool{
	"KEY": true, "TYPE": true, "CLICK": true, "WAIT": true, "SPEAK": true,
	"MEMORIZE": true, "SCAN": true, "LIST": true, "READ": true,
	"SEARCH": true, "WRITE": true, "EDIT": true,
}

// Validator is the Conscience Kernel that validates all actions
type Validator struct {
	mu              sync.RWMutex
	pendingRequests map[string]*PendingRequest
	focusedWindow   string
	trustScores     map[string]int // intent -> trust score
	auditLog        []AuditEntry
}

// PendingRequest tracks an action awaiting approval
type PendingRequest struct {
	ID         string
	Request    *protocol.ActionValidationRequest
	CreatedAt  time.Time
	ResolvedAt *time.Time
	Approved   bool
	Reason     string
}

// AuditEntry logs action validations
type AuditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	RequestID string    `json:"request_id"`
	Intent    string    `json:"intent"`
	RiskLevel int       `json:"risk_level"`
	Blocked   bool      `json:"blocked"`
	Reason    string    `json:"reason,omitempty"`
	Override  bool      `json:"override"`
}

// NewValidator creates a new Conscience Kernel validator
func NewValidator() *Validator {
	return &Validator{
		pendingRequests: make(map[string]*PendingRequest),
		trustScores:     make(map[string]int),
		auditLog:        make([]AuditEntry, 0, 1000),
	}
}

// SetFocusedWindow updates the current focus state
func (v *Validator) SetFocusedWindow(window string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.focusedWindow = window
}

// ValidateAction is the core function - ALL actions MUST pass through here
func (v *Validator) ValidateAction(ctx context.Context, req *protocol.ActionValidationRequest) *protocol.ActionValidationResult {
	if req == nil {
		return &protocol.ActionValidationResult{
			Valid:   false,
			Blocked: true,
			Reason:  "Nil validation request",
		}
	}
	// 1. Calculate maximum risk level and check blocked keywords (No lock needed)
	maxRisk := protocol.RiskLevelNone
	for i := range req.Actions {
		action := &req.Actions[i]

		// Enforce Allowlist
		actionType := strings.ToUpper(action.Type)
		if !AllowedActionTypes[actionType] {
			v.mu.Lock()
			defer v.mu.Unlock()
			result := &protocol.ActionValidationResult{
				Valid:      false,
				Blocked:    true,
				Reason:     fmt.Sprintf("Action type '%s' is not allowed", action.Type),
				RiskLevel:  protocol.RiskLevelCritical,
			}
			v.logAudit(req, result)
			return result
		}

		// Validate File System Paths
		if err := v.validateActionPath(action); err != nil {
			v.mu.Lock()
			defer v.mu.Unlock()
			result := &protocol.ActionValidationResult{
				Valid:      false,
				Blocked:    true,
				Reason:     fmt.Sprintf("Path validation failed for action %d: %v", i, err),
				RiskLevel:  protocol.RiskLevelCritical,
			}
			v.logAudit(req, result)
			return result
		}

		actionRisk := v.evaluateActionRisk(action)
		if actionRisk > maxRisk {
			maxRisk = actionRisk
		}

		// Check for blocked keywords in action payload
		if v.containsBlockedKeyword(action) {
			v.mu.Lock()
			defer v.mu.Unlock()
			result := &protocol.ActionValidationResult{
				Valid:      false,
				Blocked:    true,
				Override:   req.Override,
				TrustScore: v.getTrustScore(req.Intent),
				Reason:     fmt.Sprintf("Action %d contains blocked keyword pattern", i),
				RiskLevel:  protocol.RiskLevelCritical,
			}
			v.logAudit(req, result)
			return result
		}
	}

	// 2. State-dependent validation (Lock needed)
	v.mu.Lock()
	defer v.mu.Unlock()

	result := &protocol.ActionValidationResult{
		Valid:      true,
		Blocked:    false,
		Override:   req.Override,
		TrustScore: v.getTrustScore(req.Intent),
		RiskLevel:  maxRisk,
	}

	// Rule: RiskLevel > High (7+) requires Override
	if maxRisk >= protocol.RiskLevelHigh && !req.Override {
		result.Valid = false
		result.Blocked = true
		result.Reason = fmt.Sprintf("High risk action (level %d) requires explicit override", maxRisk)
		slog.Warn("Action blocked by Conscience Kernel",
			"request_id", req.RequestID,
			"intent", req.Intent,
			"risk_level", maxRisk,
		)
		v.logAudit(req, result)
		return result
	}

	// Check focus window if required
	if req.ExpectedWindow != "" && v.focusedWindow != "" {
		if !strings.Contains(strings.ToLower(v.focusedWindow), strings.ToLower(req.ExpectedWindow)) {
			result.Valid = false
			result.Blocked = true
			result.Reason = fmt.Sprintf("Focus mismatch: expected '%s', got '%s'", req.ExpectedWindow, v.focusedWindow)
			v.logAudit(req, result)
			return result
		}
	}

	// Store as pending request (for UI approval if needed)
	pending := &PendingRequest{
		ID:        req.RequestID,
		Request:   req,
		CreatedAt: time.Now(),
	}
	v.pendingRequests[req.RequestID] = pending

	slog.Info("Action validated by Conscience Kernel",
		"request_id", req.RequestID,
		"intent", req.Intent,
		"risk_level", maxRisk,
		"override", req.Override,
	)

	v.logAudit(req, result)
	return result
}

// validateActionPath checks for safe file system paths
func (v *Validator) validateActionPath(action *protocol.LegacyAction) error {
	actionType := strings.ToUpper(action.Type)

	// Parse payload map (LegacyAction payload is json.RawMessage, need to unmarshal)
	var payload map[string]interface{}
	if len(action.Payload) > 0 {
		if err := json.Unmarshal(action.Payload, &payload); err != nil {
			return fmt.Errorf("invalid payload json")
		}
	} else {
		// Actions that require paths must have a payload
		if actionType == "WRITE" || actionType == "EDIT" || actionType == "READ" || actionType == "LIST" || actionType == "SEARCH" {
			return fmt.Errorf("missing payload for action type '%s'", actionType)
		}
		return nil
	}

	// Helper to check path
	checkPath := func(key string) error {
		val, ok := payload[key]
		if !ok {
			return fmt.Errorf("missing required key '%s'", key)
		}
		pathStr, ok := val.(string)
		if !ok {
			return fmt.Errorf("key '%s' must be a string", key)
		}
		if !v.validateFileSystemPath(pathStr) {
			return fmt.Errorf("invalid path '%s' (must be relative and safe)", pathStr)
		}
		return nil
	}

	switch actionType {
	case "WRITE", "EDIT", "READ":
		return checkPath("path")
	case "LIST":
		// Prioritize directory, fallback to path
		if _, ok := payload["directory"]; ok {
			return checkPath("directory")
		}
		return checkPath("path")
	case "SEARCH":
		return checkPath("directory")
	}

	return nil
}

// validateFileSystemPath ensures path is relative and safe
func (v *Validator) validateFileSystemPath(path string) bool {
	// 1. Prohibit prefixes like /, \, or Windows drive letters
	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "\\") || (len(path) > 1 && path[1] == ':') {
		return false
	}

	// 2. Check for directory traversal (..)
	// filepath.Clean resolves .. but we want to detect if it tries to escape
	// A simple check is to look for ".." in the path components
	// But explicitly, we want to ensure it's relative.
	if filepath.IsAbs(path) {
		return false
	}

	// Clean the path to remove redundant separators and resolve . and ..
	cleanPath := filepath.Clean(path)

	// If clean path starts with .. or is absolute, it's unsafe
	if strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(cleanPath) {
		return false
	}

	// Additional check for Windows drive letters in cleaned path (just in case)
	if len(cleanPath) > 1 && cleanPath[1] == ':' {
		return false
	}

	return true
}

// evaluateActionRisk determines the risk level of a single action
func (v *Validator) evaluateActionRisk(action *protocol.LegacyAction) protocol.RiskLevel {
	if action == nil {
		return protocol.RiskLevelNone
	}
	// First check the action's declared risk level
	if action.RiskLevel > protocol.RiskLevelNone {
		return action.RiskLevel
	}

	// Fall back to type-based risk assessment
	actionType := strings.ToUpper(action.Type)
	if risk, exists := DangerousActionTypes[actionType]; exists {
		return risk
	}

	return protocol.RiskLevelLow // Default to low for unknown types
}

// containsBlockedKeyword checks if an action contains dangerous patterns
func (v *Validator) containsBlockedKeyword(action *protocol.LegacyAction) bool {
	if action == nil {
		return false
	}
	// Check target
	targetLower := strings.ToLower(action.Target)
	for _, keyword := range BlockedKeywords {
		if strings.Contains(targetLower, keyword) {
			return true
		}
	}

	// Check payload
	if len(action.Payload) > 0 {
		payloadStr := strings.ToLower(string(action.Payload))
		for _, keyword := range BlockedKeywords {
			if strings.Contains(payloadStr, keyword) {
				return true
			}
		}
	}

	return false
}

// getTrustScore returns historical trust for an intent
func (v *Validator) getTrustScore(intent string) int {
	if score, exists := v.trustScores[intent]; exists {
		return score
	}
	return 0
}

// RecordSuccess increases trust score for successful intent completion
func (v *Validator) RecordSuccess(intent string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if _, exists := v.trustScores[intent]; exists {
		v.trustScores[intent] += 10
		if v.trustScores[intent] > 100 {
			v.trustScores[intent] = 100
		}
	} else {
		v.trustScores[intent] = 10
	}
}

// ResolveRequest marks a pending request as resolved
func (v *Validator) ResolveRequest(requestID string, approved bool, reason string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	pending, exists := v.pendingRequests[requestID]
	if !exists {
		return fmt.Errorf("request %s not found", requestID)
	}

	now := time.Now()
	pending.ResolvedAt = &now
	pending.Approved = approved
	pending.Reason = reason

	slog.Info("Request resolved",
		"request_id", requestID,
		"approved", approved,
		"reason", reason,
	)

	return nil
}

// logAudit records an action validation for audit trail
func (v *Validator) logAudit(req *protocol.ActionValidationRequest, result *protocol.ActionValidationResult) {
	entry := AuditEntry{
		Timestamp: time.Now(),
		RequestID: req.RequestID,
		Intent:    req.Intent,
		RiskLevel: int(result.RiskLevel),
		Blocked:   result.Blocked,
		Reason:    result.Reason,
		Override:  result.Override,
	}
	v.auditLog = append(v.auditLog, entry)

	// Trim audit log if too large
	if len(v.auditLog) > 1000 {
		v.auditLog = v.auditLog[len(v.auditLog)-500:]
	}
}

// GetAuditLog returns recent audit entries
func (v *Validator) GetAuditLog(limit int) []AuditEntry {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if limit <= 0 || limit > len(v.auditLog) {
		limit = len(v.auditLog)
	}

	return v.auditLog[len(v.auditLog)-limit:]
}

// --- Implement gateway.ApprovalHandler interface ---

// RequestApproval handles incoming exec.request from the gateway
func (v *Validator) RequestApproval(ctx context.Context, req *protocol.ExecApprovalRequestParams) (*protocol.ExecApprovalResult, error) {
	// Convert to ActionValidationRequest
	var actions []protocol.LegacyAction
	if err := json.Unmarshal(req.Actions, &actions); err != nil {
		// Fallback: treat as single action
		// Include raw actions in payload to ensure keyword checking still happens
		actions = []protocol.LegacyAction{{
			Type:      "UNKNOWN",
			RiskLevel: protocol.RiskLevel(req.RiskLevel),
			Target:    MalformedJSONFallback,
			Payload:   req.Actions,
		}}
	}

	validationReq := &protocol.ActionValidationRequest{
		RequestID:      req.RequestID,
		Intent:         req.Intent,
		Actions:        actions,
		ExpectedWindow: req.ExpectedWindow,
		Override:       false, // No override by default
		TraceID:        req.TraceID,
	}

	// Generate request ID if not provided
	if validationReq.RequestID == "" {
		validationReq.RequestID = uuid.New().String()
	}

	result := v.ValidateAction(ctx, validationReq)

	return &protocol.ExecApprovalResult{
		RequestID:  validationReq.RequestID,
		Approved:   result.Valid && !result.Blocked,
		Reason:     result.Reason,
		TrustScore: result.TrustScore,
	}, nil
}

// ResolveApproval handles exec.resolve from the gateway
func (v *Validator) ResolveApproval(ctx context.Context, req *protocol.ExecApprovalResolveParams) error {
	return v.ResolveRequest(req.RequestID, req.Approved, req.Reason)
}
