// Author: Enkae (enkae.dev@pm.me)
package service

import (
	"testing"
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
