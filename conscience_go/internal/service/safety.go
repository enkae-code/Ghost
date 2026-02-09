// Author: Enkae (enkae.dev@pm.me)
package service

import (
	"strings"

	pb "ghost/kernel/internal/protocol"
)

// SafetyConfig defines rules for the SafetyChecker
type SafetyConfig struct {
	SafeMode        bool
	BlockedKeywords []string
}

// DefaultSafetyConfig returns strict defaults
func DefaultSafetyConfig() SafetyConfig {
	return SafetyConfig{
		SafeMode:        true,
		BlockedKeywords: []string{"delete", "rm ", "format ", "shutdown", "reboot", "sudo"},
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

// ValidateActions validates a slice of actions for safety, checking for nil elements
func (s *SafetyChecker) ValidateActions(actions []*pb.Action) (bool, string) {
	if actions == nil {
		return true, ""
	}
	for _, action := range actions {
		if action == nil {
			return false, "Nil action in request"
		}
		valid, reason := s.ValidateAction(action)
		if !valid {
			return false, reason
		}
	}
	return true, ""
}

// ValidateAction checks if a single action is safe and allowed
func (s *SafetyChecker) ValidateAction(action *pb.Action) (bool, string) {
	if action == nil {
		return false, "Nil action in request"
	}

	// Basic safety check: reject direct shell execution
	actionType := strings.ToUpper(action.Type)
	if actionType == "EXEC" || actionType == "SHELL" {
		return false, "Direct execution (EXEC/SHELL) is prohibited for safety"
	}

	return true, ""
}
