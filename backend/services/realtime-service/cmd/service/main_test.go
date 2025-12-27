package main

import (
	"os"
	"testing"
)

func TestGetenv(t *testing.T) {
	// Test case 1: Variable not set (default value)
	key := "TEST_ENV_VAR_REALTIME"
	def := "default_value"
	val := getenv(key, def)
	if val != def {
		t.Errorf("expected %q, got %q", def, val)
	}

	// Test case 2: Variable set
	expected := "set_value"
	os.Setenv(key, expected)
	defer os.Unsetenv(key)

	val = getenv(key, def)
	if val != expected {
		t.Errorf("expected %q, got %q", expected, val)
	}
}
