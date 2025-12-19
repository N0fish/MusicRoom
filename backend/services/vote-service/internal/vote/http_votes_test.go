package vote

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHandleVote(t *testing.T) {
	// Updated success case: User must be invited/joined even for "everyone" license.
	t.Run("success", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Post("/events/{id}/vote", server.handleVote)

		payload := voteRequest{TrackID: "tr1"}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/events/ev1/vote", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "user1")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", OwnerID: "owner", LicenseMode: "everyone"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)
		// Must return true for IsInvited
		mockStore.On("IsInvited", mock.Anything, "ev1", "user1").Return(true, nil)

		mockStore.On("CastVote", mock.Anything, "ev1", "tr1", "user1").Return(nil)
		mockStore.On("GetVoteCount", mock.Anything, "ev1", "tr1").Return(10, nil)

		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp VoteResponse
		json.Unmarshal(rec.Body.Bytes(), &resp)
		assert.Equal(t, 10, resp.TotalVotes)
	})

	t.Run("forbidden not joined everyone license", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Post("/events/{id}/vote", server.handleVote)

		payload := voteRequest{TrackID: "tr1"}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/events/ev1/vote", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "user1")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", OwnerID: "owner", LicenseMode: "everyone"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)
		// Return false for IsInvited
		mockStore.On("IsInvited", mock.Anything, "ev1", "user1").Return(false, nil)

		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("missing user id", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Post("/events/{id}/vote", server.handleVote)

		req := httptest.NewRequest("POST", "/events/ev1/vote", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("invalid json", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Post("/events/{id}/vote", server.handleVote)

		req := httptest.NewRequest("POST", "/events/ev1/vote", bytes.NewBufferString("{bad json"))
		req.Header.Set("X-User-Id", "user1")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("missing track id", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Post("/events/{id}/vote", server.handleVote)

		payload := voteRequest{TrackID: ""}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/events/ev1/vote", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "user1")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("register failure", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Post("/events/{id}/vote", server.handleVote)

		payload := voteRequest{TrackID: "tr1"}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/events/ev1/vote", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "user1")
		rec := httptest.NewRecorder()

		// Event not found error from store
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return((*Event)(nil), pgx.ErrNoRows)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("vote forbidden", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Post("/events/{id}/vote", server.handleVote)

		payload := voteRequest{TrackID: "tr1"}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/events/ev1/vote", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "user1")
		rec := httptest.NewRecorder()

		// Event loaded but closed
		ev := &Event{ID: "ev1", OwnerID: "owner", LicenseMode: "geo-time", VoteEnd: ptrTime(time.Now().Add(-1 * time.Hour))}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)
		// No need to mock IsInvited or CastVote as it should fail before

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}
func ptrTime(t time.Time) *time.Time {
	return &t
}

func TestHandleRemoveVote(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Delete("/events/{id}/vote", server.handleRemoveVote)

		payload := voteRequest{TrackID: "tr1"}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("DELETE", "/events/ev1/vote", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "user1")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", OwnerID: "owner", LicenseMode: "everyone"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)
		mockStore.On("RemoveVote", mock.Anything, "ev1", "tr1", "user1").Return(nil)
		mockStore.On("GetVoteCount", mock.Anything, "ev1", "tr1").Return(9, nil)

		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp VoteResponse
		json.Unmarshal(rec.Body.Bytes(), &resp)
		assert.Equal(t, 9, resp.TotalVotes)
	})

	t.Run("unauthorized", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Delete("/events/{id}/vote", server.handleRemoveVote)

		req := httptest.NewRequest("DELETE", "/events/ev1/vote", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("invalid vote window", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Delete("/events/{id}/vote", server.handleRemoveVote)

		payload := voteRequest{TrackID: "tr1"}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("DELETE", "/events/ev1/vote", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "user1")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", OwnerID: "owner", LicenseMode: "geo-time", VoteEnd: ptrTime(time.Now().Add(-1 * time.Hour))}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

func TestHandleTally(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Get("/events/{id}/tally", server.handleTally)

		req := httptest.NewRequest("GET", "/events/ev1/tally", nil)
		req.Header.Set("X-User-Id", "user1")
		rec := httptest.NewRecorder()

		rows := []Row{
			{Track: "tr1", Count: 5, IsMyVote: true},
			{Track: "tr2", Count: 2, IsMyVote: false},
		}
		mockStore.On("GetVoteTally", mock.Anything, "ev1", "user1").Return(rows, nil)

		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var out []Row
		json.Unmarshal(rec.Body.Bytes(), &out)
		assert.Len(t, out, 2)
	})

	t.Run("db error", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Get("/events/{id}/tally", server.handleTally)

		req := httptest.NewRequest("GET", "/events/ev1/tally", nil)
		req.Header.Set("X-User-Id", "user1")
		rec := httptest.NewRecorder()

		mockStore.On("GetVoteTally", mock.Anything, "ev1", "user1").Return([]Row(nil), errors.New("db fail"))

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}
