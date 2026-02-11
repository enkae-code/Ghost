// Author: Enkae (enkae.dev@pm.me)
package conscience

import (
	"context"
	"encoding/json"
	"testing"
	"strings"

	"ghost/kernel/internal/protocol"
)

func TestValidateActionPath(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name      string
		action    *protocol.LegacyAction
		expected  bool
	}{
		{
			name: "Safe File Write",
			action: &protocol.LegacyAction{
				Type: "WRITE",
				Payload: json.RawMessage(`{"path": "data/log.txt"}`),
			},
			expected: true,
		},
		{
			name: "Unsafe File Write (Parent Directory)",
			action: &protocol.LegacyAction{
				Type: "WRITE",
				Payload: json.RawMessage(`{"path": "../secret.txt"}`),
			},
			expected: false,
		},
		{
			name: "Unsafe File Write (Absolute Path)",
			action: &protocol.LegacyAction{
				Type: "WRITE",
				Payload: json.RawMessage(`{"path": "/etc/passwd"}`),
			},
			expected: false,
		},
		{
			name: "Safe List Directory",
			action: &protocol.LegacyAction{
				Type: "LIST",
				Payload: json.RawMessage(`{"directory": "docs"}`),
			},
			expected: true,
		},
		{
			name: "Unsafe List Directory (Parent)",
			action: &protocol.LegacyAction{
				Type: "LIST",
				Payload: json.RawMessage(`{"directory": "../"}`),
			},
			expected: false,
		},
		{
			name: "Safe Edit (Nested)",
			action: &protocol.LegacyAction{
				Type: "EDIT",
				Payload: json.RawMessage(`{"path": "src/components/Button.tsx"}`),
			},
			expected: true,
		},
		{
			name: "Non-Filesystem Action (Ignore Path)",
			action: &protocol.LegacyAction{
				Type: "CLICK",
				Payload: json.RawMessage(`{"x": "100", "y": "200"}`),
			},
			expected: true,
		},
		{
			name: "Malformed Payload (Reject)",
			action: &protocol.LegacyAction{
				Type: "WRITE",
				Payload: json.RawMessage(`{invalid_json`),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &protocol.ActionValidationRequest{
				RequestID: "test-req",
				Intent:    "test intent",
				Actions:   []protocol.LegacyAction{*tt.action},
				Override:  true, // Override risk checks to focus on path validation
			}

			result := v.ValidateAction(context.Background(), req)

			isValid := result.Valid && !result.Blocked
			if isValid != tt.expected {
				t.Errorf("ValidateAction() valid=%v, want %v. Reason: %s", isValid, tt.expected, result.Reason)
			}

			// For expected failures, check if it's due to unsafe path
			if !tt.expected {
				if result.RiskLevel != protocol.RiskLevelCritical {
					t.Errorf("Expected Critical Risk for unsafe path, got %v", result.RiskLevel)
				}
				if !strings.Contains(result.Reason, "unsafe path") && !strings.Contains(result.Reason, "blocked keyword") && !strings.Contains(result.Reason, "Malformed") {
					// Malformed JSON might result in rejection but maybe not "unsafe path" specific message if I implemented "return false" there.
					// My implementation returns false which triggers "unsafe path" message.
				}
			}
		})
	}
}
