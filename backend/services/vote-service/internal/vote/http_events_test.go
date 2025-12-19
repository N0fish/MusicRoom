package vote

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// RoundTripFunc .
type RoundTripFunc func(req *http.Request) *http.Response

// RoundTrip .
func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

// NewTestClient returns *http.Client with Transport replaced to avoid network calls
func NewTestClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: RoundTripFunc(fn),
	}
}

func TestHandleGetEvent(t *testing.T) {
	t.Run("public event success", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Get("/events/{id}", server.handleGetEvent)

		req := httptest.NewRequest("GET", "/events/ev1", nil)
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", Visibility: "public", OwnerID: "owner"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("private event forbidden", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Get("/events/{id}", server.handleGetEvent)

		req := httptest.NewRequest("GET", "/events/ev1", nil)
		req.Header.Set("X-User-Id", "stranger")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", Visibility: "private", OwnerID: "owner"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)
		mockStore.On("IsInvited", mock.Anything, "ev1", "stranger").Return(false, nil)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("private event success invited", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Get("/events/{id}", server.handleGetEvent)

		req := httptest.NewRequest("GET", "/events/ev1", nil)
		req.Header.Set("X-User-Id", "invitedUser")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", Visibility: "private", OwnerID: "owner"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)
		mockStore.On("IsInvited", mock.Anything, "ev1", "invitedUser").Return(true, nil)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("not found", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Get("/events/{id}", server.handleGetEvent)

		req := httptest.NewRequest("GET", "/events/ev1", nil)
		rec := httptest.NewRecorder()

		mockStore.On("LoadEvent", mock.Anything, "ev1").Return((*Event)(nil), pgx.ErrNoRows)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestHandleCreateEvent(t *testing.T) {
	t.Run("validation error", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Post("/events", server.handleCreateEvent)

		payload := map[string]any{"name": ""}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/events", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "user1")
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("playlist service fail", func(t *testing.T) {
		mockStore := new(MockStore)
		client := NewTestClient(func(req *http.Request) *http.Response {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(bytes.NewBufferString(`{"error":"fail"}`)),
				Header:     make(http.Header),
			}
		})

		server := &HTTPServer{store: mockStore, httpClient: client, playlistServiceURL: "http://pl"}
		r := chi.NewRouter()
		r.Post("/events", server.handleCreateEvent)

		payload := map[string]any{"name": "New Event"}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/events", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "user1")
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadGateway, rec.Code)
	})

	t.Run("success", func(t *testing.T) {
		mockStore := new(MockStore)
		client := NewTestClient(func(req *http.Request) *http.Response {
			// Mock playlist creation success
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(bytes.NewBufferString(`{"id":"pl1"}`)),
				Header:     make(http.Header),
			}
		})

		server := &HTTPServer{store: mockStore, httpClient: client, playlistServiceURL: "http://pl"}
		r := chi.NewRouter()
		r.Post("/events", server.handleCreateEvent)

		payload := map[string]any{
			"name":         "My Event",
			"visibility":   "public",
			"license_mode": "everyone",
		}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/events", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "user1")
		rec := httptest.NewRecorder()

		mockStore.On("CreateEvent", mock.Anything, mock.MatchedBy(func(ev *Event) bool {
			return ev.Name == "My Event" && ev.OwnerID == "user1" && ev.ID == "pl1"
		})).Return("pl1", nil)

		// Handler calls LoadEvent after creation
		ev := &Event{ID: "pl1", Name: "My Event", OwnerID: "user1"}
		mockStore.On("LoadEvent", mock.Anything, "pl1").Return(ev, nil)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusCreated, rec.Code)
	})

	t.Run("missing user id", func(t *testing.T) {
		server := &HTTPServer{}
		r := chi.NewRouter()
		r.Post("/events", server.handleCreateEvent)

		req := httptest.NewRequest("POST", "/events", nil)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("invalid json", func(t *testing.T) {
		server := &HTTPServer{}
		r := chi.NewRouter()
		r.Post("/events", server.handleCreateEvent)

		req := httptest.NewRequest("POST", "/events", bytes.NewBufferString("{invalid}"))
		req.Header.Set("X-User-Id", "user1")
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid vote window", func(t *testing.T) {
		server := &HTTPServer{}
		r := chi.NewRouter()
		r.Post("/events", server.handleCreateEvent)

		past := "2020-01-01T00:00:00Z"
		payload := map[string]any{"name": "Event", "vote_start": past}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/events", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "user1")
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("store create error", func(t *testing.T) {
		mockStore := new(MockStore)
		client := NewTestClient(func(req *http.Request) *http.Response {
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(bytes.NewBufferString(`{"id":"pl1"}`)),
				Header:     make(http.Header),
			}
		})

		server := &HTTPServer{store: mockStore, httpClient: client, playlistServiceURL: "http://pl"}
		r := chi.NewRouter()
		r.Post("/events", server.handleCreateEvent)

		payload := map[string]any{"name": "My Event"}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/events", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "user1")
		rec := httptest.NewRecorder()

		mockStore.On("CreateEvent", mock.Anything, mock.Anything).Return("", errors.New("db error"))

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestHandleListEvents(t *testing.T) {
	t.Run("public list", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Get("/events", server.handleListEvents)

		req := httptest.NewRequest("GET", "/events", nil)
		rec := httptest.NewRecorder()

		events := []Event{{ID: "1", Name: "Public Event"}}
		mockStore.On("ListEvents", mock.Anything, "", "public").Return(events, nil)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("user list", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Get("/events", server.handleListEvents)

		req := httptest.NewRequest("GET", "/events", nil)
		req.Header.Set("X-User-Id", "user1")
		rec := httptest.NewRecorder()

		events := []Event{{ID: "1", Name: "My Event"}}
		mockStore.On("ListEvents", mock.Anything, "user1", "public").Return(events, nil)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("store error", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Get("/events", server.handleListEvents)

		req := httptest.NewRequest("GET", "/events", nil)
		rec := httptest.NewRecorder()

		mockStore.On("ListEvents", mock.Anything, "", "public").Return([]Event(nil), errors.New("db error"))

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestHandleDeleteEvent(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Delete("/events/{id}", server.handleDeleteEvent)

		req := httptest.NewRequest("DELETE", "/events/ev1", nil)
		req.Header.Set("X-User-Id", "owner")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", OwnerID: "owner"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)
		mockStore.On("DeleteEvent", mock.Anything, "ev1").Return(nil)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("unauthorized", func(t *testing.T) {
		server := &HTTPServer{}
		r := chi.NewRouter()
		r.Delete("/events/{id}", server.handleDeleteEvent)

		req := httptest.NewRequest("DELETE", "/events/ev1", nil)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("not found", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Delete("/events/{id}", server.handleDeleteEvent)

		req := httptest.NewRequest("DELETE", "/events/ev1", nil)
		req.Header.Set("X-User-Id", "owner")
		rec := httptest.NewRecorder()

		mockStore.On("LoadEvent", mock.Anything, "ev1").Return((*Event)(nil), pgx.ErrNoRows)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("forbidden", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Delete("/events/{id}", server.handleDeleteEvent)

		req := httptest.NewRequest("DELETE", "/events/ev1", nil)
		req.Header.Set("X-User-Id", "other")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", OwnerID: "owner"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

func TestHandlePatchEvent(t *testing.T) {
	t.Run("success update name", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Patch("/events/{id}", server.handlePatchEvent)

		payload := map[string]any{"name": "Updated Name"}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("PATCH", "/events/ev1", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "owner")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", OwnerID: "owner", Name: "Old Name"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)
		mockStore.On("UpdateEvent", mock.Anything, "ev1", mock.MatchedBy(func(m map[string]any) bool {
			return m["name"] == "Updated Name"
		})).Return(nil)

		// Fetch after update
		updated := &Event{ID: "ev1", OwnerID: "owner", Name: "Updated Name"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(updated, nil)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("clear vote times", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Patch("/events/{id}", server.handlePatchEvent)

		payload := map[string]any{"vote_start": "", "vote_end": ""}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("PATCH", "/events/ev1", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "owner")
		rec := httptest.NewRecorder()

		now := time.Now()
		ev := &Event{ID: "ev1", OwnerID: "owner", VoteStart: &now}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)
		mockStore.On("UpdateEvent", mock.Anything, "ev1", mock.MatchedBy(func(m map[string]any) bool {
			start, okS := m["vote_start"]
			end, okE := m["vote_end"]
			return okS && start == nil && okE && end == nil
		})).Return(nil)

		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("no updates", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Patch("/events/{id}", server.handlePatchEvent)

		payload := map[string]any{}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("PATCH", "/events/ev1", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "owner")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", OwnerID: "owner"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("invalid license mode", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Patch("/events/{id}", server.handlePatchEvent)

		payload := map[string]any{"license_mode": "invalid"}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("PATCH", "/events/ev1", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "owner")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", OwnerID: "owner"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)
		mockStore.On("UpdateEvent", mock.Anything, "ev1", mock.MatchedBy(func(m map[string]any) bool {
			return m["license_mode"] == "invalid"
		})).Return(nil)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "invalid license mode")
	})

	t.Run("update store error", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Patch("/events/{id}", server.handlePatchEvent)

		payload := map[string]any{"name": "New Name"}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("PATCH", "/events/ev1", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "owner")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", OwnerID: "owner"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)
		mockStore.On("UpdateEvent", mock.Anything, "ev1", mock.Anything).Return(errors.New("db fail"))

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("load event error after update", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Patch("/events/{id}", server.handlePatchEvent)

		payload := map[string]any{"name": "New Name"}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("PATCH", "/events/ev1", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "owner")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", OwnerID: "owner"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil).Once()
		mockStore.On("UpdateEvent", mock.Anything, "ev1", mock.Anything).Return(nil)
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return((*Event)(nil), errors.New("load error")).Once()

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestHandleTransferOwnership(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Post("/events/{id}/transfer", server.handleTransferOwnership)

		payload := map[string]string{"newOwnerId": "newOwner"}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/events/ev1/transfer", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "owner")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", OwnerID: "owner"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)
		mockStore.On("TransferOwnership", mock.Anything, "ev1", "newOwner").Return(nil)
		mockStore.On("CreateInvite", mock.Anything, "ev1", "owner").Return(nil)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestHandleGetEventFull(t *testing.T) {
	t.Run("private event missing user id", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Get("/events/{id}", server.handleGetEvent)

		req := httptest.NewRequest("GET", "/events/ev1", nil)
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", Visibility: "private", OwnerID: "owner"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("is invited store error", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Get("/events/{id}", server.handleGetEvent)

		req := httptest.NewRequest("GET", "/events/ev1", nil)
		req.Header.Set("X-User-Id", "stranger")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", Visibility: "private", OwnerID: "owner"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)
		mockStore.On("IsInvited", mock.Anything, "ev1", "stranger").Return(false, errors.New("db error"))

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("is joined for owner", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Get("/events/{id}", server.handleGetEvent)

		req := httptest.NewRequest("GET", "/events/ev1", nil)
		req.Header.Set("X-User-Id", "owner")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", Visibility: "public", OwnerID: "owner"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var resp Event
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.True(t, resp.IsJoined)
	})
}

func TestHandlePatchEventDeep(t *testing.T) {
	t.Run("invalid license mode", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Patch("/events/{id}", server.handlePatchEvent)

		payload := map[string]any{"license_mode": "geo_time", "vote_start": "invalid"}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("PATCH", "/events/ev1", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "owner")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", OwnerID: "owner"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("update geo config", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Patch("/events/{id}", server.handlePatchEvent)

		lat, lng, rad := 1.2, 3.4, 500
		payload := map[string]any{"geo_lat": lat, "geo_lng": lng, "geo_radius_m": rad}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("PATCH", "/events/ev1", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "owner")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", OwnerID: "owner"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)
		mockStore.On("UpdateEvent", mock.Anything, "ev1", mock.MatchedBy(func(m map[string]any) bool {
			return m["geo_lat"] == lat && m["geo_lng"] == lng && m["geo_radius_m"] == rad
		})).Return(nil)
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("store update error", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Patch("/events/{id}", server.handlePatchEvent)

		payload := map[string]any{"name": "New Name"}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("PATCH", "/events/ev1", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "owner")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", OwnerID: "owner"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)
		mockStore.On("UpdateEvent", mock.Anything, "ev1", mock.Anything).Return(errors.New("db error"))

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestHandleTransferOwnershipDeep(t *testing.T) {
	t.Run("event not found", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Post("/events/{id}/transfer", server.handleTransferOwnership)

		payload := map[string]string{"newOwnerId": "newOwner"}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/events/ev1/transfer", bytes.NewReader(b))
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
		r.Post("/events/{id}/transfer", server.handleTransferOwnership)

		payload := map[string]string{"newOwnerId": "newOwner"}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/events/ev1/transfer", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "stranger")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", OwnerID: "owner"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("bad request invalid body", func(t *testing.T) {
		server := &HTTPServer{}
		r := chi.NewRouter()
		r.Post("/events/{id}/transfer", server.handleTransferOwnership)

		req := httptest.NewRequest("POST", "/events/ev1/transfer", bytes.NewBufferString("{invalid}"))
		req.Header.Set("X-User-Id", "owner")
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("bad request missing newOwnerId", func(t *testing.T) {
		server := &HTTPServer{}
		r := chi.NewRouter()
		r.Post("/events/{id}/transfer", server.handleTransferOwnership)

		payload := map[string]string{"newOwnerId": ""}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/events/ev1/transfer", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "owner")
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("transfer ownership store error", func(t *testing.T) {
		mockStore := new(MockStore)
		server := &HTTPServer{store: mockStore}
		r := chi.NewRouter()
		r.Post("/events/{id}/transfer", server.handleTransferOwnership)

		payload := map[string]string{"newOwnerId": "newOwner"}
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/events/ev1/transfer", bytes.NewReader(b))
		req.Header.Set("X-User-Id", "owner")
		rec := httptest.NewRecorder()

		ev := &Event{ID: "ev1", OwnerID: "owner"}
		mockStore.On("LoadEvent", mock.Anything, "ev1").Return(ev, nil)
		mockStore.On("TransferOwnership", mock.Anything, "ev1", "newOwner").Return(errors.New("db error"))

		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}
