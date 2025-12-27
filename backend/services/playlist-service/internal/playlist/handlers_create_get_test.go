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
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

func TestHandleCreatePlaylist_Errors(t *testing.T) {
	tests := []struct {
		name      string
		userID    string
		body      map[string]any
		mockSetup func(*MockDB)
		wantCode  int
	}{
		{
			name:      "Missing User ID",
			userID:    "",
			body:      map[string]any{"name": "My Playlist"},
			mockSetup: func(m *MockDB) {},
			wantCode:  http.StatusUnauthorized,
		},
		{
			name:      "Invalid JSON",
			userID:    "user-1",
			body:      nil,
			mockSetup: func(m *MockDB) {},
			wantCode:  http.StatusBadRequest,
		},
		{
			name:      "Empty Name",
			userID:    "user-1",
			body:      map[string]any{"name": "   "},
			mockSetup: func(m *MockDB) {},
			wantCode:  http.StatusBadRequest,
		},
		{
			name:      "Name Too Long",
			userID:    "user-1",
			body:      map[string]any{"name": strings.Repeat("a", 201)},
			mockSetup: func(m *MockDB) {},
			wantCode:  http.StatusBadRequest,
		},
		{
			name:      "Description Too Long",
			userID:    "user-1",
			body:      map[string]any{"name": "OK", "description": strings.Repeat("a", 1001)},
			mockSetup: func(m *MockDB) {},
			wantCode:  http.StatusBadRequest,
		},
		{
			name:      "Invalid Edit Mode",
			userID:    "user-1",
			body:      map[string]any{"name": "OK", "editMode": "invalid"},
			mockSetup: func(m *MockDB) {},
			wantCode:  http.StatusBadRequest,
		},
		{
			name:   "DB Error",
			userID: "user-1",
			body:   map[string]any{"name": "OK"},
			mockSetup: func(m *MockDB) {
				m.QueryRowFunc = func(ctx context.Context, sql string, args ...any) pgx.Row {
					return &MockRow{ScanFunc: func(dest ...any) error {
						return errors.New("db error")
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
			r.Post("/playlists", srv.handleCreatePlaylist)

			var bodyBytes []byte
			if tt.body != nil {
				bodyBytes, _ = json.Marshal(tt.body)
			} else {
				bodyBytes = []byte("invalid-json")
			}
			req := httptest.NewRequest("POST", "/playlists", bytes.NewReader(bodyBytes))
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

func TestHandleGetPlaylist_Errors(t *testing.T) {
	tests := []struct {
		name       string
		playlistID string
		userID     string
		mockSetup  func(*MockDB)
		wantCode   int
	}{
		{
			name:       "Not Found",
			playlistID: "pl-missing",
			userID:     "user-1",
			mockSetup: func(m *MockDB) {
				m.QueryRowFunc = func(ctx context.Context, sql string, args ...any) pgx.Row {
					return &MockRow{ScanFunc: func(dest ...any) error { return pgx.ErrNoRows }}
				}
			},
			wantCode: http.StatusNotFound,
		},
		{
			name:       "DB Error on Fetch",
			playlistID: "pl-1",
			userID:     "user-1",
			mockSetup: func(m *MockDB) {
				m.QueryRowFunc = func(ctx context.Context, sql string, args ...any) pgx.Row {
					return &MockRow{ScanFunc: func(dest ...any) error { return errors.New("db error") }}
				}
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			name:       "Private & Not Owner & Not Invited",
			playlistID: "pl-priv",
			userID:     "outsider",
			mockSetup: func(m *MockDB) {
				m.QueryRowFunc = func(ctx context.Context, sql string, args ...any) pgx.Row {
					// 1. Fetch playlist
					if strings.Contains(sql, "FROM playlists") {
						return &MockRow{ScanFunc: func(dest ...any) error {
							*dest[0].(*string) = "pl-priv"
							*dest[1].(*string) = "owner"
							*dest[2].(*string) = "Private"
							*dest[3].(*string) = "Desc"
							*dest[4].(*bool) = false // Private
							*dest[5].(*string) = "everyone"
							*dest[6].(*time.Time) = time.Now()
							*dest[7].(**string) = nil
							*dest[8].(**time.Time) = nil
							return nil
						}}
					}
					// 2. Check Member
					if strings.Contains(sql, "FROM playlist_members") {
						return &MockRow{ScanFunc: func(dest ...any) error { return pgx.ErrNoRows }} // Not invited
					}
					return &MockRow{}
				}
			},
			wantCode: http.StatusForbidden,
		},
		{
			name:       "DB Error on Tracks",
			playlistID: "pl-1",
			userID:     "owner",
			mockSetup: func(m *MockDB) {
				m.QueryRowFunc = func(ctx context.Context, sql string, args ...any) pgx.Row {
					if strings.Contains(sql, "FROM playlists") {
						return &MockRow{ScanFunc: func(dest ...any) error {
							*dest[0].(*string) = "pl-1"
							*dest[1].(*string) = "owner"
							*dest[2].(*string) = "Public"
							*dest[3].(*string) = "Desc"
							*dest[4].(*bool) = true // Public
							*dest[5].(*string) = "everyone"
							*dest[6].(*time.Time) = time.Now()
							*dest[7].(**string) = nil
							*dest[8].(**time.Time) = nil
							return nil
						}}
					}
					return &MockRow{}
				}
				m.QueryFunc = func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
					return nil, errors.New("db error tracks")
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
			r.Get("/playlists/{id}", srv.handleGetPlaylist)

			req := httptest.NewRequest("GET", fmt.Sprintf("/playlists/%s", tt.playlistID), nil)
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
