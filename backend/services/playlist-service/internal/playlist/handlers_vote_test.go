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

func TestHandleVoteTrack_Success(t *testing.T) {
	mockDB := &MockDB{}
	srv := NewServer(mockDB, nil)
	r := chi.NewRouter()
	r.Post("/playlists/{id}/tracks/{trackId}/vote", srv.handleVoteTrack)

	userID := "user-1"
	playlistID := "pl-1"
	trackID := "tr-1"

	// 1. Access Check
	mockDB.QueryRowFunc = func(ctx context.Context, sql string, args ...any) pgx.Row {
		if strings.Contains(sql, "FROM playlists") {
			return &MockRow{ScanFunc: func(dest ...any) error {
				*dest[0].(*string) = "user-1" // Owner matches request user
				*dest[1].(*bool) = true       // public
				*dest[2].(*string) = "everyone"
				return nil
			}}
		}
		return &MockRow{ScanFunc: func(dest ...any) error { return errors.New("unexpected query") }}
	}

	// 2. Tx for Vote
	mockDB.BeginTxFunc = func(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error) {
		return &MockTx{
			QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
				// Check existing vote
				if strings.Contains(sql, "FROM track_votes") {
					return &MockRow{ScanFunc: func(dest ...any) error {
						return pgx.ErrNoRows // No existing vote
					}}
				}
				// Get track playlist_id
				if strings.Contains(sql, "playlist_id FROM tracks") {
					return &MockRow{ScanFunc: func(dest ...any) error {
						*dest[0].(*string) = playlistID
						return nil
					}}
				}
				return &MockRow{}
			},
			ExecFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				// Insert vote, Update track score
				return pgconn.CommandTag{}, nil
			},
			CommitFunc: func(ctx context.Context) error { return nil },
		}, nil
	}

	body, _ := json.Marshal(map[string]any{"value": 1})
	req := httptest.NewRequest("POST", fmt.Sprintf("/playlists/%s/tracks/%s/vote", playlistID, trackID), bytes.NewReader(body))
	req.Header.Set("X-User-Id", userID)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestHandleVoteTrack_Errors(t *testing.T) {
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
			playlistID: "pl-missing",
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
		{
			name:       "Already Voted (Conflict)",
			playlistID: "pl-1",
			trackID:    "tr-1",
			userID:     "user-1",
			mockSetup: func(m *MockDB) {
				m.QueryRowFunc = func(ctx context.Context, sql string, args ...any) pgx.Row {
					if strings.Contains(sql, "FROM playlists") {
						return &MockRow{ScanFunc: func(dest ...any) error {
							*dest[0].(*string) = "user-1" // Owner
							*dest[1].(*bool) = true
							*dest[2].(*string) = "everyone"
							return nil
						}}
					}
					// Return current vote count after conflict
					if strings.Contains(sql, "SELECT vote_count") {
						return &MockRow{ScanFunc: func(dest ...any) error {
							*dest[0].(*int) = 5
							return nil
						}}
					}
					return &MockRow{}
				}
				m.BeginTxFunc = func(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error) {
					return &MockTx{
						ExecFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
							// Simulate Unique Violation
							return pgconn.CommandTag{}, &pgconn.PgError{Code: "23505"}
						},
						RollbackFunc: func(ctx context.Context) error { return nil },
						QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
							// For the vote count fetch inside handler error path
							return &MockRow{ScanFunc: func(dest ...any) error {
								*dest[0].(*int) = 5
								return nil
							}}
						},
					}, nil
				}
			},
			wantCode: http.StatusConflict,
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
			r.Post("/playlists/{id}/tracks/{trackId}/vote", srv.handleVoteTrack)

			req := httptest.NewRequest("POST", fmt.Sprintf("/playlists/%s/tracks/%s/vote", tt.playlistID, tt.trackID), nil)
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
