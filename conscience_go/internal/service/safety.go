// Author: Enkae (enkae.dev@pm.me)
package service

import (
	"strings"

	pb "ghost/kernel/internal/protocol"
)

// SafetyConfig defines rules for the SafetyChecker.
type SafetyConfig struct {
	// SafeMode enables or disables safety checks.
	SafeMode bool
	// BlockedKeywords is a list of substrings that trigger a safety violation if found in an intent.
	BlockedKeywords []string
	// AllowedActions is a set of action types that are permitted.
	AllowedActions map[string]bool
}

// DefaultSafetyConfig returns strict defaults for safety validation.
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
			"WRITE":    true,
			"EDIT":     true,
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

// IsDangerous checks if an intent contains blocked keywords when SafeMode is on.
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

// ValidateActions checks a slice of actions against safety policies.
func (s *SafetyChecker) ValidateActions(actions []*pb.Action) (bool, string) {
	if !s.config.SafeMode {
		return true, ""
	}

	for _, action := range actions {
		if ok, reason := s.ValidateAction(action); !ok {
			return false, reason
		}
	}

	return true, ""
}

// ValidateAction checks a single action against safety policies.
func (s *SafetyChecker) ValidateAction(action *pb.Action) (bool, string) {
	if !s.config.SafeMode {
		return true, ""
	}

	actionType := strings.ToUpper(action.Type)
	if !s.config.AllowedActions[actionType] {
		return false, "Action type '" + actionType + "' is not in the allowlist"
	}

	// Path safety checks for filesystem actions
	switch actionType {
	case "WRITE", "READ", "EDIT":
		path := action.Payload["path"]
		if !s.isSafePath(path) {
			return false, "Unsafe path in action payload: " + path
		}
	case "SEARCH", "LIST":
		dir := action.Payload["directory"]
		if dir == "" {
			dir = action.Payload["path"]
		}
		if !s.isSafePath(dir) {
			return false, "Unsafe directory in action payload: " + dir
		}
	}

	return true, ""
}

// isSafePath returns true if the path is relative and does not contain directory traversal.
func (s *SafetyChecker) isSafePath(path string) bool {
	if path == "" {
		return true
	}

	// No absolute paths (simple check for Unix and Windows)
	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "\\") || (len(path) > 1 && path[1] == ':') {
		return false
	}

	// No traversal
	if strings.Contains(path, "..") {
		return false
	}

	return true
}
