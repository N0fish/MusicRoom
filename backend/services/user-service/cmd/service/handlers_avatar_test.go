package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleUploadAvatar(t *testing.T) {
	// Setup temp dir for avatars
	tempDir, err := os.MkdirTemp("", "avatar_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Override env var for the test
	os.Setenv("AVATAR_DIR", tempDir)
	defer os.Unsetenv("AVATAR_DIR")

	s, mock := setupMockServer(t)
	defer mock.Close()

	me := "11111111-1111-1111-1111-111111111111"

	t.Run("Success", func(t *testing.T) {
		// Create multipart body
		body := new(bytes.Buffer)
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("file", "avatar.png")
		require.NoError(t, err)
		io.WriteString(part, "fake png content")
		writer.Close()

		req := newRequestWithUser("POST", "/users/me/avatar/upload", me)
		req.Body = io.NopCloser(body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		// 1. getOrCreateProfile
		mock.ExpectQuery("SELECT.*FROM user_profiles").
			WithArgs(me).
			WillReturnRows(pgxmock.NewRows([]string{
				"id", "user_id", "display_name", "username",
				"avatar_url", "has_custom_avatar", "bio",
				"visibility", "preferences", "created_at", "updated_at",
			}).AddRow(
				"pid", me, "Test User", "testuser",
				"old_url", false, "Bio",
				"public", []byte(`{}`), time.Now(), time.Now(),
			))

		// 2. saveProfile (Update with new avatar URL)
		// It updates avatar_url and has_custom_avatar
		mock.ExpectExec("UPDATE user_profiles").
			WithArgs(
				"Test User", "testuser", pgxmock.AnyArg(), true, "Bio", "public",
				pgxmock.AnyArg(), pgxmock.AnyArg(), me,
			).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		s.handleUploadAvatar(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify file creation
		expectedPath := filepath.Join(tempDir, "custom", me+".png")
		_, err = os.Stat(expectedPath)
		assert.NoError(t, err, "Avatar file should exist")
	})

	t.Run("InvalidFileType", func(t *testing.T) {
		body := new(bytes.Buffer)
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("file", "malicious.exe")
		require.NoError(t, err)
		io.WriteString(part, "dangerous content")
		writer.Close()

		req := newRequestWithUser("POST", "/users/me/avatar/upload", me)
		req.Body = io.NopCloser(body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		// No DB calls expected
		s.handleUploadAvatar(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "unsupported file type")
	})

	t.Run("FileTooLarge", func(t *testing.T) {
		body := new(bytes.Buffer)
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("file", "large.png")
		require.NoError(t, err)

		// Write just over 5MB
		// 5 * 1024 * 1024 = 5242880 bytes
		// We'll write 5.1MB
		largeContent := make([]byte, 5*1024*1024+100)
		_, err = part.Write(largeContent)
		require.NoError(t, err)

		writer.Close()

		req := newRequestWithUser("POST", "/users/me/avatar/upload", me)
		req.Body = io.NopCloser(body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		s.handleUploadAvatar(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		// The error message from http.MaxBytesReader / ParseMultipartForm might vary slightly but "file too large" or "request body too large" is expected
		// Our handler says: "file too large or invalid form"
		assert.Contains(t, w.Body.String(), "file too large")
	})

	t.Run("MissingFile", func(t *testing.T) {
		body := new(bytes.Buffer)
		writer := multipart.NewWriter(body)
		// Use wrong field name
		part, err := writer.CreateFormFile("wrong_field", "avatar.png")
		require.NoError(t, err)
		io.WriteString(part, "content")
		writer.Close()

		req := newRequestWithUser("POST", "/users/me/avatar/upload", me)
		req.Body = io.NopCloser(body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		s.handleUploadAvatar(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "file is required")
	})

	t.Run("Unauthorized", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/users/me/avatar/upload", nil)
		w := httptest.NewRecorder()

		s.handleUploadAvatar(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("DBError_GetOrCreateProfile", func(t *testing.T) {
		body := new(bytes.Buffer)
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("file", "avatar.png")
		require.NoError(t, err)
		io.WriteString(part, "fake content")
		writer.Close()

		req := newRequestWithUser("POST", "/users/me/avatar/upload", me)
		req.Body = io.NopCloser(body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		mock.ExpectQuery("SELECT.*FROM user_profiles").
			WithArgs(me).
			WillReturnError(io.ErrUnexpectedEOF)

		s.handleUploadAvatar(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "internal error")
	})

	t.Run("DBError_SaveProfile", func(t *testing.T) {
		body := new(bytes.Buffer)
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("file", "avatar.png")
		require.NoError(t, err)
		io.WriteString(part, "fake content")
		writer.Close()

		req := newRequestWithUser("POST", "/users/me/avatar/upload", me)
		req.Body = io.NopCloser(body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		mock.ExpectQuery("SELECT.*FROM user_profiles").
			WithArgs(me).
			WillReturnRows(pgxmock.NewRows([]string{
				"id", "user_id", "display_name", "username",
				"avatar_url", "has_custom_avatar", "bio",
				"visibility", "preferences", "created_at", "updated_at",
			}).AddRow(
				"pid", me, "Test User", "testuser",
				"old_url", false, "Bio",
				"public", []byte(`{}`), time.Now(), time.Now(),
			))

		mock.ExpectExec("UPDATE user_profiles").
			WithArgs(
				"Test User", "testuser", pgxmock.AnyArg(), true, "Bio", "public",
				pgxmock.AnyArg(), pgxmock.AnyArg(), me,
			).
			WillReturnError(io.ErrUnexpectedEOF)

		s.handleUploadAvatar(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "cannot save avatar")
	})
}

func TestHandleGenerateRandomAvatar(t *testing.T) {
	// Setup temp dir for avatars
	tempDir, err := os.MkdirTemp("", "avatar_random_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create dummy avatar files
	// listAvatarFiles scans AVATAR_DIR, recursive logic might differ but let's put in root
	err = os.WriteFile(filepath.Join(tempDir, "1.png"), []byte("png"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "2.jpg"), []byte("jpg"), 0644)
	require.NoError(t, err)

	os.Setenv("AVATAR_DIR", tempDir)
	defer os.Unsetenv("AVATAR_DIR")

	s, mock := setupMockServer(t)
	defer mock.Close()

	me := "11111111-1111-1111-1111-111111111111"

	t.Run("Success", func(t *testing.T) {
		req := newRequestWithUser("POST", "/users/me/avatar/random", me)
		w := httptest.NewRecorder()

		// 1. getOrCreateProfile
		mock.ExpectQuery("SELECT.*FROM user_profiles").
			WithArgs(me).
			WillReturnRows(pgxmock.NewRows([]string{
				"id", "user_id", "display_name", "username",
				"avatar_url", "has_custom_avatar", "bio",
				"visibility", "preferences", "created_at", "updated_at",
			}).AddRow(
				"pid", me, "Test User", "testuser",
				"old_url", false, "Bio",
				"public", []byte(`{}`), time.Now(), time.Now(),
			))

		// 2. saveProfile (Update with new random URL)
		// It updates avatar_url (random value) and has_custom_avatar=true
		mock.ExpectExec("UPDATE user_profiles").
			WithArgs(
				"Test User", "testuser", pgxmock.AnyArg(), true, "Bio", "public",
				pgxmock.AnyArg(), pgxmock.AnyArg(), me,
			).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		s.handleGenerateRandomAvatar(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp UserProfileResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.True(t, resp.HasCustomAvatar)
		// URL should start with /avatars/ and end with png or jpg
		assert.Contains(t, resp.AvatarURL, "/avatars/")
	})

	t.Run("Unauthorized", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/users/me/avatar/random", nil)
		w := httptest.NewRecorder()

		s.handleGenerateRandomAvatar(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("DBError_GetOrCreateProfile", func(t *testing.T) {
		req := newRequestWithUser("POST", "/users/me/avatar/random", me)
		w := httptest.NewRecorder()

		mock.ExpectQuery("SELECT.*FROM user_profiles").
			WithArgs(me).
			WillReturnError(io.ErrUnexpectedEOF)

		s.handleGenerateRandomAvatar(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "internal error")
	})

	t.Run("DBError_SaveProfile", func(t *testing.T) {
		req := newRequestWithUser("POST", "/users/me/avatar/random", me)
		w := httptest.NewRecorder()

		mock.ExpectQuery("SELECT.*FROM user_profiles").
			WithArgs(me).
			WillReturnRows(pgxmock.NewRows([]string{
				"id", "user_id", "display_name", "username",
				"avatar_url", "has_custom_avatar", "bio",
				"visibility", "preferences", "created_at", "updated_at",
			}).AddRow(
				"pid", me, "Test User", "testuser",
				"old_url", false, "Bio",
				"public", []byte(`{}`), time.Now(), time.Now(),
			))

		mock.ExpectExec("UPDATE user_profiles").
			WithArgs(
				"Test User", "testuser", pgxmock.AnyArg(), true, "Bio", "public",
				pgxmock.AnyArg(), pgxmock.AnyArg(), me,
			).
			WillReturnError(io.ErrUnexpectedEOF)

		s.handleGenerateRandomAvatar(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "internal error")
	})
}
