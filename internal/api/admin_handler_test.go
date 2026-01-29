package api

import (
	"testing"
)

// Pure logic tests - these don't require database

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "***"},
		{"1234", "***"},
		{"12345678", "***"},
		{"123456789", "1234***6789"},
		{"sk-ant-api03-xxxxxxxxxxxxxx-xxxxxxxxxxxxx", "sk-a***xxxx"},
	}

	for _, tt := range tests {
		result := maskAPIKey(tt.input)
		if result != tt.expected {
			t.Errorf("maskAPIKey(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestMaskToken(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "***"},
		{"1234", "***"},
		{"12345678", "***"},
		{"123456789", "1234***6789"},
		{"test-token-12345", "test***2345"},
	}

	for _, tt := range tests {
		result := maskToken(tt.input)
		if result != tt.expected {
			t.Errorf("maskToken(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
