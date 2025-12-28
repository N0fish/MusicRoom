package playlist

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestHandleAddTrack_Success(t *testing.T) {
	// Setup
	mockDB := &MockDB{}
	srv := NewServer(mockDB, nil) // No redis needed for this test
	r := chi.NewRouter()
	r.Post("/playlists/{id}/tracks", srv.handleAddTrack)

	userID := "user-123"
	playlistID := "pl-456"

	// Mock DB Expectations
	// 1. getPlaylistAccessInfo: SELECT owner_id ...
	mockDB.QueryRowFunc = func(ctx context.Context, sql string, args ...any) pgx.Row {
		if strings.Contains(sql, "SELECT owner_id") {
			return &MockRow{
				ScanFunc: func(dest ...any) error {
					// owner_id, is_public, edit_mode
					*dest[0].(*string) = userID // Owner matches
					*dest[1].(*bool) = true
					*dest[2].(*string) = "everyone"
					return nil
				},
			}
		}
		// 2. Insert Track: INSERT INTO tracks ...
		if strings.Contains(sql, "INSERT INTO tracks") {
			return &MockRow{
				ScanFunc: func(dest ...any) error {
					// id, playlist_id, title...
					*dest[0].(*string) = "track-789"
					*dest[1].(*string) = playlistID
					*dest[2].(*string) = "Song Title"
					return nil
				},
			}
		}
		return &MockRow{ScanFunc: func(dest ...any) error { return errors.New("unexpected query") }}
	}

	// Request
	body, _ := json.Marshal(map[string]any{
		"title":           "Song Title",
		"artist":          "Artist Name",
		"provider":        "youtube",
		"providerTrackId": "vid123",
		"durationMs":      60000,
	})
	req := httptest.NewRequest("POST", fmt.Sprintf("/playlists/%s/tracks", playlistID), bytes.NewReader(body))
	req.Header.Set("X-User-Id", userID)

	// Execute
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Assert
	if w.Code != http.StatusCreated {
		t.Errorf("Expected 201 Created, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestHandleMoveTrack_Success(t *testing.T) {
	// Setup
	mockDB := &MockDB{}
	srv := NewServer(mockDB, nil)
	r := chi.NewRouter()
	r.Patch("/playlists/{id}/tracks/{trackId}", srv.handleMoveTrack)

	userID := "user-123"
	playlistID := "pl-456"
	trackID := "track-789"

	// Existing: Position 5. Moving to: Position 2.
	currentPos := 5
	newPos := 2
	totalTracks := 10

	// 1. Check Access (QueryRow)
	mockDB.QueryRowFunc = func(ctx context.Context, sql string, args ...any) pgx.Row {
		if strings.Contains(sql, "SELECT owner_id") {
			return &MockRow{
				ScanFunc: func(dest ...any) error {
					*dest[0].(*string) = userID
					*dest[1].(*bool) = true
					*dest[2].(*string) = "everyone"
					return nil
				},
			}
		}
		return &MockRow{ScanFunc: func(dest ...any) error { return errors.New("unexpected non-tx query") }}
	}

	// 2. BeginTx
	mockTx := &MockTx{}
	mockDB.BeginTxFunc = func(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error) {
		return mockTx, nil
	}

	// Tx Operations
	mockTx.QueryRowFunc = func(ctx context.Context, sql string, args ...any) pgx.Row {
		// 3. Select existing pos: SELECT playlist_id, position ...
		if strings.Contains(sql, "SELECT playlist_id, position") {
			return &MockRow{
				ScanFunc: func(dest ...any) error {
					*dest[0].(*string) = playlistID
					*dest[1].(*int) = currentPos
					return nil
				},
			}
		}
		// 4. Count: SELECT COUNT(*) ...
		if strings.Contains(sql, "SELECT COUNT(*)") {
			return &MockRow{
				ScanFunc: func(dest ...any) error {
					*dest[0].(*int) = totalTracks
					return nil
				},
			}
		}
		return &MockRow{ScanFunc: func(dest ...any) error { return errors.New("unexpected tx query") }}
	}

	mockTx.ExecFunc = func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
		// 5. Shift existing tracks: UPDATE tracks SET position ...
		// 6. Set new position: UPDATE tracks SET position = $3 ...
		// We can just return success
		return pgconn.CommandTag{}, nil
	}

	mockTx.CommitFunc = func(ctx context.Context) error {
		return nil
	}

	mockTx.RollbackFunc = func(ctx context.Context) error {
		return nil
	}

	// Request
	body, _ := json.Marshal(map[string]any{
		"newPosition": newPos,
	})
	req := httptest.NewRequest("PATCH", fmt.Sprintf("/playlists/%s/tracks/%s", playlistID, trackID), bytes.NewReader(body))
	req.Header.Set("X-User-Id", userID)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestHandleAddTrack_Forbidden(t *testing.T) {
	mockDB := &MockDB{}
	srv := NewServer(mockDB, nil)
	r := chi.NewRouter()
	r.Post("/playlists/{id}/tracks", srv.handleAddTrack)

	userID := "user-outsider"
	playlistID := "pl-private"

	mockDB.QueryRowFunc = func(ctx context.Context, sql string, args ...any) pgx.Row {
		if strings.Contains(sql, "SELECT owner_id") {
			return &MockRow{
				ScanFunc: func(dest ...any) error {
					*dest[0].(*string) = "user-owner" // Different provider
					*dest[1].(*bool) = false          // Private
					*dest[2].(*string) = "invited"
					return nil
				},
			}
		}
		if strings.Contains(sql, "SELECT user_id") { // Invited check
			return &MockRow{
				ScanFunc: func(dest ...any) error {
					return pgx.ErrNoRows // Not invite
				},
			}
		}
		return &MockRow{}
	}

	body, _ := json.Marshal(map[string]any{
		"title": "Song",
	})
	req := httptest.NewRequest("POST", fmt.Sprintf("/playlists/%s/tracks", playlistID), bytes.NewReader(body))
	req.Header.Set("X-User-Id", userID)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden, got %d", w.Code)
	}
}

func TestHandleDeleteTrack_Success(t *testing.T) {
	mockDB := &MockDB{}
	srv := NewServer(mockDB, nil)
	r := chi.NewRouter()
	r.Delete("/playlists/{id}/tracks/{trackId}", srv.handleDeleteTrack)

	userID := "owner-123"
	playlistID := "pl-001"
	trackID := "track-removed"

	// 1. BeginTx
	mockDB.BeginTxFunc = func(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error) {
		return &MockTx{
			QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
				// Access Info & Track Info
				if strings.Contains(sql, "FROM playlists") {
					return &MockRow{
						ScanFunc: func(dest ...any) error {
							*dest[0].(*string) = userID     // owner
							*dest[1].(*bool) = false        // public
							*dest[2].(*string) = "everyone" // edit mode
							*dest[3].(*string) = userID     // added_by
							return nil
						},
					}
				}
				// Get Track Pos
				if strings.Contains(sql, "SELECT position") && strings.Contains(sql, "FROM tracks") {
					return &MockRow{
						ScanFunc: func(dest ...any) error {
							*dest[0].(*int) = 1 // position is int
							return nil
						},
					}
				}
				return &MockRow{
					ScanFunc: func(dest ...any) error {
						return errors.New("unexpected query: " + sql)
					},
				}
			},
			ExecFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				// Delete
				if strings.Contains(sql, "DELETE FROM tracks") {
					return pgconn.CommandTag{}, nil
				}
				// Update (Compact)
				if strings.Contains(sql, "UPDATE tracks") {
					return pgconn.CommandTag{}, nil
				}
				return pgconn.CommandTag{}, nil
			},
			CommitFunc: func(ctx context.Context) error {
				return nil
			},
		}, nil
	}

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/playlists/%s/tracks/%s", playlistID, trackID), nil)
	req.Header.Set("X-User-Id", userID)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected 204 No Content, got %d", w.Code)
		t.Logf("Response body: %s", w.Body.String())
	}
}

func TestHandleAddTrack_Errors(t *testing.T) {
	tests := []struct {
		name       string
		playlistID string
		userID     string
		body       map[string]any
		mockSetup  func(*MockDB)
		wantCode   int
	}{
		{
			name:       "Missing User ID",
			playlistID: "pl-1",
			userID:     "",
			body:       map[string]any{"title": "Song"},
			mockSetup:  func(m *MockDB) {},
			wantCode:   http.StatusUnauthorized,
		},
		{
			name:       "Invalid JSON",
			playlistID: "pl-1",
			userID:     "user-1",
			body:       nil,
			mockSetup: func(m *MockDB) {
				m.QueryRowFunc = func(ctx context.Context, sql string, args ...any) pgx.Row {
					if strings.Contains(sql, "FROM playlists") {
						return &MockRow{ScanFunc: func(dest ...any) error {
							*dest[0].(*string) = "user-1"
							*dest[1].(*bool) = true
							*dest[2].(*string) = "everyone"
							return nil
						}}
					}
					return &MockRow{}
				}
			},
			wantCode: http.StatusBadRequest,
		},
		{
			name:       "DB Error on Access Info",
			playlistID: "pl-1",
			userID:     "user-1",
			body:       map[string]any{"title": "Song"},
			mockSetup: func(m *MockDB) {
				m.QueryRowFunc = func(ctx context.Context, sql string, args ...any) pgx.Row {
					return &MockRow{ScanFunc: func(dest ...any) error {
						return errors.New("db explosion")
					}}
				}
			},
			wantCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := &MockDB{}
			if tt.mockSetup != nil {
				tt.mockSetup(mockDB)
			}
			srv := NewServer(mockDB, nil)
			r := chi.NewRouter()
			r.Post("/playlists/{id}/tracks", srv.handleAddTrack)

			var bodyBytes []byte
			if tt.body != nil {
				bodyBytes, _ = json.Marshal(tt.body)
			} else {
				bodyBytes = []byte("invalid-json")
			}

			url := fmt.Sprintf("/playlists/pl-1/tracks")
			if tt.playlistID != "" {
				url = fmt.Sprintf("/playlists/%s/tracks", tt.playlistID)
			}

			req := httptest.NewRequest("POST", url, bytes.NewReader(bodyBytes))
			if tt.userID != "" {
				req.Header.Set("X-User-Id", tt.userID)
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.wantCode {
				t.Errorf("expected %d, got %d", tt.wantCode, w.Code)
			}
		})
	}
}

func TestHandleMoveTrack_Errors(t *testing.T) {
	// Table driven tests for missing params, forbidden, db errors...
	tests := []struct {
		name       string
		playlistID string
		trackID    string
		userID     string
		body       map[string]any
		mockSetup  func(*MockDB)
		wantCode   int
	}{
		{
			name:       "Invalid JSON",
			playlistID: "pl-1",
			trackID:    "tr-1",
			userID:     "user-1",
			body:       nil,
			mockSetup:  func(m *MockDB) {},
			wantCode:   http.StatusBadRequest,
		},
		{
			name:       "Negative Position",
			playlistID: "pl-1",
			trackID:    "tr-1",
			userID:     "user-1",
			body:       map[string]any{"newPosition": -1},
			mockSetup:  func(m *MockDB) {},
			wantCode:   http.StatusBadRequest,
		},
		{
			name:       "Playlist Not Found",
			playlistID: "pl-1",
			trackID:    "tr-1",
			userID:     "user-1",
			body:       map[string]any{"newPosition": 2},
			mockSetup: func(m *MockDB) {
				m.QueryRowFunc = func(ctx context.Context, sql string, args ...any) pgx.Row {
					return &MockRow{ScanFunc: func(dest ...any) error { return pgx.ErrNoRows }}
				}
			},
			wantCode: http.StatusNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := &MockDB{}
			if tt.mockSetup != nil {
				tt.mockSetup(mockDB)
			}
			srv := NewServer(mockDB, nil)
			r := chi.NewRouter()
			r.Patch("/playlists/{id}/tracks/{trackId}", srv.handleMoveTrack)

			var bodyBytes []byte
			if tt.body != nil {
				bodyBytes, _ = json.Marshal(tt.body)
			} else {
				bodyBytes = []byte("invalid-json")
			}
			req := httptest.NewRequest("PATCH", fmt.Sprintf("/playlists/%s/tracks/%s", tt.playlistID, tt.trackID), bytes.NewReader(bodyBytes))
			if tt.userID != "" {
				req.Header.Set("X-User-Id", tt.userID)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tt.wantCode {
				t.Errorf("expected %d, got %d", tt.wantCode, w.Code)
			}
		})
	}
}

func TestHandleDeleteTrack_Errors(t *testing.T) {
	tests := []struct {
		name       string
		playlistID string
		trackID    string
		userID     string
		mockSetup  func(*MockDB)
		wantCode   int
	}{
		{
			name:       "Playlist Not Found",
			playlistID: "pl-1",
			trackID:    "tr-1",
			userID:     "user-1",
			mockSetup: func(m *MockDB) {
				m.QueryRowFunc = func(ctx context.Context, sql string, args ...any) pgx.Row {
					return &MockRow{ScanFunc: func(dest ...any) error { return pgx.ErrNoRows }}
				}
			},
			wantCode: http.StatusNotFound,
		},
		{
			name:       "Forbidden",
			playlistID: "pl-priv",
			trackID:    "tr-1",
			userID:     "outsider",
			mockSetup: func(m *MockDB) {
				m.QueryRowFunc = func(ctx context.Context, sql string, args ...any) pgx.Row {
					if strings.Contains(sql, "FROM playlists") {
						return &MockRow{ScanFunc: func(dest ...any) error {
							*dest[0].(*string) = "owner"
							*dest[1].(*bool) = false
							*dest[2].(*string) = "invited"
							return nil
						}}
					}
					// invited check
					return &MockRow{ScanFunc: func(dest ...any) error { return pgx.ErrNoRows }}
				}
			},
			wantCode: http.StatusForbidden,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := &MockDB{}
			if tt.mockSetup != nil {
				tt.mockSetup(mockDB)
			}
			srv := NewServer(mockDB, nil)
			r := chi.NewRouter()
			r.Delete("/playlists/{id}/tracks/{trackId}", srv.handleDeleteTrack)

			req := httptest.NewRequest("DELETE", fmt.Sprintf("/playlists/%s/tracks/%s", tt.playlistID, tt.trackID), nil)
			if tt.userID != "" {
				req.Header.Set("X-User-Id", tt.userID)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tt.wantCode {
				t.Errorf("expected %d, got %d", tt.wantCode, w.Code)
			}
		})
	}
}
