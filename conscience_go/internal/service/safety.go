package service

import (
	"strings"
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
