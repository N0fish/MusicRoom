package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
)

func TestHandleGetMe(t *testing.T) {
	s, mock := setupMockServer(t)
	defer mock.Close()

	me := "11111111-1111-1111-1111-111111111111"

	t.Run("Success", func(t *testing.T) {
		req := newRequestWithUser("GET", "/users/me", me)
		w := httptest.NewRecorder()

		// Expect getOrCreateProfile
		// It tries to find profile, if not found inserts.
		// Let's assume profile exists for simplicity first, or we match the logic.
		// getOrCreateProfile: selects from user_profiles.

		mock.ExpectQuery("SELECT.*FROM user_profiles WHERE user_id").
			WithArgs(me).
			WillReturnRows(pgxmock.NewRows([]string{
				"id", "user_id", "display_name", "username",
				"avatar_url", "has_custom_avatar", "bio",
				"visibility", "preferences", "is_premium", "created_at", "updated_at",
			}).AddRow(
				"profile-id", me, "Test User", "testuser",
				"url", false, "I am a test user",
				"public", []byte(`{}`), false, time.Now(), time.Now(),
			))

		s.handleGetMe(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp UserProfileResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, "testuser", resp.Username)
	})

	t.Run("InternalError", func(t *testing.T) {
		req := newRequestWithUser("GET", "/users/me", me)
		w := httptest.NewRecorder()

		mock.ExpectQuery("SELECT.*FROM user_profiles").
			WithArgs(me).
			WillReturnError(pgx.ErrTxClosed) // Simulate DB error

		s.handleGetMe(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestHandlePatchMe(t *testing.T) {
	s, mock := setupMockServer(t)
	defer mock.Close()

	me := "11111111-1111-1111-1111-111111111111"

	t.Run("Success", func(t *testing.T) {
		updateReq := UpdateUserProfileRequest{
			DisplayName: ptr("New Name"),
			Bio:         ptr("New Bio"),
			Visibility:  ptr("private"),
		}
		body, _ := json.Marshal(updateReq)
		req := newRequestWithUser("PATCH", "/users/me", me)
		req.Body = io.NopCloser(bytes.NewReader(body))
		w := httptest.NewRecorder()

		// 1. getOrCreateProfile (Get)
		mock.ExpectQuery("SELECT.*FROM user_profiles WHERE user_id").
			WithArgs(me).
			WillReturnRows(pgxmock.NewRows([]string{
				"id", "user_id", "display_name", "username",
				"avatar_url", "has_custom_avatar", "bio",
				"visibility", "preferences", "is_premium", "created_at", "updated_at",
			}).AddRow(
				"pid", me, "Old Name", "testuser",
				"url", false, "Old Bio",
				"public", []byte(`{}`), false, time.Now(), time.Now(),
			))

		// 2. saveProfile (Upsert/Update)
		// saveProfile performs UPDATE
		mock.ExpectExec("UPDATE user_profiles").
			WithArgs(
				"New Name", "testuser", "url", false, "New Bio", "private",
				pgxmock.AnyArg(), false, pgxmock.AnyArg(), me,
			).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		s.handlePatchMe(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp UserProfileResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "New Name", resp.DisplayName)
		assert.Equal(t, "New Bio", resp.Bio)
		assert.Equal(t, "private", resp.Visibility)
	})
}

func TestHandleGetPublicProfile(t *testing.T) {
	s, mock := setupMockServer(t)
	defer mock.Close()

	me := "11111111-1111-1111-1111-111111111111"
	target := "22222222-2222-2222-2222-222222222222"

	t.Run("PublicProfile_AsStranger", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users/"+target, nil)
		// Not logged in or logged in as someone else (me)
		ctx := context.WithValue(req.Context(), ctxUserIDKey{}, me)
		req = req.WithContext(ctx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", target)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()

		// 1. Get Target Profile
		mock.ExpectQuery("SELECT.*FROM user_profiles").
			WithArgs(target).
			WillReturnRows(pgxmock.NewRows([]string{
				"id", "user_id", "display_name", "username",
				"avatar_url", "has_custom_avatar", "bio",
				"visibility", "preferences", "is_premium", "created_at", "updated_at",
			}).AddRow(
				"tid", target, "Target", "target",
				"url", false, "Hidden Bio",
				"public", []byte(`{}`), false, time.Now(), time.Now(),
			))

		// 2. Check Friend Status (if not owner)
		// areFriends calls: SELECT EXISTS (...)
		mock.ExpectQuery("SELECT EXISTS.*user_friends").
			WithArgs(me, target). // sorted
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))

		s.handleGetPublicProfile(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp PublicUserProfileResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "Hidden Bio", resp.Bio) // Public visibility -> shows bio
	})

	t.Run("PrivateProfile_AsStranger", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users/"+target, nil)
		ctx := context.WithValue(req.Context(), ctxUserIDKey{}, me)
		req = req.WithContext(ctx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", target)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()

		mock.ExpectQuery("SELECT.*FROM user_profiles").
			WithArgs(target).
			WillReturnRows(pgxmock.NewRows([]string{
				"id", "user_id", "display_name", "username",
				"avatar_url", "has_custom_avatar", "bio",
				"visibility", "preferences", "is_premium", "created_at", "updated_at",
			}).AddRow(
				"tid", target, "Target", "target",
				"url", false, "Secret Bio",
				"private", []byte(`{}`), false, time.Now(), time.Now(),
			))

		mock.ExpectQuery("SELECT EXISTS.*user_friends").
			WithArgs(me, target).
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))

		s.handleGetPublicProfile(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp PublicUserProfileResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Empty(t, resp.Bio) // Private -> empty bio
	})

	t.Run("FriendsProfile_AsFriend", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users/"+target, nil)
		ctx := context.WithValue(req.Context(), ctxUserIDKey{}, me)
		req = req.WithContext(ctx)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", target)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()

		mock.ExpectQuery("SELECT.*FROM user_profiles").
			WithArgs(target).
			WillReturnRows(pgxmock.NewRows([]string{
				"id", "user_id", "display_name", "username",
				"avatar_url", "has_custom_avatar", "bio",
				"visibility", "preferences", "is_premium", "created_at", "updated_at",
			}).AddRow(
				"tid", target, "Target", "target",
				"url", false, "Friend Bio",
				"friends", []byte(`{}`), false, time.Now(), time.Now(),
			))

		mock.ExpectQuery("SELECT EXISTS.*user_friends").
			WithArgs(me, target).
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))

		s.handleGetPublicProfile(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp PublicUserProfileResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "Friend Bio", resp.Bio)
	})
}

// Helper for pointer
func ptr(s string) *string {
	return &s
}

func TestHandlePatchMe_ErrorCases(t *testing.T) {
	s, mock := setupMockServer(t)
	defer mock.Close()

	userID := "11111111-1111-1111-1111-111111111111"

	t.Run("InvalidJSON", func(t *testing.T) {
		req := newRequestWithUser("PATCH", "/users/me", userID)
		req.Body = io.NopCloser(bytes.NewBufferString("invalid json"))
		w := httptest.NewRecorder()
		s.handlePatchMe(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("InvalidVisibility", func(t *testing.T) {
		// Valid JSON structure but invalid value caught by Validate()
		body := `{"visibility": "invalid"}`
		req := newRequestWithUser("PATCH", "/users/me", userID)
		req.Body = io.NopCloser(bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		s.handlePatchMe(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("DBUpdateError", func(t *testing.T) {
		body := `{"displayName": "New Name"}`
		req := newRequestWithUser("PATCH", "/users/me", userID)
		req.Body = io.NopCloser(bytes.NewBufferString(body))
		w := httptest.NewRecorder()

		// 1. getOrCreateProfile success
		mock.ExpectQuery("SELECT id, user_id").
			WithArgs(userID).
			WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "display_name", "username", "avatar_url", "has_custom_avatar", "bio", "visibility", "preferences", "is_premium", "created_at", "updated_at"}).
				AddRow("1", userID, "Old", "old", "", false, "", "public", []byte("{}"), false, time.Now(), time.Now()))

		// 2. ensureUsername inside getOrCreateProfile shouldn't run logic if scan returned "old"
		// If returned profile has "old", ensureUsername returns "old". No DB calls.

		// 3. saveProfile -> error
		mock.ExpectExec("UPDATE user_profiles").
			WithArgs("New Name", "old", "", false, "", "public", pgxmock.AnyArg(), false, pgxmock.AnyArg(), userID).
			WillReturnError(errors.New("db boom"))

		s.handlePatchMe(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestHandleCheckUserExists(t *testing.T) {
	s, mock := setupMockServer(t)
	defer mock.Close()

	target := "22222222-2222-2222-2222-222222222222"

	t.Run("Exists", func(t *testing.T) {
		req := httptest.NewRequest("HEAD", "/users/"+target+"/exists", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", target)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()

		rows := pgxmock.NewRows([]string{
			"id", "user_id", "display_name", "username",
			"avatar_url", "has_custom_avatar", "bio",
			"visibility", "preferences", "is_premium", "created_at", "updated_at",
		}).AddRow(
			"pid", target, "Name", "user",
			"url", false, "bio",
			"public", []byte(`{}`), false, time.Now(), time.Now(),
		)

		mock.ExpectQuery("SELECT.*FROM user_profiles WHERE user_id").
			WithArgs(target).
			WillReturnRows(rows)

		s.handleCheckUserExists(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("NotFound", func(t *testing.T) {
		req := httptest.NewRequest("HEAD", "/users/"+target+"/exists", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", target)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()

		mock.ExpectQuery("SELECT.*FROM user_profiles WHERE user_id").
			WithArgs(target).
			WillReturnError(pgx.ErrNoRows)

		s.handleCheckUserExists(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// Needed imports: "io" added above? Yes.
func TestHandleBecomePremium(t *testing.T) {
	s, mock := setupMockServer(t)
	defer mock.Close()

	me := "11111111-1111-1111-1111-111111111111"

	t.Run("Success", func(t *testing.T) {
		req := newRequestWithUser("POST", "/users/me/premium", me)
		w := httptest.NewRecorder()

		// 1. getOrCreateProfile
		mock.ExpectQuery("SELECT.*FROM user_profiles WHERE user_id").
			WithArgs(me).
			WillReturnRows(pgxmock.NewRows([]string{
				"id", "user_id", "display_name", "username",
				"avatar_url", "has_custom_avatar", "bio",
				"visibility", "preferences", "is_premium", "created_at", "updated_at",
			}).AddRow(
				"pid", me, "Test User", "testuser",
				"url", false, "bio",
				"public", []byte(`{}`), false, time.Now(), time.Now(),
			))

		// 2. saveProfile (is_premium set to true)
		mock.ExpectExec("UPDATE user_profiles").
			WithArgs("Test User", "testuser", "url", false, "bio", "public", pgxmock.AnyArg(), true, pgxmock.AnyArg(), me).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		s.handleBecomePremium(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp UserProfileResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.True(t, resp.IsPremium)
	})
}
