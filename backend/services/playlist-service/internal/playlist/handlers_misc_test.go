package playlist

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestHandleHealth_Success(t *testing.T) {
	mockDB := &MockDB{}
	srv := NewServer(mockDB, nil)
	r := chi.NewRouter()
	r.Get("/health", srv.handleHealth)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", w.Code)
	}
	// Body check optional, mostly just ensuring it doesn't crash
}

func TestHandleBroadcastEvent_Success(t *testing.T) {
	mockDB := &MockDB{}
	// Note: We are passing nil for redis client.
	// publishEvent checks for s.rdb != nil before publishing,
	// so this allows us to test the HTTP handler logic without crashing.
	srv := NewServer(mockDB, nil)

	r := chi.NewRouter()
	r.Post("/realtime/event", srv.handleBroadcastEvent)

	body, _ := json.Marshal(map[string]any{
		"type": "some_event",
		"payload": map[string]any{
			"foo": "bar",
		},
	})
	req := httptest.NewRequest("POST", "/realtime/event", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", w.Code)
	}
}
