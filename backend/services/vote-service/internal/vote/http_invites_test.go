package vote

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHandleCreateInvite(t *testing.T) {
	t.Run("missing user id", func(t *testing.T) {
		server := &HTTPServer{}
		r := chi.NewRouter()
		r.Post("/events/{id}/invites", server.handleCreateInvite)

		req := httptest.NewRequest("POST", "/events/ev1/invites", nil)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("invalid body", func(t *testing.T) {
		server := &HTTPServer{}
		r := chi.NewRouter()
		r.Post("/events/{id}/invites", server.handleCreateInvite)

		req := httptest.NewRequest("POST", "/events/ev1/invites", bytes.NewBufferString("{invalid}"))
		req.Header.Set("X-User-Id", "owner")
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("missing target userId", func(t *testing.T) {
		server := &HTTPServer{}
		r := chi.NewRouter()
		r.Post("/events/{id}/invites", server.handleCreateInvite)

		payload := map[string]string{"userId": ""}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/events/ev1/invites", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "owner")
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("event not found", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Post("/events/{id}/invites", server.handleCreateInvite)

		payload := map[string]string{"userId": "user1"}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/events/ev1/invites", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "owner")
		rec := httptest.NewRecorder()

		mockStore.On("LoadEvent", mock.Anything, "ev1").Return((*Event)(nil), pgx.ErrNoRows)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("forbidden not owner", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Post("/events/{id}/invites", server.handleCreateInvite)

		payload := map[string]string{"userId": "user2"}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/events/ev1/invites", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "stranger")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", OwnerID: "owner", Visibility: "private"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("self invite public event allowed", func(t *testing.T) {
		mockStore := new(MockStore)
		// Mock external check user
		userSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer userSrv.Close()

		server := &HTTPServer{store: mockStore, httpClient: http.DefaultClient, userServiceURL: userSrv.URL}
		r := chi.NewRouter()
		r.Post("/events/{id}/invites", server.handleCreateInvite)

		payload := map[string]string{"userId": "user1"}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/events/ev1/invites", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "user1")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", OwnerID: "owner", Visibility: "public"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)
		mockStore.On("CreateInvite", mock.Anything, "ev1", "user1", "contributor").Return(nil)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("success self invite as guest (public invited_only)", func(t *testing.T) {
		mockStore := new(MockStore)
		// User exists check mock
		userSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer userSrv.Close()

		server := &HTTPServer{store: mockStore, httpClient: http.DefaultClient, userServiceURL: userSrv.URL}
		r := chi.NewRouter()
		r.Post("/events/{id}/invites", server.handleCreateInvite)

		payload := map[string]string{"userId": "user1"}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/events/ev1/invites", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "user1")
		rec := httptest.NewRecorder()

		// Event is public visibility but invited_only license
		ev := &Event{ID: "ev1", OwnerID: "owner", Visibility: "public", LicenseMode: "invited_only"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)
		mockStore.On("CreateInvite", mock.Anything, "ev1", "user1", "guest").Return(nil)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("user does not exist", func(t *testing.T) {
		mockStore := new(MockStore)
		userSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer userSrv.Close()

		server := &HTTPServer{store: mockStore, httpClient: http.DefaultClient, userServiceURL: userSrv.URL}
		r := chi.NewRouter()
		r.Post("/events/{id}/invites", server.handleCreateInvite)

		payload := map[string]string{"userId": "ghost"}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/events/ev1/invites", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "owner")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", OwnerID: "owner", Visibility: "private"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("store error", func(t *testing.T) {
		mockStore := new(MockStore)
		userSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer userSrv.Close()

		server := &HTTPServer{store: mockStore, httpClient: http.DefaultClient, userServiceURL: userSrv.URL}
		r := chi.NewRouter()
		r.Post("/events/{id}/invites", server.handleCreateInvite)

		payload := map[string]string{"userId": "user1"}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/events/ev1/invites", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "owner")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", OwnerID: "owner", Visibility: "private"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)
		mockStore.On("CreateInvite", mock.Anything, "ev1", "user1", "contributor").Return(errors.New("db error"))

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestHandleDeleteInvite(t *testing.T) {
	t.Run("event not found", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Delete("/events/{id}/invites/{userId}", server.handleDeleteInvite)

		req := httptest.NewRequest("DELETE", "/events/ev1/invites/u1", nil)
		req.Header.Set("X-User-Id", "owner")
		rec := httptest.NewRecorder()

		mockStore.On("LoadEvent", mock.Anything, "ev1").Return((*Event)(nil), pgx.ErrNoRows)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("forbidden not owner or self", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Delete("/events/{id}/invites/{userId}", server.handleDeleteInvite)

		req := httptest.NewRequest("DELETE", "/events/ev1/invites/u1", nil)
		req.Header.Set("X-User-Id", "stranger")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", OwnerID: "owner"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("success delete self", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore, httpClient: http.DefaultClient}
		r := chi.NewRouter()
		r.Delete("/events/{id}/invites/{userId}", server.handleDeleteInvite)

		req := httptest.NewRequest("DELETE", "/events/ev1/invites/u1", nil)
		req.Header.Set("X-User-Id", "u1")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", OwnerID: "owner"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)
		mockStore.On("DeleteInvite", mock.Anything, "ev1", "u1").Return(nil)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("load event error", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Delete("/events/{id}/invites/{userId}", server.handleDeleteInvite)

		req := httptest.NewRequest("DELETE", "/events/ev1/invites/u1", nil)
		req.Header.Set("X-User-Id", "u1")
		rec := httptest.NewRecorder()

		mockStore.On("LoadEvent", mock.Anything, "ev1").Return((*Event)(nil), errors.New("db error"))

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("delete invite store error", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Delete("/events/{id}/invites/{userId}", server.handleDeleteInvite)

		req := httptest.NewRequest("DELETE", "/events/ev1/invites/u1", nil)
		req.Header.Set("X-User-Id", "owner")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", OwnerID: "owner"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)
		mockStore.On("DeleteInvite", mock.Anything, "ev1", "u1").Return(errors.New("db error"))

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestHandleListInvites(t *testing.T) {
	t.Run("forbidden not invited", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Get("/events/{id}/invites", server.handleListInvites)

		req := httptest.NewRequest("GET", "/events/ev1/invites", nil)
		req.Header.Set("X-User-Id", "stranger")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", OwnerID: "owner"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)
		mockStore.On("IsInvited", mock.Anything, "ev1", "stranger").Return(false, nil)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("success owner lists", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Get("/events/{id}/invites", server.handleListInvites)

		req := httptest.NewRequest("GET", "/events/ev1/invites", nil)
		req.Header.Set("X-User-Id", "owner")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", OwnerID: "owner"}
		invites := []Invite{{UserID: "u1"}}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)
		mockStore.On("ListInvites", mock.Anything, "ev1").Return(invites, nil)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var resp []*Invite
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Len(t, resp, 1)
	})

	t.Run("is invited error", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Get("/events/{id}/invites", server.handleListInvites)

		req := httptest.NewRequest("GET", "/events/ev1/invites", nil)
		req.Header.Set("X-User-Id", "stranger")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", OwnerID: "owner"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)
		mockStore.On("IsInvited", mock.Anything, "ev1", "stranger").Return(false, errors.New("db error"))

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("list invites error", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Get("/events/{id}/invites", server.handleListInvites)

		req := httptest.NewRequest("GET", "/events/ev1/invites", nil)
		req.Header.Set("X-User-Id", "owner")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", OwnerID: "owner"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)
		mockStore.On("ListInvites", mock.Anything, "ev1").Return(([]Invite)(nil), errors.New("db error"))

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}
