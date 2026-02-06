// Author: Enkae (enkae.dev@pm.me)
package main

import (
	"testing"
)

// TestIsDangerousAction uses table-driven tests to verify security logic
func TestIsDangerousAction(t *testing.T) {
	tests := []struct {
		name           string
		action         Action
		config         *Config
		expectedResult bool
		description    string
	}{
		{
			name: "Safe mode ON - blocks 'delete' keyword",
			action: Action{
				Type:    "TYPE",
				Payload: map[string]interface{}{"text": "delete file.txt"},
			},
			config: &Config{
				Security: struct {
					SafeMode        bool     `json:"safe_mode"`
					BlockedKeywords []string `json:"blocked_keywords"`
				}{
					SafeMode:        true,
					BlockedKeywords: []string{"delete", "rm ", "format ", "shutdown"},
				},
			},
			expectedResult: true,
			description:    "Should block text containing 'delete' when safe mode is ON",
		},
		{
			name: "Safe mode ON - blocks 'rm -rf' command",
			action: Action{
				Type:    "TYPE",
				Payload: map[string]interface{}{"text": "rm -rf /"},
			},
			config: &Config{
				Security: struct {
					SafeMode        bool     `json:"safe_mode"`
					BlockedKeywords []string `json:"blocked_keywords"`
				}{
					SafeMode:        true,
					BlockedKeywords: []string{"delete", "rm ", "format ", "shutdown"},
				},
			},
			expectedResult: true,
			description:    "Should block 'rm -rf' when safe mode is ON",
		},
		{
			name: "Safe mode OFF - allows dangerous keywords",
			action: Action{
				Type:    "TYPE",
				Payload: map[string]interface{}{"text": "delete file.txt"},
			},
			config: &Config{
				Security: struct {
					SafeMode        bool     `json:"safe_mode"`
					BlockedKeywords []string `json:"blocked_keywords"`
				}{
					SafeMode:        false,
					BlockedKeywords: []string{"delete", "rm ", "format ", "shutdown"},
				},
			},
			expectedResult: false,
			description:    "Should allow dangerous keywords when safe mode is OFF",
		},
		{
			name: "Safe mode ON - allows safe text",
			action: Action{
				Type:    "TYPE",
				Payload: map[string]interface{}{"text": "Hello World"},
			},
			config: &Config{
				Security: struct {
					SafeMode        bool     `json:"safe_mode"`
					BlockedKeywords []string `json:"blocked_keywords"`
				}{
					SafeMode:        true,
					BlockedKeywords: []string{"delete", "rm ", "format ", "shutdown"},
				},
			},
			expectedResult: false,
			description:    "Should allow safe text when safe mode is ON",
		},
		{
			name: "Safe mode ON - blocks action type with dangerous keyword",
			action: Action{
				Type:    "DELETE_FILE",
				Payload: map[string]interface{}{},
			},
			config: &Config{
				Security: struct {
					SafeMode        bool     `json:"safe_mode"`
					BlockedKeywords []string `json:"blocked_keywords"`
				}{
					SafeMode:        true,
					BlockedKeywords: []string{"delete", "rm ", "format ", "shutdown"},
				},
			},
			expectedResult: true,
			description:    "Should block action type containing dangerous keyword",
		},
		{
			name: "Safe mode ON - case insensitive matching",
			action: Action{
				Type:    "TYPE",
				Payload: map[string]interface{}{"text": "DELETE FILE.TXT"},
			},
			config: &Config{
				Security: struct {
					SafeMode        bool     `json:"safe_mode"`
					BlockedKeywords []string `json:"blocked_keywords"`
				}{
					SafeMode:        true,
					BlockedKeywords: []string{"delete", "rm ", "format ", "shutdown"},
				},
			},
			expectedResult: true,
			description:    "Should block dangerous keywords regardless of case",
		},
		{
			name: "Empty blocklist - allows everything",
			action: Action{
				Type:    "TYPE",
				Payload: map[string]interface{}{"text": "rm -rf /"},
			},
			config: &Config{
				Security: struct {
					SafeMode        bool     `json:"safe_mode"`
					BlockedKeywords []string `json:"blocked_keywords"`
				}{
					SafeMode:        true,
					BlockedKeywords: []string{},
				},
			},
			expectedResult: false,
			description:    "Should allow all actions when blocklist is empty",
		},
		{
			name: "Nil config - allows everything",
			action: Action{
				Type:    "TYPE",
				Payload: map[string]interface{}{"text": "delete everything"},
			},
			config:         nil,
			expectedResult: false,
			description:    "Should allow all actions when config is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set global config for test
			originalConfig := appConfig
			appConfig = tt.config
			defer func() { appConfig = originalConfig }()

			result := isDangerousAction(tt.action)

			if result != tt.expectedResult {
				t.Errorf("%s\nExpected: %v, Got: %v\nAction: %+v",
					tt.description, tt.expectedResult, result, tt.action)
			}
		})
	}
}

// TestConfigLoading verifies config loading with safe defaults
func TestConfigLoading(t *testing.T) {
	// This test verifies that loadConfig returns valid defaults when config.json is missing
	config, err := loadConfig()

	if err != nil {
		t.Fatalf("loadConfig should not return error with defaults: %v", err)
	}

	if config == nil {
		t.Fatal("loadConfig should return non-nil config")
	}

	// Verify defaults are set
	if config.System.Version == "" {
		t.Error("Default version should be set")
	}

	if config.Network.KernelPort == 0 {
		t.Error("Default kernel port should be set")
	}

	if !config.Security.SafeMode {
		t.Error("Safe mode should be enabled by default")
	}

	if len(config.Security.BlockedKeywords) == 0 {
		t.Error("Default blocklist should contain keywords")
	}
}

// BenchmarkIsDangerousAction measures performance of security checks
func BenchmarkIsDangerousAction(b *testing.B) {
	appConfig = &Config{
		Security: struct {
			SafeMode        bool     `json:"safe_mode"`
			BlockedKeywords []string `json:"blocked_keywords"`
		}{
			SafeMode:        true,
			BlockedKeywords: []string{"delete", "rm ", "format ", "shutdown", "reboot", "sudo", "admin"},
		},
	}

	action := Action{
		Type:    "TYPE",
		Payload: map[string]interface{}{"text": "This is a safe message with no dangerous keywords"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		isDangerousAction(action)
	}
}
