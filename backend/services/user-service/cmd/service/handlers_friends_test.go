package main

import (
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

	t.Run("InvalidTarget_Me", func(t *testing.T) {
		req := newRequestWithUser("POST", "/users/me/friends/"+me+"/request", me)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", me)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()

		s.handleSendFriendRequest(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "invalid target user")
	})

	t.Run("InvalidUUID", func(t *testing.T) {
		badUUID := "not-a-uuid"
		req := newRequestWithUser("POST", "/users/me/friends/"+badUUID+"/request", me)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", badUUID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()

		s.handleSendFriendRequest(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "invalid target user id")
	})

	t.Run("TargetNotFound", func(t *testing.T) {
		req := newRequestWithUser("POST", "/users/me/friends/"+target+"/request", me)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", target)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()

		mock.ExpectQuery("SELECT EXISTS.*user_profiles").
			WithArgs(target).
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))

		s.handleSendFriendRequest(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "target user not found")
	})

	t.Run("ReciprocalCheckError", func(t *testing.T) {
		req := newRequestWithUser("POST", "/users/me/friends/"+target+"/request", me)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", target)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()

		mock.ExpectQuery("SELECT EXISTS.*user_profiles").WithArgs(target).WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
		mock.ExpectQuery("SELECT EXISTS.*user_friends.*").WithArgs(me, target).WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))

		// Reciprocal found but Scan error (other than NoRows)
		mock.ExpectQuery("SELECT id FROM friend_requests").
			WithArgs(target, me).
			WillReturnError(io.ErrUnexpectedEOF)

		s.handleSendFriendRequest(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("ReciprocalAcceptError", func(t *testing.T) {
		req := newRequestWithUser("POST", "/users/me/friends/"+target+"/request", me)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", target)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()

		mock.ExpectQuery("SELECT EXISTS.*user_profiles").WithArgs(target).WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
		mock.ExpectQuery("SELECT EXISTS.*user_friends.*").WithArgs(me, target).WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))

		mock.ExpectQuery("SELECT id FROM friend_requests").
			WithArgs(target, me).
			WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("req-id"))

		mock.ExpectExec("UPDATE friend_requests").
			WithArgs("req-id").
			WillReturnError(io.ErrUnexpectedEOF)

		s.handleSendFriendRequest(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("ReciprocalAddFriendsError", func(t *testing.T) {
		req := newRequestWithUser("POST", "/users/me/friends/"+target+"/request", me)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", target)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()

		mock.ExpectQuery("SELECT EXISTS.*user_profiles").WithArgs(target).WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
		mock.ExpectQuery("SELECT EXISTS.*user_friends.*").WithArgs(me, target).WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))

		mock.ExpectQuery("SELECT id FROM friend_requests").
			WithArgs(target, me).
			WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("req-id"))

		mock.ExpectExec("UPDATE friend_requests").
			WithArgs("req-id").
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		mock.ExpectExec("INSERT INTO user_friends").
			WithArgs(me, target).
			WillReturnError(io.ErrUnexpectedEOF)

		s.handleSendFriendRequest(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
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

func TestHandleRemoveFriend(t *testing.T) {
	s, mock := setupMockServer(t)
	defer mock.Close()

	me := "11111111-1111-1111-1111-111111111111"
	friend := "33333333-3333-3333-3333-333333333333"

	t.Run("Success", func(t *testing.T) {
		req := newRequestWithUser("DELETE", "/users/me/friends/"+friend, me)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", friend)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()

		// removeFriends executes delete on user_friends with sorted IDs
		// me < friend (1... < 3...)
		mock.ExpectExec("DELETE FROM user_friends").
			WithArgs(me, friend).
			WillReturnResult(pgxmock.NewResult("DELETE", 1))

		s.handleRemoveFriend(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "removed")
	})
}

func TestHandleListIncomingFriendRequests(t *testing.T) {
	s, mock := setupMockServer(t)
	defer mock.Close()

	me := "11111111-1111-1111-1111-111111111111"

	t.Run("Success", func(t *testing.T) {
		req := newRequestWithUser("GET", "/users/me/friends/requests/incoming", me)
		w := httptest.NewRecorder()

		rows := pgxmock.NewRows([]string{
			"user_id", "username", "display_name", "avatar_url", "has_custom_avatar", "visibility", "created_at",
		}).AddRow(
			"sender-id", "sender", "Sender Name", "url", false, "public", time.Now(),
		)

		mock.ExpectQuery("SELECT p.user_id, p.username.*FROM friend_requests").
			WithArgs(me).
			WillReturnRows(rows)

		s.handleListIncomingFriendRequests(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp IncomingFriendRequestsResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Len(t, resp.Items, 1)
		assert.Equal(t, "sender", resp.Items[0].From.Username)
	})
}

func TestHandleFriends_ErrorCases(t *testing.T) {
	s, mock := setupMockServer(t)
	defer mock.Close()

	me := "11111111-1111-1111-1111-111111111111"

	t.Run("SendRequest_DBError", func(t *testing.T) {
		target := "22222222-2222-2222-2222-222222222222"
		req := newRequestWithUser("POST", "/users/"+target+"/friends/request", me)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", target)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// 1. userExists -> true
		mock.ExpectQuery("SELECT EXISTS.*user_profiles").
			WithArgs(target).
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))

		// 2. areFriends -> false
		mock.ExpectQuery("SELECT EXISTS.*user_friends").
			WithArgs(me, target). // sorted
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))

		// 3. check pending -> nil (no pending)
		mock.ExpectQuery("SELECT id FROM friend_requests").
			WithArgs(target, me).
			WillReturnError(pgx.ErrNoRows)

		// 4. Insert -> Error
		mock.ExpectQuery("INSERT INTO friend_requests").
			WithArgs(me, target).
			WillReturnError(errors.New("db boom"))

		s.handleSendFriendRequest(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("AcceptRequest_DBError", func(t *testing.T) {
		from := "22222222-2222-2222-2222-222222222222"
		req := newRequestWithUser("POST", "/users/"+from+"/friends/accept", me)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", from)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// 1. Check pending -> Found
		mock.ExpectQuery("SELECT id FROM friend_requests").
			WithArgs(from, me).
			WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("req-id"))

		// 2. Update status -> Error
		mock.ExpectExec("UPDATE friend_requests").
			WithArgs("req-id").
			WillReturnError(errors.New("db boom"))

		s.handleAcceptFriendRequest(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("RemoveFriend_DBError", func(t *testing.T) {
		friend := "22222222-2222-2222-2222-222222222222"
		req := newRequestWithUser("DELETE", "/users/"+friend+"/friends", me)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", friend)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// 1. Delete -> Error
		mock.ExpectExec("DELETE FROM user_friends").
			WithArgs(me, friend).
			WillReturnError(errors.New("db boom"))

		s.handleRemoveFriend(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("RejectRequest_DBError", func(t *testing.T) {
		from := "22222222-2222-2222-2222-222222222222"
		req := newRequestWithUser("POST", "/users/"+from+"/friends/reject", me)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", from)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Logic:
		// res, err := s.db.Exec(...)
		// if err != nil { writeError... }
		// So checking error first.

		mock.ExpectExec("UPDATE friend_requests").
			WithArgs(from, me).
			WillReturnError(errors.New("db boom"))

		s.handleRejectFriendRequest(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("ListFriends_DBError", func(t *testing.T) {
		req := newRequestWithUser("GET", "/users/me/friends", me)
		w := httptest.NewRecorder()

		mock.ExpectQuery("SELECT p.user_id, p.username").
			WithArgs(me).
			WillReturnError(errors.New("db boom"))

		s.handleListFriends(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("ListIncoming_DBError", func(t *testing.T) {
		req := newRequestWithUser("GET", "/users/me/friends/requests/incoming", me)
		w := httptest.NewRecorder()

		mock.ExpectQuery("SELECT p.user_id, p.username.*FROM friend_requests").
			WithArgs(me).
			WillReturnError(errors.New("db boom"))

		s.handleListIncomingFriendRequests(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
