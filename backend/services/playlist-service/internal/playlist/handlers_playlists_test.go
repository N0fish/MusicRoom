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
	"github.com/jackc/pgx/v5/pgconn"
)

func TestHandleListPublicPlaylists_Success(t *testing.T) {
	mockDB := &MockDB{}
	srv := NewServer(mockDB, nil)
	r := chi.NewRouter()
	r.Get("/playlists/public", srv.handleListPublicPlaylists)

	mockDB.QueryFunc = func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
		if strings.Contains(sql, "FROM playlists") && strings.Contains(sql, "is_public = TRUE") {
			return &MockRows{
				Data: [][]any{
					{
						"pl-1", "user-1", "Public List", "Desc", true, "everyone", time.Now(),
					},
				},
				Idx: -1,
			}, nil
		}
		return nil, errors.New("unexpected query: " + sql)
	}

	req := httptest.NewRequest("GET", "/playlists/public", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", w.Code)
	}

	var playlists []Playlist
	json.NewDecoder(w.Body).Decode(&playlists)
	if len(playlists) != 1 {
		t.Errorf("Expected 1 playlist, got %d", len(playlists))
	}
	if playlists[0].ID != "pl-1" {
		t.Errorf("Expected pl-1, got %s", playlists[0].ID)
	}
}

func TestHandlePatchPlaylist_Success(t *testing.T) {
	mockDB := &MockDB{}
	srv := NewServer(mockDB, nil)
	r := chi.NewRouter()
	r.Patch("/playlists/{id}", srv.handlePatchPlaylist)

	userID := "owner-123"
	playlistID := "pl-001"

	// 1. BeginTx
	mockDB.BeginTxFunc = func(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error) {
		return &MockTx{
			QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
				// Select existing playlist
				return &MockRow{
					ScanFunc: func(dest ...any) error {
						*dest[0].(*string) = playlistID
						*dest[1].(*string) = userID
						*dest[2].(*string) = "Old Name"
						*dest[3].(*string) = "Old Desc"
						*dest[4].(*bool) = false
						*dest[5].(*string) = "invited"
						*dest[6].(*time.Time) = time.Now()
						return nil
					},
				}
			},
			ExecFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				// Update
				if strings.Contains(sql, "UPDATE playlists") {
					return pgconn.CommandTag{}, nil
				}
				return pgconn.CommandTag{}, errors.New("unexpected exec")
			},
			CommitFunc: func(ctx context.Context) error {
				return nil
			},
		}, nil
	}

	newName := "New Name"
	body, _ := json.Marshal(map[string]any{"name": newName})
	req := httptest.NewRequest("PATCH", fmt.Sprintf("/playlists/%s", playlistID), bytes.NewReader(body))
	req.Header.Set("X-User-Id", userID)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", w.Code)
	}

	var pl Playlist
	json.NewDecoder(w.Body).Decode(&pl)
	if pl.Name != newName {
		t.Errorf("Expected name %s, got %s", newName, pl.Name)
	}
}

func TestHandlePatchPlaylist_Errors(t *testing.T) {
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
			body:       map[string]any{"name": "New"},
			mockSetup:  func(m *MockDB) {},
			wantCode:   http.StatusUnauthorized,
		},
		{
			name:       "Invalid JSON",
			playlistID: "pl-1",
			userID:     "user-1",
			body:       nil,
			mockSetup:  func(m *MockDB) {},
			wantCode:   http.StatusBadRequest,
		},
		{
			name:       "Not Owner (Forbidden)",
			playlistID: "pl-1",
			userID:     "outsider",
			body:       map[string]any{"name": "New"},
			mockSetup: func(m *MockDB) {
				m.BeginTxFunc = func(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error) {
					return &MockTx{
						QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
							// Check owner
							if strings.Contains(sql, "FROM playlists") {
								return &MockRow{ScanFunc: func(dest ...any) error {
									*dest[0].(*string) = "pl-1"
									*dest[1].(*string) = "owner-1" // Different owner
									*dest[2].(*string) = "Old"
									*dest[3].(*string) = "Desc"
									*dest[4].(*bool) = true
									*dest[5].(*string) = "everyone"
									*dest[6].(*time.Time) = time.Now()
									return nil
								}}
							}
							return &MockRow{}
						},
						RollbackFunc: func(ctx context.Context) error { return nil },
					}, nil
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
			r.Patch("/playlists/{id}", srv.handlePatchPlaylist)

			var bodyBytes []byte
			if tt.body != nil {
				bodyBytes, _ = json.Marshal(tt.body)
			} else {
				bodyBytes = []byte("invalid-json")
			}
			req := httptest.NewRequest("PATCH", fmt.Sprintf("/playlists/%s", tt.playlistID), bytes.NewReader(bodyBytes))
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
