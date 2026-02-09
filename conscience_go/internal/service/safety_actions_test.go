// Author: Enkae (enkae.dev@pm.me)
package service

import (
	"testing"
	pb "ghost/kernel/internal/protocol"
)

func TestValidateActions(t *testing.T) {
	tests := []struct {
		name           string
		actions        []*pb.Action
		config         SafetyConfig
		expectedResult bool
		expectedReason string
	}{
		{
			name: "Safe actions allowed",
			actions: []*pb.Action{
				{Type: "KEY", Payload: map[string]string{"key": "enter"}},
				{Type: "TYPE", Payload: map[string]string{"text": "hello"}},
			},
			config: DefaultSafetyConfig(),
			expectedResult: true,
			expectedReason: "",
		},
		{
			name: "Unknown action type blocked",
			actions: []*pb.Action{
				{Type: "EXEC", Payload: map[string]string{"cmd": "rm -rf /"}},
			},
			config: DefaultSafetyConfig(),
			expectedResult: false,
			expectedReason: "Action type not allowed: EXEC",
		},
		{
			name: "WRITE with safe path allowed",
			actions: []*pb.Action{
				{Type: "WRITE", Payload: map[string]string{"path": "data/notes.txt", "content": "hi"}},
			},
			config: DefaultSafetyConfig(),
			expectedResult: true,
			expectedReason: "",
		},
		{
			name: "WRITE with absolute path blocked",
			actions: []*pb.Action{
				{Type: "WRITE", Payload: map[string]string{"path": "/etc/passwd", "content": "bad"}},
			},
			config: DefaultSafetyConfig(),
			expectedResult: false,
			expectedReason: "Unsafe path detected: /etc/passwd",
		},
		{
			name: "WRITE with parent traversal blocked",
			actions: []*pb.Action{
				{Type: "WRITE", Payload: map[string]string{"path": "../secret.txt", "content": "bad"}},
			},
			config: DefaultSafetyConfig(),
			expectedResult: false,
			expectedReason: "Unsafe path detected: ../secret.txt",
		},
		{
			name: "READ with safe path allowed",
			actions: []*pb.Action{
				{Type: "READ", Payload: map[string]string{"path": "data/config.json"}},
			},
			config: DefaultSafetyConfig(),
			expectedResult: true,
			expectedReason: "",
		},
		{
			name: "READ with absolute path blocked",
			actions: []*pb.Action{
				{Type: "READ", Payload: map[string]string{"path": "/etc/shadow"}},
			},
			config: DefaultSafetyConfig(),
			expectedResult: false,
			expectedReason: "Unsafe path detected: /etc/shadow",
		},
		{
			name: "SEARCH with safe directory allowed",
			actions: []*pb.Action{
				{Type: "SEARCH", Payload: map[string]string{"directory": "src", "pattern": "*.go"}},
			},
			config: DefaultSafetyConfig(),
			expectedResult: true,
			expectedReason: "",
		},
		{
			name: "SEARCH with unsafe directory blocked",
			actions: []*pb.Action{
				{Type: "SEARCH", Payload: map[string]string{"directory": "../secret", "pattern": "*"}},
			},
			config: DefaultSafetyConfig(),
			expectedResult: false,
			expectedReason: "Unsafe path detected: ../secret",
		},
		{
			name: "Safe mode OFF allows dangerous actions",
			actions: []*pb.Action{
				{Type: "EXEC", Payload: map[string]string{"cmd": "rm -rf /"}},
			},
			config: SafetyConfig{SafeMode: false},
			expectedResult: true,
			expectedReason: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewSafetyChecker(tt.config)
			result, reason := checker.ValidateActions(tt.actions)

			if result != tt.expectedResult {
				t.Errorf("Expected result: %v, Got: %v", tt.expectedResult, result)
			}

			if reason != tt.expectedReason {
				t.Errorf("Expected reason: %q, Got: %q", tt.expectedReason, reason)
			}
		})
	}
}
