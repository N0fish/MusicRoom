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

// ensure MockDB is recognized; assuming it lives in mocks_test.go in same package.

func TestHandleAddInvite_Success(t *testing.T) {
	mockDB := &MockDB{}
	srv := NewServer(mockDB, nil)
	r := chi.NewRouter()
	r.Post("/playlists/{id}/invites", srv.handleAddInvite)

	userID := "owner-123"
	playlistID := "pl-001"
	inviteeID := "user-456"

	// 1. getPlaylistAccessInfo
	mockDB.QueryRowFunc = func(ctx context.Context, sql string, args ...any) pgx.Row {
		return &MockRow{
			ScanFunc: func(dest ...any) error {
				// owner_id, is_public, edit_mode
				*dest[0].(*string) = userID
				*dest[1].(*bool) = false
				*dest[2].(*string) = "invited"
				return nil
			},
		}
	}

	// 2. Insert Invite
	mockDB.ExecFunc = func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
		if strings.Contains(sql, "INSERT INTO playlist_members") {
			return pgconn.CommandTag{}, nil
		}
		return pgconn.CommandTag{}, errors.New("unexpected exec")
	}

	body, _ := json.Marshal(map[string]string{"userId": inviteeID})
	req := httptest.NewRequest("POST", fmt.Sprintf("/playlists/%s/invites", playlistID), bytes.NewReader(body))
	req.Header.Set("X-User-Id", userID)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected 204 No Content, got %d", w.Code)
	}
}

func TestHandleListInvites_Success(t *testing.T) {
	mockDB := &MockDB{}
	srv := NewServer(mockDB, nil)
	r := chi.NewRouter()
	r.Get("/playlists/{id}/invites", srv.handleListInvites)

	userID := "owner-123"
	playlistID := "pl-001"

	// 1. Access info (Owner)
	mockDB.QueryRowFunc = func(ctx context.Context, sql string, args ...any) pgx.Row {
		return &MockRow{
			ScanFunc: func(dest ...any) error {
				*dest[0].(*string) = userID
				*dest[1].(*bool) = false
				*dest[2].(*string) = "invited"
				return nil
			},
		}
	}

	// 2. List Members
	mockDB.QueryFunc = func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
		if strings.Contains(sql, "FROM playlist_members") {
			return &MockRows{
				Data: [][]any{
					{"user-1", time.Now()},
					{"user-2", time.Now()},
				},
				Idx: -1,
			}, nil
		}
		return nil, errors.New("unexpected query: " + sql)
	}

	req := httptest.NewRequest("GET", fmt.Sprintf("/playlists/%s/invites", playlistID), nil)
	req.Header.Set("X-User-Id", userID)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", w.Code)
	}

	var invites []struct{ UserID string }
	json.NewDecoder(w.Body).Decode(&invites)
	if len(invites) != 2 {
		t.Errorf("Expected 2 invites, got %d", len(invites))
	}
}

func TestHandleDeleteInvite_Success(t *testing.T) {
	mockDB := &MockDB{}
	srv := NewServer(mockDB, nil)
	r := chi.NewRouter()
	r.Delete("/playlists/{id}/invites/{userId}", srv.handleDeleteInvite)

	userID := "owner-123"
	targetID := "user-999"
	playlistID := "pl-001"

	mockDB.QueryRowFunc = func(ctx context.Context, sql string, args ...any) pgx.Row {
		return &MockRow{ScanFunc: func(dest ...any) error {
			*dest[0].(*string) = userID
			*dest[1].(*bool) = false
			*dest[2].(*string) = "invited"
			return nil
		}}
	}

	mockDB.ExecFunc = func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
		if strings.Contains(sql, "DELETE FROM playlist_members") {
			return pgconn.CommandTag{}, nil
		}
		return pgconn.CommandTag{}, errors.New("unexpected exec")
	}

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/playlists/%s/invites/%s", playlistID, targetID), nil)
	req.Header.Set("X-User-Id", userID)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected 204 No Content, got %d", w.Code)
	}
}

func TestHandleAddInvite_Errors(t *testing.T) {
	tests := []struct {
		name       string
		playlistID string
		userID     string
		body       map[string]any
		mockSetup  func(*MockDB)
		wantCode   int
	}{
		{
			name:       "Missing User Context",
			playlistID: "pl-1",
			userID:     "",
			body:       map[string]any{"userId": "target-1"},
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
			name:       "Missing Target UserID",
			playlistID: "pl-1",
			userID:     "user-1",
			body:       map[string]any{"userId": ""},
			mockSetup:  func(m *MockDB) {},
			wantCode:   http.StatusBadRequest,
		},
		{
			name:       "Playlist Not Found",
			playlistID: "pl-missing",
			userID:     "user-1",
			body:       map[string]any{"userId": "target-1"},
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
			userID:     "outsider",
			body:       map[string]any{"userId": "target-1"},
			mockSetup: func(m *MockDB) {
				m.QueryRowFunc = func(ctx context.Context, sql string, args ...any) pgx.Row {
					return &MockRow{ScanFunc: func(dest ...any) error {
						*dest[0].(*string) = "owner"
						*dest[1].(*bool) = false
						*dest[2].(*string) = "everyone"
						return nil
					}}
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
			r.Post("/playlists/{id}/invites", srv.handleAddInvite)

			var bodyBytes []byte
			if tt.body != nil {
				bodyBytes, _ = json.Marshal(tt.body)
			} else {
				bodyBytes = []byte("invalid-json")
			}
			url := fmt.Sprintf("/playlists/%s/invites", tt.playlistID)
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

func TestHandleListInvites_Errors(t *testing.T) {
	tests := []struct {
		name       string
		playlistID string
		userID     string
		mockSetup  func(*MockDB)
		wantCode   int
	}{
		{
			name:       "Playlist Not Found",
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
			name:       "Forbidden",
			playlistID: "pl-priv",
			userID:     "outsider",
			mockSetup: func(m *MockDB) {
				m.QueryRowFunc = func(ctx context.Context, sql string, args ...any) pgx.Row {
					if strings.Contains(sql, "FROM playlists") {
						return &MockRow{ScanFunc: func(dest ...any) error {
							*dest[0].(*string) = "owner"
							*dest[1].(*bool) = false
							*dest[2].(*string) = "everyone"
							return nil
						}}
					}
					// check member
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
			r.Get("/playlists/{id}/invites", srv.handleListInvites)

			req := httptest.NewRequest("GET", fmt.Sprintf("/playlists/%s/invites", tt.playlistID), nil)
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
