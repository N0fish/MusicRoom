package playlist

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// handleNextTrack skips to the next track in the queue.
// POST /playlists/{id}/next
// NextTrack advances the playlist to the next track.
// This is used by the HTTP handler and the background ticker.
func (s *Server) NextTrack(ctx context.Context, playlistID string) (map[string]any, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		log.Printf("playlist-service: next track begin tx: %v", err)
		return nil, err
	}
	defer tx.Rollback(ctx)

	// 1. Get current state
	var currentTrackID *string
	err = tx.QueryRow(ctx, `SELECT current_track_id FROM playlists WHERE id = $1 FOR UPDATE`, playlistID).Scan(&currentTrackID)
	if err != nil {
		log.Printf("playlist-service: next track get current: %v", err)
		return nil, err
	}

	// 2. Update old track to 'played'
	if currentTrackID != nil {
		_, err = tx.Exec(ctx, `UPDATE tracks SET status = 'played' WHERE id = $1`, *currentTrackID)
		if err != nil {
			log.Printf("playlist-service: next track update old: %v", err)
			return nil, err
		}
	}

	// 3. Find next 'queued' track
	var nextTrackID string
	var nextTrackDurationMs int
	err = tx.QueryRow(ctx, `
		SELECT id, duration_ms
		FROM tracks
		WHERE playlist_id = $1 AND status = 'queued'
		ORDER BY position ASC
		LIMIT 1
		FOR UPDATE
	`, playlistID).Scan(&nextTrackID, &nextTrackDurationMs)

	updatedState := map[string]any{
		"playlistId":       playlistID,
		"currentTrackId":   nil,
		"playingStartedAt": nil,
		"status":           "stopped",
	}

	if errors.Is(err, pgx.ErrNoRows) {
		// End of playlist
		_, err = tx.Exec(ctx, `
			UPDATE playlists 
			SET current_track_id = NULL, playing_started_at = NULL 
			WHERE id = $1
		`, playlistID)
		if err != nil {
			log.Printf("playlist-service: next track clear playlist: %v", err)
			return nil, err
		}
	} else if err != nil {
		log.Printf("playlist-service: next track find next: %v", err)
		return nil, err
	} else {
		// Found next track
		now := time.Now()
		_, err = tx.Exec(ctx, `
			UPDATE tracks SET status = 'playing' WHERE id = $1
		`, nextTrackID)
		if err != nil {
			log.Printf("playlist-service: next track set playing: %v", err)
			return nil, err
		}

		_, err = tx.Exec(ctx, `
			UPDATE playlists 
			SET current_track_id = $2, playing_started_at = $3 
			WHERE id = $1
		`, playlistID, nextTrackID, now)
		if err != nil {
			log.Printf("playlist-service: next track update playlist: %v", err)
			return nil, err
		}

		updatedState["currentTrackId"] = nextTrackID
		updatedState["playingStartedAt"] = now
		updatedState["status"] = "playing"
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("playlist-service: next track commit: %v", err)
		return nil, err
	}

	s.publishEvent(ctx, map[string]any{
		"type":    "player.state_changed",
		"payload": updatedState,
	})

	return updatedState, nil
}

// handleNextTrack skips to the next track in the queue.
// POST /playlists/{id}/next
func (s *Server) handleNextTrack(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := r.Header.Get("X-User-Id")
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "missing user context")
		return
	}

	playlistID := chi.URLParam(r, "id")
	if playlistID == "" {
		writeError(w, http.StatusBadRequest, "missing playlist id")
		return
	}

	// 1. Check access (HTTP only)
	ownerID, isPublic, editMode, err := s.getPlaylistAccessInfo(ctx, playlistID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "playlist not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	invited := false
	if userID != "" && userID != ownerID {
		invited, err = s.userIsInvited(ctx, playlistID, userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
	}

	if !isPublic && userID != ownerID && !invited {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	allowControl := false
	if userID == ownerID {
		allowControl = true
	} else if editMode == editModeEveryone {
		allowControl = true
	} else if editMode == editModeInvited && invited {
		allowControl = true
	}

	if !allowControl {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	// 2. Invoke reuseable logic
	updatedState, err := s.NextTrack(ctx, playlistID)
	if err != nil {
		// Assuming logged inside NextTrack
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, updatedState)
}
