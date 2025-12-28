package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUniqueStrings(t *testing.T) {
	t.Run("NoDuplicates", func(t *testing.T) {
		in := []string{"a", "b", "c"}
		out := uniqueStrings(in)
		assert.Equal(t, in, out)
	})

	t.Run("Duplicates", func(t *testing.T) {
		in := []string{"a", "b", "a", "A", " b "}
		out := uniqueStrings(in)
		// Should be "a", "b" (case insensitive, trimmed)
		assert.Len(t, out, 2)
		assert.Equal(t, "a", out[0])
		assert.Equal(t, "b", out[1])
	})

	t.Run("EmptyStrings", func(t *testing.T) {
		in := []string{"a", "", "  ", "b"}
		out := uniqueStrings(in)
		assert.Equal(t, []string{"a", "b"}, out)
	})
}

func TestValidateUpdateProfile(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		name := "valid name"
		req := UpdateUserProfileRequest{
			DisplayName: &name,
		}
		assert.NoError(t, req.Validate())
	})

	t.Run("InvalidVisibility", func(t *testing.T) {
		vis := "invalid"
		req := UpdateUserProfileRequest{
			Visibility: &vis,
		}
		assert.Error(t, req.Validate())
	})

	t.Run("AvatarURLSet", func(t *testing.T) {
		url := "http://example.com"
		req := UpdateUserProfileRequest{
			AvatarURL: &url,
		}
		assert.Error(t, req.Validate())
	})

	t.Run("TooLong", func(t *testing.T) {
		name := strings.Repeat("a", 101)
		req := UpdateUserProfileRequest{
			DisplayName: &name,
		}
		assert.NoError(t, req.Validate()) // It trims, doesn't error
		assert.Equal(t, 100, len(*req.DisplayName))
	})
}
