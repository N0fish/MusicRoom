package utils

import (
	"testing"
)

func TestIsValidUUID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "Valid UUID",
			input:    "550e8400-e29b-41d4-a716-446655440000",
			expected: true,
		},
		{
			name:     "Valid UUID uppercase",
			input:    "550E8400-E29B-41D4-A716-446655440000",
			expected: true,
		},
		{
			name:     "Invalid UUID - empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "Invalid UUID - garbage",
			input:    "not-a-uuid",
			expected: false,
		},
		{
			name:     "Invalid UUID - wrong length",
			input:    "550e8400-e29b-41d4-a716-44665544000",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidUUID(tt.input)
			if result != tt.expected {
				t.Errorf("IsValidUUID(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
