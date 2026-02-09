// Author: Enkae (enkae.dev@pm.me)
package service

import (
	pb "ghost/kernel/internal/protocol"
	"testing"
)

func TestValidateAction(t *testing.T) {
	tests := []struct {
		name     string
		action   *pb.Action
		config   SafetyConfig
		expected bool
	}{
		{
			name: "Allowed Action - CLICK",
			action: &pb.Action{
				Type: "CLICK",
			},
			config:   DefaultSafetyConfig(),
			expected: true,
		},
		{
			name: "Blocked Action - EXEC",
			action: &pb.Action{
				Type: "EXEC",
			},
			config:   DefaultSafetyConfig(),
			expected: false,
		},
		{
			name: "Safe Path - READ",
			action: &pb.Action{
				Type: "READ",
				Payload: map[string]string{
					"path": "docs/readme.md",
				},
			},
			config:   DefaultSafetyConfig(),
			expected: true,
		},
		{
			name: "Unsafe Path (Absolute Unix) - READ",
			action: &pb.Action{
				Type: "READ",
				Payload: map[string]string{
					"path": "/etc/passwd",
				},
			},
			config:   DefaultSafetyConfig(),
			expected: false,
		},
		{
			name: "Unsafe Path (Absolute Windows) - READ",
			action: &pb.Action{
				Type: "READ",
				Payload: map[string]string{
					"path": "\\Windows\\System32\\config",
				},
			},
			config:   DefaultSafetyConfig(),
			expected: false,
		},
		{
			name: "Unsafe Path (Drive Windows) - READ",
			action: &pb.Action{
				Type: "READ",
				Payload: map[string]string{
					"path": "C:\\Users\\Admin",
				},
			},
			config:   DefaultSafetyConfig(),
			expected: false,
		},
		{
			name: "Unsafe Path (Traversal) - WRITE",
			action: &pb.Action{
				Type: "WRITE",
				Payload: map[string]string{
					"path": "../secret.txt",
				},
			},
			config:   DefaultSafetyConfig(),
			expected: false,
		},
		{
			name: "Safe Path - EDIT",
			action: &pb.Action{
				Type: "EDIT",
				Payload: map[string]string{
					"path": "local/config.json",
				},
			},
			config:   DefaultSafetyConfig(),
			expected: true,
		},
		{
			name: "Unsafe Path - EDIT",
			action: &pb.Action{
				Type: "EDIT",
				Payload: map[string]string{
					"path": "/usr/local/bin/script",
				},
			},
			config:   DefaultSafetyConfig(),
			expected: false,
		},
		{
			name: "Unsafe Directory - SEARCH",
			action: &pb.Action{
				Type: "SEARCH",
				Payload: map[string]string{
					"directory": "/usr/bin",
				},
			},
			config:   DefaultSafetyConfig(),
			expected: false,
		},
		{
			name: "Safe Directory - LIST",
			action: &pb.Action{
				Type: "LIST",
				Payload: map[string]string{
					"directory": "my_folder",
				},
			},
			config:   DefaultSafetyConfig(),
			expected: true,
		},
		{
			name: "Unsafe Directory - LIST (via path key)",
			action: &pb.Action{
				Type: "LIST",
				Payload: map[string]string{
					"path": "/home/user",
				},
			},
			config:   DefaultSafetyConfig(),
			expected: false,
		},
		{
			name: "Safe Mode Off - Allows anything",
			action: &pb.Action{
				Type: "EXEC",
			},
			config: SafetyConfig{
				SafeMode: false,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSafetyChecker(tt.config)
			ok, _ := s.ValidateAction(tt.action)
			if ok != tt.expected {
				t.Errorf("%s: expected %v, got %v", tt.name, tt.expected, ok)
			}
		})
	}
}

func TestValidateActions(t *testing.T) {
	config := DefaultSafetyConfig()
	s := NewSafetyChecker(config)

	tests := []struct {
		name     string
		actions  []*pb.Action
		expected bool
	}{
		{
			name: "All Safe Actions",
			actions: []*pb.Action{
				{Type: "CLICK"},
				{Type: "TYPE", Payload: map[string]string{"text": "hello"}},
			},
			expected: true,
		},
		{
			name: "One Unsafe Action",
			actions: []*pb.Action{
				{Type: "CLICK"},
				{Type: "EXEC"},
			},
			expected: false,
		},
		{
			name:     "Empty Actions",
			actions:  []*pb.Action{},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, _ := s.ValidateActions(tt.actions)
			if ok != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, ok)
			}
		})
	}
}
