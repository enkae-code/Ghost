// Author: Enkae (enkae.dev@pm.me)
package service

import (
	pb "ghost/kernel/internal/protocol"
	"testing"
)

func TestValidateAction_Extra(t *testing.T) {
	tests := []struct {
		name     string
		action   *pb.Action
		config   SafetyConfig
		expected bool
	}{
		// Directory Traversal
		{
			name: "Nested traversal - READ",
			action: &pb.Action{
				Type: "READ",
				Payload: map[string]string{
					"path": "foo/../bar",
				},
			},
			config:   DefaultSafetyConfig(),
			expected: false,
		},
		{
			name: "Nested traversal mixed slash - READ",
			action: &pb.Action{
				Type: "READ",
				Payload: map[string]string{
					"path": "foo\\..\\bar",
				},
			},
			config:   DefaultSafetyConfig(),
			expected: false,
		},
		{
			name: "Nested traversal at end - READ",
			action: &pb.Action{
				Type: "READ",
				Payload: map[string]string{
					"path": "foo/..",
				},
			},
			config:   DefaultSafetyConfig(),
			expected: false,
		},

		// Absolute Paths
		{
			name: "Root absolute path - READ",
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
			name: "Windows root absolute path - READ",
			action: &pb.Action{
				Type: "READ",
				Payload: map[string]string{
					"path": "\\Windows",
				},
			},
			config:   DefaultSafetyConfig(),
			expected: false,
		},

		// Drive Letters
		{
			name: "Drive letter Windows style - READ",
			action: &pb.Action{
				Type: "READ",
				Payload: map[string]string{
					"path": "C:\\Windows",
				},
			},
			config:   DefaultSafetyConfig(),
			expected: false,
		},
		{
			name: "Drive letter mixed style - READ",
			action: &pb.Action{
				Type: "READ",
				Payload: map[string]string{
					"path": "C:/Windows",
				},
			},
			config:   DefaultSafetyConfig(),
			expected: false,
		},
		{
			name: "Lowercase drive letter - READ",
			action: &pb.Action{
				Type: "READ",
				Payload: map[string]string{
					"path": "c:/windows",
				},
			},
			config:   DefaultSafetyConfig(),
			expected: false,
		},

		// Valid relative paths
		{
			name: "Deep relative path - READ",
			action: &pb.Action{
				Type: "READ",
				Payload: map[string]string{
					"path": "docs/subfolder/file.txt",
				},
			},
			config:   DefaultSafetyConfig(),
			expected: true,
		},
		{
			name: "Relative path with mixed slashes (Go handles this usually but safe checker allows) - READ",
			action: &pb.Action{
				Type: "READ",
				Payload: map[string]string{
					"path": "docs\\subfolder/file.txt",
				},
			},
			config:   DefaultSafetyConfig(),
			expected: true,
		},

		// Null byte injection (Go strings handle this but checking behavior)
		// The current checker doesn't explicitly block null bytes, but they shouldn't pass as valid file paths usually.
		// However, for pure path safety logic, it passes if no '..' or absolute markers are found.
		// We'll skip this for now as it's not strictly path traversal.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSafetyChecker(tt.config)
			ok, reason := s.ValidateAction(tt.action)
			if ok != tt.expected {
				t.Errorf("%s: expected %v, got %v (reason: %s)", tt.name, tt.expected, ok, reason)
			}
		})
	}
}
