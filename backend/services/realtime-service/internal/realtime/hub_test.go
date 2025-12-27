package realtime

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

var testUpgrader = websocket.Upgrader{}

func TestHub_Run(t *testing.T) {
	// Create a new Hub
	hub := NewHub()
	go hub.Run()

	// Helper to create a connected client via valid websocket handshake
	// Returns:
	// - clientWs: The websocket connection held by the TEST (simulating the external user)
	// - internalClient: The *Client struct created inside the server handler (what the Hub sees)
	// - cleanup: A function to close servers and connections
	createConnectedClient := func() (*websocket.Conn, *Client, func()) {
		var internalClient *Client
		var createdWg sync.WaitGroup
		createdWg.Add(1)

		// Create a test server that upgrades usage to a websocket and registers a Client
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

			// Start the write pump to handle sending messages from Hub -> Websocket
			// We don't strictly need readPump for these tests unless we test incoming messages,
			// but we need to keep the connection alive.
			go client.writePump() // This will process the sending loop

			// We also need to read to handle ping/pong and close
			go client.readPump()
		}))

		// Convert http URL to ws URL
		url := "ws" + strings.TrimPrefix(server.URL, "http")

		// Connect to the server
		clientWs, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			t.Fatalf("Failed to dial: %v", err)
		}

		createdWg.Wait()

		cleanup := func() {
			server.Close()
			clientWs.Close()
			// internalClient internal connection is closed by writePump/readPump logic usually
		}

		return clientWs, internalClient, cleanup
	}

	t.Run("Register Client", func(t *testing.T) {
		clientWs, internalClient, cleanup := createConnectedClient()
		defer cleanup()

		hub.register <- internalClient

		// Allow some time for registration
		time.Sleep(50 * time.Millisecond)

		// Test Broadcast
		msg := []byte("hello")
		hub.broadcast <- msg

		// Read from the external websocket client
		_, received, err := clientWs.ReadMessage()
		if err != nil {
			t.Fatalf("Failed to read message: %v", err)
		}

		if string(received) != string(msg) {
			t.Errorf("Expected message %s, got %s", msg, received)
		}
	})

	t.Run("Unregister Client", func(t *testing.T) {
		_, internalClient, cleanup := createConnectedClient()
		defer cleanup()

		hub.register <- internalClient
		time.Sleep(10 * time.Millisecond)

		hub.unregister <- internalClient
		time.Sleep(50 * time.Millisecond)

		// Verify client.send is closed.
		// Since writePump closes the connection when channel is closed, we can check that too.
		select {
		case _, ok := <-internalClient.send:
			if ok {
				t.Error("Expected internalClient.send to be closed")
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("Timed out waiting for send channel close")
		}
	})

	t.Run("Broadcast to Multiple Clients", func(t *testing.T) {
		clientWs1, internalClient1, cleanup1 := createConnectedClient()
		defer cleanup1()
		clientWs2, internalClient2, cleanup2 := createConnectedClient()
		defer cleanup2()

		hub.register <- internalClient1
		hub.register <- internalClient2
		time.Sleep(50 * time.Millisecond)

		msg := []byte("broadcast_test")
		hub.broadcast <- msg

		// Helper to read verification
		verifyReceive := func(ws *websocket.Conn, name string) {
			_, received, err := ws.ReadMessage()
			if err != nil {
				t.Errorf("%s: Failed to read: %v", name, err)
				return
			}
			if string(received) != string(msg) {
				t.Errorf("%s: Expected %s, got %s", name, msg, received)
			}
		}

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			verifyReceive(clientWs1, "Client1")
		}()
		go func() {
			defer wg.Done()
			verifyReceive(clientWs2, "Client2")
		}()

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(500 * time.Millisecond):
			t.Error("Timeout waiting for clients to receive message")
		}
	})
}
