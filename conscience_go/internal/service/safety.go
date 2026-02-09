// Author: Enkae (enkae.dev@pm.me)
package service

import (
	"path/filepath"
	"strings"

	pb "ghost/kernel/internal/protocol"
)

// SafetyConfig defines rules for the SafetyChecker
type SafetyConfig struct {
	SafeMode        bool
	BlockedKeywords []string
	AllowedActions  map[string]bool
}

// DefaultSafetyConfig returns strict defaults
func DefaultSafetyConfig() SafetyConfig {
	return SafetyConfig{
		SafeMode:        true,
		BlockedKeywords: []string{"delete", "rm ", "format ", "shutdown", "reboot", "sudo"},
		AllowedActions: map[string]bool{
			"KEY":      true,
			"TYPE":     true,
			"CLICK":    true,
			"WAIT":     true,
			"SPEAK":    true,
			"MEMORIZE": true,
			"SCAN":     true,
			"LIST":     true,
			"READ":     true,
			"SEARCH":   true,
			"WRITE":    true, // Checked separately for path safety
			"EDIT":     true, // Checked separately for path safety
		},
	}
}

// SafetyChecker enforces security policies on intents
type SafetyChecker struct {
	config SafetyConfig
}

// NewSafetyChecker creates a checker with the given config
func NewSafetyChecker(config SafetyConfig) *SafetyChecker {
	return &SafetyChecker{config: config}
}

// IsDangerous checks if an intent contains blocked keywords when SafeMode is on
func (s *SafetyChecker) IsDangerous(intent string) (bool, string) {
	if !s.config.SafeMode {
		return false, ""
	}

	lowerIntent := strings.ToLower(intent)
	for _, keyword := range s.config.BlockedKeywords {
		if strings.Contains(lowerIntent, keyword) {
			return true, keyword
		}
	}

	return false, ""
}

// ValidateActions checks if a list of actions contains any forbidden operations
func (s *SafetyChecker) ValidateActions(actions []*pb.Action) (bool, string) {
	if !s.config.SafeMode {
		return true, ""
	}

	for _, action := range actions {
		valid, reason := s.ValidateAction(action)
		if !valid {
			return false, reason
		}
	}
	return true, ""
}

// ValidateAction checks if a single action is safe
func (s *SafetyChecker) ValidateAction(action *pb.Action) (bool, string) {
	if !s.config.SafeMode {
		return true, ""
	}

	actionType := strings.ToUpper(action.Type)

	// 1. Whitelist Check
	if !s.config.AllowedActions[actionType] {
		return false, "Action type not allowed: " + actionType
	}

	// 2. Path Safety Check (File Operations)
	if actionType == "WRITE" || actionType == "EDIT" || actionType == "READ" || actionType == "LIST" || actionType == "SEARCH" {
		// SEARCH uses 'directory' instead of 'path'
		pathKey := "path"
		if actionType == "SEARCH" {
			pathKey = "directory"
		}

		path := action.Payload[pathKey]
		if path == "" {
			return false, actionType + " action missing '" + pathKey + "'"
		}
		if !s.isSafePath(path) {
			return false, "Unsafe path detected: " + path
		}
	}

	return true, ""
}

// isSafePath ensures path is relative and does not traverse upwards
func (s *SafetyChecker) isSafePath(path string) bool {
	// Reject absolute paths
	if filepath.IsAbs(path) {
		return false
	}

	// Check for Windows drive letters (even if not absolute by filepath logic on Linux)
	if len(path) > 1 && path[1] == ':' {
		return false
	}

	// Check for root path
	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "\\") {
		return false
	}

	// Clean the path to resolve ..
	cleanPath := filepath.Clean(path)

	// Reject paths starting with .. (traversal)
	if strings.HasPrefix(cleanPath, "..") {
		return false
	}

	// Also check for explicit ".." in raw string to be safe against trickery
	if strings.Contains(path, "..") {
		return false
	}

	return true
}
