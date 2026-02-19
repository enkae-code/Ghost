// Author: Enkae (enkae.dev@pm.me)
package conscience

import (
	"encoding/json"
	"testing"
	"ghost/kernel/internal/protocol"
)

func TestValidateFileSystemPath(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		path     string
		expected bool
	}{
		{"safe/path.txt", true},
		{"safe/subdir/file.txt", true},
		{"/absolute/path", false},
		// {"C:\\Windows\\System32", false}, // Go might interpret this as relative on Linux/Mac if not handled carefully, but validateFileSystemPath checks for absolute paths. On non-Windows, C:\ is just a valid filename with backslash.
		// However, my implementation checks specifically for `len(path) > 1 && path[1] == ':'` so it should be false everywhere.
		{"C:\\Windows\\System32", false},
		{"../parent", false},
		{"safe/../../unsafe", false},
		{"./safe.txt", true},
	}

	for _, test := range tests {
		if result := v.validateFileSystemPath(test.path); result != test.expected {
			t.Errorf("validateFileSystemPath(%q) = %v; want %v", test.path, result, test.expected)
		}
	}
}

func TestValidateActionPath(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name        string
		actionType  string
		payload     map[string]interface{}
		expectError bool
	}{
		{
			name:       "Write Safe Path",
			actionType: "WRITE",
			payload:    map[string]interface{}{"path": "safe.txt"},
			expectError: false,
		},
		{
			name:       "Write Unsafe Path",
			actionType: "WRITE",
			payload:    map[string]interface{}{"path": "/unsafe.txt"},
			expectError: true,
		},
		{
			name:       "List Directory",
			actionType: "LIST",
			payload:    map[string]interface{}{"directory": "safe_dir"},
			expectError: false,
		},
		{
			name:       "List Path Fallback",
			actionType: "LIST",
			payload:    map[string]interface{}{"path": "safe_dir"},
			expectError: false,
		},
		{
			name:       "Search Directory",
			actionType: "SEARCH",
			payload:    map[string]interface{}{"directory": "safe_dir"},
			expectError: false,
		},
		{
			name:       "Search Missing Directory",
			actionType: "SEARCH",
			payload:    map[string]interface{}{"path": "safe_dir"}, // SEARCH falls back to path
			expectError: false,
		},
	}

	for _, test := range tests {
		payloadBytes, _ := json.Marshal(test.payload)
		action := &protocol.LegacyAction{
			Type:    test.actionType,
			Payload: payloadBytes,
		}

		err := v.validateActionPath(action)
		if (err != nil) != test.expectError {
			t.Errorf("%s: validateActionPath() error = %v, expectError %v", test.name, err, test.expectError)
		}
	}
}
