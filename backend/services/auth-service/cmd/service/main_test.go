package main

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMustParseDuration(t *testing.T) {
	// Test default value
	d := mustParseDuration("NON_EXISTENT_KEY", "1h")
	assert.Equal(t, time.Hour, d)

	// Test environment override
	os.Setenv("MOCK_TTL", "2h")
	defer os.Unsetenv("MOCK_TTL")
	d = mustParseDuration("MOCK_TTL", "1h")
	assert.Equal(t, 2*time.Hour, d)
}

func TestGetenv(t *testing.T) {
	os.Setenv("TEST_KEY", "val")
	assert.Equal(t, "val", getenv("TEST_KEY", "default"))

	os.Unsetenv("TEST_KEY")
	assert.Equal(t, "default", getenv("TEST_KEY", "default"))
}
