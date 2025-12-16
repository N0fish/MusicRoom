package playlist

import (
	"errors"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// handleVoteTrack handles upvoting a track.
// POST /playlists/{id}/tracks/{trackId}/vote
func (s *Server) handleVoteTrack(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := r.Header.Get("X-User-Id")
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "missing user context")
		return
	}

	playlistID := chi.URLParam(r, "id")
	trackID := chi.URLParam(r, "trackId")
	if playlistID == "" || trackID == "" {
		writeError(w, http.StatusBadRequest, "missing playlist or track id")
		return
	}

	// 1. Check access
	ownerID, isPublic, _, err := s.getPlaylistAccessInfo(ctx, playlistID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "playlist not found")
		return
	}
	if err != nil {
		log.Printf("playlist-service: vote track fetch playlist: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	invited := false
	if userID != "" && userID != ownerID {
		invited, err = s.userIsInvited(ctx, playlistID, userID)
		if err != nil {
			log.Printf("playlist-service: vote track invited check: %v", err)
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
	}

	if !isPublic && userID != ownerID && !invited {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		log.Printf("playlist-service: vote track begin tx: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer tx.Rollback(ctx)

	// 2. Increment vote count
	var newVoteCount int
	var status string
	err = tx.QueryRow(ctx, `
		UPDATE tracks
		SET vote_count = vote_count + 1
		WHERE id = $1 AND playlist_id = $2
		RETURNING vote_count, status
	`, trackID, playlistID).Scan(&newVoteCount, &status)

	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "track not found")
		return
	}
	if err != nil {
		log.Printf("playlist-service: vote track update: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	if status == "queued" {
		// 3. Reorder tracks based on votes
		params := []any{playlistID}
		rows, err := tx.Query(ctx, `
			SELECT id, vote_count, created_at, position
			FROM tracks
			WHERE playlist_id = $1 AND status = 'queued'
			ORDER BY position ASC
			FOR UPDATE
		`, params...)
		if err != nil {
			log.Printf("playlist-service: vote track select all: %v", err)
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}

		type trackSortInfo struct {
			ID        string
			VoteCount int
			CreatedAt time.Time
			Position  int
		}

		var tracks []trackSortInfo
		for rows.Next() {
			var t trackSortInfo
			if err := rows.Scan(&t.ID, &t.VoteCount, &t.CreatedAt, &t.Position); err != nil {
				rows.Close()
				log.Printf("playlist-service: vote track scan ids: %v", err)
				writeError(w, http.StatusInternalServerError, "database error")
				return
			}
			tracks = append(tracks, t)
		}
		rows.Close()

		// Stable sort
		sort.SliceStable(tracks, func(i, j int) bool {
			if tracks[i].VoteCount != tracks[j].VoteCount {
				return tracks[i].VoteCount > tracks[j].VoteCount
			}
			return tracks[i].CreatedAt.Before(tracks[j].CreatedAt)
		})

		// Find start position (after any playing/played tracks)
		var startPos int = 0
		err = tx.QueryRow(ctx, `
			SELECT COALESCE(MAX(position) + 1, 0)
			FROM tracks
			WHERE playlist_id = $1 AND status != 'queued'
		`, playlistID).Scan(&startPos)
		if err != nil {
			log.Printf("playlist-service: vote track get start pos: %v", err)
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}

		// Update positions
		// Optimization: Only update if position changed?
		// But simpler to update all for correctness in this complex flow.
		// To avoid unique constraint violation, update to negative first?
		// Or since we have a list of all IDs, just update them.

		// NOTE: Updating position on same table with unique index in loop can fail.
		// Safe strategy:
		// 1. Set all target positions to (-position - 1000000)
		// 2. Set them to correct new position

		// Step 1: Temporary move out of way
		for _, t := range tracks {
			_, err := tx.Exec(ctx, `UPDATE tracks SET position = -1 * position - 1000000 WHERE id = $1`, t.ID)
			if err != nil {
				log.Printf("playlist-service: vote track temp move: %v", err)
				writeError(w, http.StatusInternalServerError, "database error")
				return
			}
		}

		// Step 2: Set correct order
		for i, t := range tracks {
			newPos := startPos + i
			_, err := tx.Exec(ctx, `UPDATE tracks SET position = $1 WHERE id = $2`, newPos, t.ID)
			if err != nil {
				log.Printf("playlist-service: vote track final move: %v", err)
				writeError(w, http.StatusInternalServerError, "database error")
				return
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("playlist-service: vote track commit: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	s.publishEvent(ctx, map[string]any{
		"type": "track.updated",
		"payload": map[string]any{
			"playlistId": playlistID,
			"trackId":    trackID,
			"voteCount":  newVoteCount,
		},
	})

	if status == "queued" {
		s.publishEvent(ctx, map[string]any{
			"type": "playlist.reordered",
			"payload": map[string]any{
				"playlistId": playlistID,
			},
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"voteCount": newVoteCount})
}
