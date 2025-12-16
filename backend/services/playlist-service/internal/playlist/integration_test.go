package playlist

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// setupIntegrationTest connects to local DB or skips test.
// Returns a Server, a cleanup function, and the db pool.
func setupIntegrationTest(t *testing.T) (*Server, func(), *pgxpool.Pool) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://musicroom:musicroom@localhost:5432/musicroom?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("Skipping integration test: cannot connect to DB: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("Skipping integration test: cannot ping DB: %v", err)
	}

	// Create a dedicated server instance with nil Redis (we verify DB state)
	srv := NewServer(pool, nil)

	// Run Migrations
	if err := AutoMigrate(ctx, pool); err != nil {
		pool.Close()
		t.Fatalf("AutoMigrate failed: %v", err)
	}

	// Cleanup callback
	cleanup := func() {
		// Optional: Drop tables? No, keep data for inspection or use transaction rollback?
		// Transaction rollback for EACH test is best practice, but `setupIntegrationTest` does pool.
		// For now simple cleanup of created playlist is in the test function itself.
		pool.Close()
	}

	return srv, cleanup, pool
}

func TestVotingAndPlaybackFlow(t *testing.T) {
	srv, cleanup, pool := setupIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()

	// 1. Create a Playlist
	// We'll insert directly to DB or use handler? Handler is better integration.
	// But direct DB is easier for setup. Let's use handlers where possible to test them.

	router := srv.Router()

	// User ID for the test
	userID := "test-user-1"

	// Create Playlist
	createBody := map[string]any{
		"name":        "Integration Test Playlist",
		"description": "Testing code",
		"isPublic":    true,
	}
	body, _ := json.Marshal(createBody)
	req := httptest.NewRequest("POST", "/playlists", bytes.NewReader(body))
	req.Header.Set("X-User-Id", userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("Failed to create playlist: %d %s", w.Code, w.Body.String())
	}

	var pl Playlist
	json.Unmarshal(w.Body.Bytes(), &pl)
	playlistID := pl.ID
	t.Logf("Created playlist: %s", playlistID)

	// Cleanup data at end
	defer func() {
		pool.Exec(ctx, "DELETE FROM playlists WHERE id = $1", playlistID)
	}()

	// 2. Add Tracks (A, B, C)
	// Track A
	trackA := addTrack(t, router, userID, playlistID, "Track A", 0)
	// Track B
	trackB := addTrack(t, router, userID, playlistID, "Track B", 0)
	// Track C
	trackC := addTrack(t, router, userID, playlistID, "Track C", 0)

	// Verify Initial Order (A, B, C) via GET
	checkOrder(t, router, userID, playlistID, []string{trackA.ID, trackB.ID, trackC.ID})

	// 3. Vote for Track C
	// Vote 1
	voteForTrack(t, router, userID, playlistID, trackC.ID)

	// Verify Order: C (1 vote), A (0), B (0)
	// Note: A and B are tied at 0, order depends on created_at. A was created first.
	// Expected: C, A, B
	checkOrder(t, router, userID, playlistID, []string{trackC.ID, trackA.ID, trackB.ID})

	// Vote for B twice
	voteForTrack(t, router, userID, playlistID, trackB.ID)
	voteForTrack(t, router, userID, playlistID, trackB.ID)

	// Order: B(2), C(1), A(0)
	checkOrder(t, router, userID, playlistID, []string{trackB.ID, trackC.ID, trackA.ID})

	// 4. Play (Next Track)
	// Should pick B
	callNextTrack(t, router, userID, playlistID, trackB.ID, "playing")

	// Verify Order in GET:
	// Played/Playing tracks are sometimes returned at top or bottom?
	// `handlers_playlists.go` orders by `position ASC`.
	// Our `next_track` logic does existing `stopped` -> `played`.
	// Does it change position of the playing track?
	// `handleNextTrack`:
	// 	  UPDATE tracks SET status = 'playing' WHERE id = ...
	//    Does NOT change position.
	// `handleVoteTrack`:
	//    Only selects `status = 'queued'`.
	//    Only re-orders queued tracks.
	//    It puts them AFTER existing playing/played tracks?
	//    `SELECT COALESCE(MAX(position) + 1, 0) FROM tracks WHERE ... status != 'queued'`
	//    So queued tracks start AFTER playing/played.
	// Since B was at pos 0, and became playing, it stays at pos 0 (or close).
	// Queued tracks (C, A) are re-ordered after B.

	// Actually, `handleVoteTrack` recalculates positions for ALL queued tracks starting from `startPos`.
	// `startPos` = MAX(position of non-queued) + 1.
	// If B is playing, it is non-queued. Its position is likely 0 (from before).
	// So queued tracks C and A will start at 1.

	// Let's verify status of B is playing.
	checkTrackStatus(t, pool, trackB.ID, "playing")

	// 5. Play Next Again
	// Should pick C (since it has 1 vote, A has 0)
	callNextTrack(t, router, userID, playlistID, trackC.ID, "playing")

	// B should be 'played'
	checkTrackStatus(t, pool, trackB.ID, "played")
	// C should be 'playing'
	checkTrackStatus(t, pool, trackC.ID, "playing")
}

func addTrack(t *testing.T, r chi.Router, userID, playlistID, title string, duration int) Track {
	body, _ := json.Marshal(map[string]any{
		"title":           title,
		"artist":          "Test Artist",
		"provider":        "youtube",
		"providerTrackId": "vid",
		"thumbnailUrl":    "url",
		"durationMs":      duration,
	})
	req := httptest.NewRequest("POST", fmt.Sprintf("/playlists/%s/tracks", playlistID), bytes.NewReader(body))
	req.Header.Set("X-User-Id", userID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Add track failed: %d %s", w.Code, w.Body.String())
	}
	var tr Track
	json.Unmarshal(w.Body.Bytes(), &tr)
	return tr
}

func voteForTrack(t *testing.T, r chi.Router, userID, playlistID, trackID string) {
	req := httptest.NewRequest("POST", fmt.Sprintf("/playlists/%s/tracks/%s/vote", playlistID, trackID), nil)
	req.Header.Set("X-User-Id", userID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Vote failed: %d %s", w.Code, w.Body.String())
	}
}

func callNextTrack(t *testing.T, r chi.Router, userID, playlistID, expectedID, expectedStatus string) {
	req := httptest.NewRequest("POST", fmt.Sprintf("/playlists/%s/next", playlistID), nil)
	req.Header.Set("X-User-Id", userID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Next track failed: %d %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)

	if id, ok := resp["currentTrackId"].(string); !ok || id != expectedID {
		t.Errorf("Expected currentTrackId %s, got %v", expectedID, resp["currentTrackId"])
	}
	if status, ok := resp["status"].(string); !ok || status != expectedStatus {
		t.Errorf("Expected status %s, got %v", expectedStatus, resp["status"])
	}
}

func checkOrder(t *testing.T, r chi.Router, userID, playlistID string, expectedIDs []string) {
	req := httptest.NewRequest("GET", fmt.Sprintf("/playlists/%s", playlistID), nil)
	req.Header.Set("X-User-Id", userID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var res struct {
		Tracks []Track `json:"tracks"`
	}
	json.Unmarshal(w.Body.Bytes(), &res)

	if len(res.Tracks) != len(expectedIDs) {
		t.Errorf("Expected %d tracks, got %d", len(expectedIDs), len(res.Tracks))
		return
	}

	for i, tr := range res.Tracks {
		if tr.ID != expectedIDs[i] {
			t.Errorf("Index %d: expected %s, got %s (Title: %s, Votes: %d)", i, expectedIDs[i], tr.ID, tr.Title, tr.VoteCount)
		}
	}
}

func checkTrackStatus(t *testing.T, pool *pgxpool.Pool, trackID, expectedStatus string) {
	var status string
	err := pool.QueryRow(context.Background(), "SELECT status FROM tracks WHERE id=$1", trackID).Scan(&status)
	if err != nil {
		t.Fatalf("Check status failed: %v", err)
	}
	if status != expectedStatus {
		t.Errorf("Track %s status: expected %sw, got %s", trackID, expectedStatus, status)
	}
}
