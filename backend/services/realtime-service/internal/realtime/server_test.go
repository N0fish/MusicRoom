package realtime

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

func TestServer_HandleHealth(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	s.handleHealth(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.Status)
	}
}

func TestServer_HandleWS(t *testing.T) {
	// Setup miniredis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	hub := NewHub()
	go hub.Run()

	s := NewServer(hub, rdb, context.Background(), "http://localhost:3000")

	t.Run("Upgrade Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(s.handleWS))
		defer server.Close()

		url := "ws" + strings.TrimPrefix(server.URL, "http")

		// Use a dialer that sets the Origin header to match allowed origin
		dialer := websocket.DefaultDialer
		header := http.Header{}
		header.Set("Origin", "http://localhost:3000")

		ws, _, err := dialer.Dial(url, header)
		if err != nil {
			t.Fatalf("Failed to dial: %v", err)
		}
		defer ws.Close()
	})

	t.Run("Forbidden Origin", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(s.handleWS))
		defer server.Close()

		url := "ws" + strings.TrimPrefix(server.URL, "http")

		dialer := websocket.DefaultDialer
		header := http.Header{}
		header.Set("Origin", "http://evil.com")

		_, resp, err := dialer.Dial(url, header)
		if err == nil {
			t.Fatal("Expected error dialing with bad origin, got nil")
		}
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("Expected 403 Forbidden, got %v", resp.StatusCode)
		}
	})
}

func TestServer_HandleEvents(t *testing.T) {
	// Setup miniredis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	s := NewServer(nil, rdb, context.Background(), "") // Hub not needed for this handler test

	payload := map[string]string{"event": "test_event"}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/events", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	s.handleEvents(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.Status)
	}

	// Verify published to Redis
	// miniredis doesn't inherently store publish history in a simple way to query directly
	// without a subscriber, but we can check if it received the command or we can subscribe.
	// We'll use a real subscriber test in integration test below.
}

func TestServer_Router(t *testing.T) {
	s := NewServer(nil, nil, context.Background(), "")
	r := s.Router()

	tests := []struct {
		method string
		path   string
	}{
		{"GET", "/health"},
		{"GET", "/ws"},
		{"POST", "/events"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(tt.method, tt.path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// We just want to check that the router routes to *some* handler
		// and doesn't return 404 (unless the handler itself returns 404, which none of ours do for valid paths)
		if w.Result().StatusCode == http.StatusNotFound {
			t.Errorf("Expected route %s %s to be registered, got 404", tt.method, tt.path)
		}
	}
}

func TestServer_HandleEvents_Errors(t *testing.T) {
	// Setup miniredis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	s := NewServer(nil, rdb, context.Background(), "")

	t.Run("Invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/events", bytes.NewBufferString("invalid json"))
		w := httptest.NewRecorder()
		s.handleEvents(w, req)
		if w.Result().StatusCode != http.StatusBadRequest {
			t.Errorf("Expected 400 Bad Request, got %v", w.Result().StatusCode)
		}
	})

	t.Run("Redis Error", func(t *testing.T) {
		mr.SetError("redis connection failed")

		payload := map[string]string{"event": "test"}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/events", bytes.NewBuffer(body))
		w := httptest.NewRecorder()

		s.handleEvents(w, req)

		if w.Result().StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected 500 Internal Server Error, got %v", w.Result().StatusCode)
		}
	})
}

func TestIntegration_RedisPubSub(t *testing.T) {
	// Full integration: Post -> Server -> Redis -> Server(Subscriber) -> Hub -> Client

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	hub := NewHub()
	go hub.Run()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := NewServer(hub, rdb, ctx, "")

	// Start Redis Subscriber in background
	go s.RunRedisSubscriber()

	// Wait for subscription to establish (naive wait)
	time.Sleep(50 * time.Millisecond)

	// Register a client manually to the Hub
	clientWs, internalClient, cleanup := createTestConnectedClient(t, hub)
	defer cleanup()

	hub.register <- internalClient
	time.Sleep(20 * time.Millisecond)

	// Send an event via HTTP handler
	payload := map[string]string{"msg": "integration_test"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/events", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	s.handleEvents(w, req)

	// Verify Client received it
	_, message, err := clientWs.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read from websocket: %v", err)
	}

	// The server marshals the payload again before publishing, so expect JSON string matching
	// We sent `{"msg":"integration_test"}` -> handleEvents decodes -> marshals -> publishes
	// Subscriber receives -> sends to hub -> sends to client
	// So we expect `{"msg":"integration_test"}`
	expected, _ := json.Marshal(payload)
	if string(message) != string(expected) {
		t.Errorf("Expected %s, got %s", expected, message)
	}
}

// Helper duplicated/adapted from hub_test.go for reuse
func createTestConnectedClient(t *testing.T, hub *Hub) (*websocket.Conn, *Client, func()) {
	var internalClient *Client
	var createdWg sync.WaitGroup
	createdWg.Add(1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("Failed to upgrade: %v", err)
			return
		}
		client := &Client{
			hub:  hub,
			conn: conn,
			send: make(chan []byte, 256),
		}
		internalClient = client
		createdWg.Done()
		go client.writePump()
		go client.readPump()
	}))

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	clientWs, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}

	createdWg.Wait()

	return clientWs, internalClient, func() {
		server.Close()
		clientWs.Close()
	}
}
