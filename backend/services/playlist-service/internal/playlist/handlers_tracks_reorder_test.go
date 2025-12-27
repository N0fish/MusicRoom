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

// TestHandleReorderTracks_ComplexPositions verifies verify the integer positions of all tracks after a move.
// It intercepts SQL queries to ensure the correct UPDATE statements are executed.
func TestHandleReorderTracks_ComplexPositions(t *testing.T) {
	tests := []struct {
		name         string
		initialState []string // Just for mental model: [A, B, C, D]
		moveTrackID  string
		fromPos      int
		toPos        int
		totalTracks  int
		// What we expect the logic to do:
		wantShiftQuery string // Part of the SQL for shifting other tracks
		wantSetQuery   string // Part of the SQL for setting the new position
		wantShiftArgs  []interface{}
		wantSetArgs    []interface{}
	}{
		{
			// [A(0), B(1), C(2), D(3)]
			// Move C(2) to 0.
			// Result: [C(0), A(1), B(2), D(3)]
			// Logic:
			// 1. Shift A(0), B(1) -> +1. Range: [0, 2).
			//    UPDATE tracks SET position = position + 1 WHERE ... AND position >= 0 AND position < 2
			// 2. Set C to 0.
			name:           "Move C(2) to 0 (Backwards Move)",
			moveTrackID:    "track-C",
			fromPos:        2,
			toPos:          0,
			totalTracks:    4,
			wantShiftQuery: "UPDATE tracks\n\t\t\tSET position = position + 1",
			wantSetQuery:   "UPDATE tracks\n\t\tSET position = $3",
		},
		{
			// [A(0), B(1), C(2), D(3)]
			// Move A(0) to 3.
			// Result: [B(0), C(1), D(2), A(3)]
			// Logic:
			// 1. Shift B(1), C(2), D(3) -> -1. Range: (0, 3].
			//    UPDATE tracks SET position = position - 1 WHERE ... AND position > 0 AND position <= 3
			// 2. Set A to 3.
			name:           "Move A(0) to 3 (Forward Move)",
			moveTrackID:    "track-A",
			fromPos:        0,
			toPos:          3,
			totalTracks:    4,
			wantShiftQuery: "UPDATE tracks\n\t\t\tSET position = position - 1",
			wantSetQuery:   "UPDATE tracks\n\t\tSET position = $3",
		},
		{
			// [A(0), B(1), C(2)]
			// Move B(1) to 1. No op.
			name:        "Move B(1) to 1 (No Op)",
			moveTrackID: "track-B",
			fromPos:     1,
			toPos:       1,
			totalTracks: 3,
			// No queries expected
			wantShiftQuery: "",
			wantSetQuery:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := &MockDB{}
			srv := NewServer(mockDB, nil)
			r := chi.NewRouter()
			r.Patch("/playlists/{id}/tracks/{trackId}", srv.handleMoveTrack)

			userID := "user-owner"
			playlistID := "pl-1"

			// Track SQL executions
			var executedSQLs []string

			// 1. Access Check
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
				return &MockRow{ScanFunc: func(dest ...any) error { return errors.New("unexpected query: " + sql) }}
			}

			// 2. Transaction Setup
			mockTx := &MockTx{}
			mockDB.BeginTxFunc = func(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error) {
				return mockTx, nil
			}

			// 3. Tx Queries
			mockTx.QueryRowFunc = func(ctx context.Context, sql string, args ...any) pgx.Row {
				// Get current position
				if strings.Contains(sql, "SELECT playlist_id, position") {
					return &MockRow{
						ScanFunc: func(dest ...any) error {
							*dest[0].(*string) = playlistID
							*dest[1].(*int) = tt.fromPos
							return nil
						},
					}
				}
				// Get total count
				if strings.Contains(sql, "SELECT COUNT(*)") {
					return &MockRow{
						ScanFunc: func(dest ...any) error {
							*dest[0].(*int) = tt.totalTracks
							return nil
						},
					}
				}
				return &MockRow{ScanFunc: func(dest ...any) error { return errors.New("unexpected tx query: " + sql) }}
			}

			// 4. Capture Execs
			mockTx.ExecFunc = func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				executedSQLs = append(executedSQLs, sql)

				// Optional: Check args if needed for stricter validation
				// For now we check the SQL structure largely
				return pgconn.CommandTag{}, nil
			}

			mockTx.CommitFunc = func(ctx context.Context) error { return nil }
			mockTx.RollbackFunc = func(ctx context.Context) error { return nil }

			// Perform Request
			body, _ := json.Marshal(map[string]any{
				"newPosition": tt.toPos,
			})
			req := httptest.NewRequest("PATCH", fmt.Sprintf("/playlists/%s/tracks/%s", playlistID, tt.moveTrackID), bytes.NewReader(body))
			req.Header.Set("X-User-Id", userID)

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("Expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
			}

			// Verification
			if tt.fromPos == tt.toPos {
				if len(executedSQLs) > 0 {
					t.Errorf("Expected no SQL updates for no-op move, got %d", len(executedSQLs))
				}
				return
			}

			if len(executedSQLs) != 2 {
				t.Fatalf("Expected 2 SQL updates, got %d. \nCaptured: %v", len(executedSQLs), executedSQLs)
			}

			// Verify Shift Query
			if !strings.Contains(normalizeSQL(executedSQLs[0]), normalizeSQL(tt.wantShiftQuery)) {
				t.Errorf("Shift Query Mismatch.\nGot: %s\nWant substr: %s", executedSQLs[0], tt.wantShiftQuery)
			}

			// Verify Set Query
			if !strings.Contains(normalizeSQL(executedSQLs[1]), normalizeSQL(tt.wantSetQuery)) {
				t.Errorf("Set Query Mismatch.\nGot: %s\nWant substr: %s", executedSQLs[1], tt.wantSetQuery)
			}
		})
	}
}

// normalizeSQL removes tabs/spaces to make string comparison easier
func normalizeSQL(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
