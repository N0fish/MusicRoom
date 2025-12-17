package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to create a request with signed-in user context
func newRequestWithUser(method, url, userID string) *http.Request {
	req := httptest.NewRequest(method, url, nil)
	ctx := context.WithValue(req.Context(), ctxUserIDKey{}, userID)
	return req.WithContext(ctx)
}

// Helper to setup mock DB and Server
func setupMockServer(t *testing.T) (*Server, pgxmock.PgxPoolIface) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	return &Server{db: mock}, mock
}

func TestHandleSendFriendRequest(t *testing.T) {
	s, mock := setupMockServer(t)
	defer mock.Close()

	me := "11111111-1111-1111-1111-111111111111"
	target := "22222222-2222-2222-2222-222222222222"

	t.Run("Success", func(t *testing.T) {
		req := newRequestWithUser("POST", "/users/me/friends/"+target+"/request", me)

		// Chi context for URL param
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", target)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// 1. Check if target user exists
		mock.ExpectQuery("SELECT EXISTS.*user_profiles WHERE user_id").
			WithArgs(target).
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))

		// 2. Check if already friends
		mock.ExpectQuery("SELECT EXISTS.*user_friends.*").
			WithArgs(me, target). // sorted order handled in query logic? no, handler sorts. me < target
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))

		// 3. Check existing pending request (reciprocal check)
		// handler checks for: "SELECT id FROM friend_requests WHERE from_user_id = target AND to_user_id = me AND status = 'pending'"
		mock.ExpectQuery("SELECT id FROM friend_requests").
			WithArgs(target, me).
			WillReturnRows(pgxmock.NewRows([]string{"id"})) // Returns empty result (Scan error handled as no row)

		// 4. Insert new request
		// "INSERT INTO friend_requests ... RETURNING id ..."
		mock.ExpectQuery("INSERT INTO friend_requests").
			WithArgs(me, target).
			WillReturnRows(pgxmock.NewRows([]string{"id", "from_user_id", "to_user_id", "status", "created_at", "updated_at"}).
				AddRow("req-uuid", me, target, "pending", time.Now(), time.Now()))

		s.handleSendFriendRequest(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp FriendRequestResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "pending", resp.Status)
	})

	t.Run("AlreadyFriends", func(t *testing.T) {
		req := newRequestWithUser("POST", "/users/me/friends/"+target+"/request", me)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", target)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()

		mock.ExpectQuery("SELECT EXISTS.*user_profiles").
			WithArgs(target).
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))

		mock.ExpectQuery("SELECT EXISTS.*user_friends.*").
			WithArgs(me, target). // sorted
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))

		s.handleSendFriendRequest(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "already friends")
	})

	t.Run("ReciprocalRequest", func(t *testing.T) {
		req := newRequestWithUser("POST", "/users/me/friends/"+target+"/request", me)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", target)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()

		mock.ExpectQuery("SELECT EXISTS.*user_profiles").WithArgs(target).WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
		mock.ExpectQuery("SELECT EXISTS.*user_friends.*").WithArgs(me, target).WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))

		// Reciprocal found
		existingReqID := "reciprocal-req-id"
		mock.ExpectQuery("SELECT id FROM friend_requests").
			WithArgs(target, me).
			WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(existingReqID))

		// Update request status
		mock.ExpectExec("UPDATE friend_requests").
			WithArgs(existingReqID).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		// Add friends (insert into user_friends)
		mock.ExpectExec("INSERT INTO user_friends").
			WithArgs(me, target). // sorted
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		s.handleSendFriendRequest(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "accepted")
	})
}

func TestHandleAcceptFriendRequest(t *testing.T) {
	s, mock := setupMockServer(t)
	defer mock.Close()

	me := "11111111-1111-1111-1111-111111111111"
	from := "33333333-3333-3333-3333-333333333333"

	t.Run("Success", func(t *testing.T) {
		req := newRequestWithUser("POST", "/users/me/friends/"+from+"/accept", me)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", from)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()

		reqID := "req-id-123"
		mock.ExpectQuery("SELECT id FROM friend_requests").
			WithArgs(from, me).
			WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(reqID))

		mock.ExpectExec("UPDATE friend_requests").
			WithArgs(reqID).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		// Sorted IDs for addFriends: 11.. < 33.. -> me, from
		mock.ExpectExec("INSERT INTO user_friends").
			WithArgs(me, from).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		s.handleAcceptFriendRequest(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "accepted")
	})

	t.Run("NoPendingRequest", func(t *testing.T) {
		req := newRequestWithUser("POST", "/users/me/friends/"+from+"/accept", me)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", from)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()

		// Return no rows (Scan error)
		mock.ExpectQuery("SELECT id FROM friend_requests").
			WithArgs(from, me).
			WillReturnError(pgx.ErrNoRows)

		s.handleAcceptFriendRequest(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandleRejectFriendRequest(t *testing.T) {
	s, mock := setupMockServer(t)
	defer mock.Close()

	me := "11111111-1111-1111-1111-111111111111"
	from := "33333333-3333-3333-3333-333333333333"

	t.Run("Success", func(t *testing.T) {
		req := newRequestWithUser("POST", "/users/me/friends/"+from+"/reject", me)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", from)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()

		mock.ExpectExec("UPDATE friend_requests").
			WithArgs(from, me).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		s.handleRejectFriendRequest(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "rejected")
	})
}

func TestHandleListFriends(t *testing.T) {
	s, mock := setupMockServer(t)
	defer mock.Close()

	me := "11111111-1111-1111-1111-111111111111"

	t.Run("Success", func(t *testing.T) {
		req := newRequestWithUser("GET", "/users/me/friends", me)
		w := httptest.NewRecorder()

		rows := pgxmock.NewRows([]string{
			"user_id", "username", "display_name", "avatar_url", "has_custom_avatar", "visibility",
		}).
			AddRow("2222-2222", "buddy", "Buddy", "url", false, "public").
			AddRow("3333-3333", "chum", "Chum", "url", false, "public")

		mock.ExpectQuery("SELECT p.user_id, p.username").
			WithArgs(me).
			WillReturnRows(rows)

		s.handleListFriends(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "buddy")
		assert.Contains(t, w.Body.String(), "chum")
	})
}
