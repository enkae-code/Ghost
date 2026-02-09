// Author: Enkae (enkae.dev@pm.me)
package service

import (
	"testing"

	pb "ghost/kernel/internal/protocol"
)

func TestIsDangerous(t *testing.T) {
	tests := []struct {
		name           string
		intent         string
		config         SafetyConfig
		expectedResult bool
		expectedKw     string
		description    string
	}{
		{
			name:   "Safe mode ON - blocks 'delete'",
			intent: "delete the file",
			config: SafetyConfig{
				SafeMode:        true,
				BlockedKeywords: []string{"delete"},
			},
			expectedResult: true,
			expectedKw:     "delete",
			description:    "Should block intent containing blocked keyword",
		},
		{
			name:   "Safe mode ON - case insensitive",
			intent: "PLEASE DELETE THIS",
			config: SafetyConfig{
				SafeMode:        true,
				BlockedKeywords: []string{"delete"},
			},
			expectedResult: true,
			expectedKw:     "delete",
			description:    "Should block dangerous keywords regardless of case",
		},
		{
			name:   "Safe mode OFF - allows dangerous",
			intent: "delete everything",
			config: SafetyConfig{
				SafeMode:        false,
				BlockedKeywords: []string{"delete"},
			},
			expectedResult: false,
			expectedKw:     "",
			description:    "Should allow dangerous keywords when safe mode is OFF",
		},
		{
			name:   "Safe mode ON - allows safe text",
			intent: "hello world",
			config: SafetyConfig{
				SafeMode:        true,
				BlockedKeywords: []string{"delete"},
			},
			expectedResult: false,
			expectedKw:     "",
			description:    "Should allow safe text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewSafetyChecker(tt.config)
			result, kw := checker.IsDangerous(tt.intent)

			if result != tt.expectedResult {
				t.Errorf("%s\nExpected: %v, Got: %v", tt.description, tt.expectedResult, result)
			}

			if kw != tt.expectedKw {
				t.Errorf("%s\nExpected Keyword: %q, Got: %q", tt.description, tt.expectedKw, kw)
			}
		})
	}
}

func TestValidateActions(t *testing.T) {
	checker := NewSafetyChecker(DefaultSafetyConfig())

	tests := []struct {
		name          string
		actions       []*pb.Action
		expectedValid bool
		expectedReason string
	}{
		{
			name:          "Nil actions slice",
			actions:       nil,
			expectedValid: true,
		},
		{
			name:          "Empty actions slice",
			actions:       []*pb.Action{},
			expectedValid: true,
		},
		{
			name: "Single valid action",
			actions: []*pb.Action{
				{Type: "CLICK"},
			},
			expectedValid: true,
		},
		{
			name: "Multiple valid actions",
			actions: []*pb.Action{
				{Type: "TYPE"},
				{Type: "WAIT"},
			},
			expectedValid: true,
		},
		{
			name: "Action with nil element",
			actions: []*pb.Action{
				{Type: "CLICK"},
				nil,
			},
			expectedValid: false,
			expectedReason: "Nil action in request",
		},
		{
			name: "Blocked action type EXEC",
			actions: []*pb.Action{
				{Type: "EXEC"},
			},
			expectedValid: false,
			expectedReason: "Direct execution (EXEC/SHELL) is prohibited for safety",
		},
		{
			name: "Blocked action type SHELL",
			actions: []*pb.Action{
				{Type: "SHELL"},
			},
			expectedValid: false,
			expectedReason: "Direct execution (EXEC/SHELL) is prohibited for safety",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, reason := checker.ValidateActions(tt.actions)
			if valid != tt.expectedValid {
				t.Errorf("ValidateActions() valid = %v, want %v", valid, tt.expectedValid)
			}
			if tt.expectedReason != "" && reason != tt.expectedReason {
				t.Errorf("ValidateActions() reason = %q, want %q", reason, tt.expectedReason)
			}
		})
	}
}
