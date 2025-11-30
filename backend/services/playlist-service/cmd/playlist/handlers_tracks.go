package playlist

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

func (s *Server) handleAddTrack(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := r.Header.Get("X-User-Id")
	if userID == "" {
		http.Error(w, "missing user context", http.StatusUnauthorized)
		return
	}

	playlistID := chi.URLParam(r, "id")
	if playlistID == "" {
		http.Error(w, "missing playlist id", http.StatusBadRequest)
		return
	}

	ownerID, isPublic, editMode, err := s.getPlaylistAccessInfo(ctx, playlistID)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "playlist not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: add track fetch playlist: %v", err)
		return
	}

	invited := false
	if userID != "" && userID != ownerID {
		invited, err = s.userIsInvited(ctx, playlistID, userID)
		if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			log.Printf("playlist-service: add track invited check: %v", err)
			return
		}
	}

	// Private playlist: only owner or invited can access at all.
	if !isPublic && userID != ownerID && !invited {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// License for edit rights.
	if userID != ownerID {
		if editMode == editModeInvited && !invited {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		// editModeEveryone: any authenticated user with access is allowed.
	}

	var body struct {
		Title  string `json:"title"`
		Artist string `json:"artist"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	body.Title = strings.TrimSpace(body.Title)
	body.Artist = strings.TrimSpace(body.Artist)
	if body.Title == "" || len(body.Title) > 300 {
		http.Error(w, "title must be between 1 and 300 characters", http.StatusBadRequest)
		return
	}
	if len(body.Artist) > 200 {
		http.Error(w, "artist is too long", http.StatusBadRequest)
		return
	}

	var tr Track
	err = s.db.QueryRow(ctx, `
		INSERT INTO tracks (playlist_id, title, artist, position)
		VALUES ($1,$2,$3, COALESCE(
			(SELECT MAX(position)+1 FROM tracks WHERE playlist_id = $1),
			0
		))
		RETURNING id, playlist_id, title, artist, position, created_at
	`, playlistID, body.Title, body.Artist).Scan(
		&tr.ID, &tr.PlaylistID, &tr.Title, &tr.Artist, &tr.Position, &tr.CreatedAt,
	)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: add track insert: %v", err)
		return
	}

	event := map[string]any{
		"type": "track.added",
		"payload": map[string]any{
			"playlistId": playlistID,
			"track":      tr,
		},
	}
	s.publishEvent(ctx, event)

	writeJSON(w, http.StatusCreated, tr)
}

// handleMoveTrack reorders a track within its playlist (concurrency-sensitive).
func (s *Server) handleMoveTrack(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := r.Header.Get("X-User-Id")
	if userID == "" {
		http.Error(w, "missing user context", http.StatusUnauthorized)
		return
	}

	playlistID := chi.URLParam(r, "id")
	trackID := chi.URLParam(r, "trackId")
	if playlistID == "" || trackID == "" {
		http.Error(w, "missing playlist or track id", http.StatusBadRequest)
		return
	}

	var body struct {
		NewPosition int `json:"newPosition"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if body.NewPosition < 0 {
		http.Error(w, "newPosition must be >= 0", http.StatusBadRequest)
		return
	}

	ownerID, isPublic, editMode, err := s.getPlaylistAccessInfo(ctx, playlistID)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "playlist not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: move track fetch playlist: %v", err)
		return
	}

	invited := false
	if userID != "" && userID != ownerID {
		invited, err = s.userIsInvited(ctx, playlistID, userID)
		if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			log.Printf("playlist-service: move track invited check: %v", err)
			return
		}
	}

	if !isPublic && userID != ownerID && !invited {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if userID != ownerID {
		if editMode == editModeInvited && !invited {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: move track begin tx: %v", err)
		return
	}
	defer tx.Rollback(ctx)

	var currentPos int
	var trackPlaylistID string
	err = tx.QueryRow(ctx, `
		SELECT playlist_id, position
		FROM tracks
		WHERE id = $1 AND playlist_id = $2
		FOR UPDATE
	`, trackID, playlistID).Scan(&trackPlaylistID, &currentPos)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "track not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: move track fetch track: %v", err)
		return
	}

	var total int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM tracks WHERE playlist_id = $1
	`, playlistID).Scan(&total); err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: move track count: %v", err)
		return
	}
	if total <= 0 {
		http.Error(w, "no tracks to move", http.StatusConflict)
		return
	}

	newPos := body.NewPosition
	if newPos >= total {
		newPos = total - 1
	}
	if newPos == currentPos {
		if err := tx.Commit(ctx); err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			log.Printf("playlist-service: move track commit noop: %v", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"trackId": trackID,
			"from":    currentPos,
			"to":      newPos,
		})
		return
	}

	if newPos > currentPos {
		_, err = tx.Exec(ctx, `
			UPDATE tracks
			SET position = position - 1
			WHERE playlist_id = $1
			  AND position > $2
			  AND position <= $3
		`, playlistID, currentPos, newPos)
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE tracks
			SET position = position + 1
			WHERE playlist_id = $1
			  AND position >= $3
			  AND position < $2
		`, playlistID, currentPos, newPos)
	}
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: move track shift: %v", err)
		return
	}

	_, err = tx.Exec(ctx, `
		UPDATE tracks
		SET position = $3
		WHERE id = $2 AND playlist_id = $1
	`, playlistID, trackID, newPos)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: move track set position: %v", err)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: move track commit: %v", err)
		return
	}

	event := map[string]any{
		"type": "track.moved",
		"payload": map[string]any{
			"playlistId": playlistID,
			"trackId":    trackID,
			"from":       currentPos,
			"to":         newPos,
		},
	}
	s.publishEvent(ctx, event)

	writeJSON(w, http.StatusOK, map[string]any{
		"trackId": trackID,
		"from":    currentPos,
		"to":      newPos,
	})
}

func (s *Server) handleDeleteTrack(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := r.Header.Get("X-User-Id")
	if userID == "" {
		http.Error(w, "missing user context", http.StatusUnauthorized)
		return
	}

	playlistID := chi.URLParam(r, "id")
	trackID := chi.URLParam(r, "trackId")
	if playlistID == "" || trackID == "" {
		http.Error(w, "missing playlist or track id", http.StatusBadRequest)
		return
	}

	ownerID, isPublic, editMode, err := s.getPlaylistAccessInfo(ctx, playlistID)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "playlist not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: delete track fetch playlist: %v", err)
		return
	}

	invited := false
	if userID != "" && userID != ownerID {
		invited, err = s.userIsInvited(ctx, playlistID, userID)
		if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			log.Printf("playlist-service: delete track invited check: %v", err)
			return
		}
	}

	if !isPublic && userID != ownerID && !invited {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if userID != ownerID {
		if editMode == editModeInvited && !invited {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: delete track begin tx: %v", err)
		return
	}
	defer tx.Rollback(ctx)

	var pos int
	err = tx.QueryRow(ctx, `
		SELECT position
		FROM tracks
		WHERE id = $1 AND playlist_id = $2
		FOR UPDATE
	`, trackID, playlistID).Scan(&pos)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "track not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: delete track fetch: %v", err)
		return
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM tracks
		WHERE id = $1 AND playlist_id = $2
	`, trackID, playlistID); err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: delete track delete: %v", err)
		return
	}

	if _, err := tx.Exec(ctx, `
		UPDATE tracks
		SET position = position - 1
		WHERE playlist_id = $1 AND position > $2
	`, playlistID, pos); err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: delete track compact: %v", err)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: delete track commit: %v", err)
		return
	}

	event := map[string]any{
		"type": "track.deleted",
		"payload": map[string]any{
			"playlistId": playlistID,
			"trackId":    trackID,
			"position":   pos,
		},
	}
	s.publishEvent(ctx, event)

	w.WriteHeader(http.StatusNoContent)
}
