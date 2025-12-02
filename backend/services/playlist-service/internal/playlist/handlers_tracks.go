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
		writeError(w, http.StatusUnauthorized, "missing user context")
		return
	}

	playlistID := chi.URLParam(r, "id")
	if playlistID == "" {
		writeError(w, http.StatusBadRequest, "missing playlist id")
		return
	}

	ownerID, isPublic, editMode, err := s.getPlaylistAccessInfo(ctx, playlistID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "playlist not found")
		return
	}
	if err != nil {
		log.Printf("playlist-service: add track fetch playlist: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	invited := false
	if userID != "" && userID != ownerID {
		invited, err = s.userIsInvited(ctx, playlistID, userID)
		if err != nil {
			log.Printf("playlist-service: add track invited check: %v", err)
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
	}

	// Private playlist: only owner or invited can access at all.
	if !isPublic && userID != ownerID && !invited {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	// License for edit rights.
	if userID != ownerID {
		if editMode == editModeInvited && !invited {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		// editModeEveryone: any authenticated user with access is allowed.
	}

	var body struct {
		Title         string `json:"title"`
		Artist        string `json:"artist"`
		Provider      string `json:"provider"`
		ProviderTrack string `json:"providerTrackId"`
		ThumbnailURL  string `json:"thumbnailUrl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	body.Title = strings.TrimSpace(body.Title)
	body.Artist = strings.TrimSpace(body.Artist)
	body.Provider = strings.TrimSpace(strings.ToLower(body.Provider))
	body.ProviderTrack = strings.TrimSpace(body.ProviderTrack)
	body.ThumbnailURL = strings.TrimSpace(body.ThumbnailURL)

	if body.Title == "" || len(body.Title) > 300 {
		writeError(w, http.StatusBadRequest, "title must be between 1 and 300 characters")
		return
	}
	if len(body.Artist) > 200 {
		writeError(w, http.StatusBadRequest, "artist is too long")
		return
	}

	if body.Provider != "" {
		if body.Provider != "youtube" {
			writeError(w, http.StatusBadRequest, "unsupported provider (only \"youtube\" is allowed)")
			return
		}
		if body.ProviderTrack == "" {
			writeError(w, http.StatusBadRequest, "providerTrackId is required when provider is set")
			return
		}
	}

	var tr Track
	err = s.db.QueryRow(ctx, `
      INSERT INTO tracks (
          playlist_id,
          title,
          artist,
          position,
          provider,
          provider_track_id,
          thumbnail_url
      )
      VALUES (
          $1, $2, $3,
          COALESCE(
            (SELECT MAX(position)+1 FROM tracks WHERE playlist_id = $1),
            0
          ),
          $4, $5, $6
      )
      RETURNING id, playlist_id, title, artist, position, created_at,
                provider, provider_track_id, thumbnail_url
  `,
		playlistID,
		body.Title,
		body.Artist,
		body.Provider,
		body.ProviderTrack,
		body.ThumbnailURL,
	).Scan(
		&tr.ID,
		&tr.PlaylistID,
		&tr.Title,
		&tr.Artist,
		&tr.Position,
		&tr.CreatedAt,
		&tr.Provider,
		&tr.ProviderTrackID,
		&tr.ThumbnailURL,
	)
	if err != nil {
		log.Printf("playlist-service: add track insert: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
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
		writeError(w, http.StatusUnauthorized, "missing user context")
		return
	}

	playlistID := chi.URLParam(r, "id")
	trackID := chi.URLParam(r, "trackId")
	if playlistID == "" || trackID == "" {
		writeError(w, http.StatusBadRequest, "missing playlist or track id")
		return
	}

	var body struct {
		NewPosition int `json:"newPosition"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.NewPosition < 0 {
		writeError(w, http.StatusBadRequest, "newPosition must be >= 0")
		return
	}

	ownerID, isPublic, editMode, err := s.getPlaylistAccessInfo(ctx, playlistID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "playlist not found")
		return
	}
	if err != nil {
		log.Printf("playlist-service: move track fetch playlist: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	invited := false
	if userID != "" && userID != ownerID {
		invited, err = s.userIsInvited(ctx, playlistID, userID)
		if err != nil {
			log.Printf("playlist-service: move track invited check: %v", err)
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
	}

	if !isPublic && userID != ownerID && !invited {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if userID != ownerID {
		if editMode == editModeInvited && !invited {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		log.Printf("playlist-service: move track begin tx: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
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
		writeError(w, http.StatusNotFound, "track not found")
		return
	}
	if err != nil {
		log.Printf("playlist-service: move track fetch track: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	var total int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM tracks WHERE playlist_id = $1
	`, playlistID).Scan(&total); err != nil {
		log.Printf("playlist-service: move track count: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if total <= 0 {
		writeError(w, http.StatusConflict, "no tracks to move")
		return
	}

	newPos := body.NewPosition
	if newPos >= total {
		newPos = total - 1
	}
	if newPos == currentPos {
		if err := tx.Commit(ctx); err != nil {
			log.Printf("playlist-service: move track commit noop: %v", err)
			writeError(w, http.StatusInternalServerError, "database error")
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
		log.Printf("playlist-service: move track shift: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	_, err = tx.Exec(ctx, `
		UPDATE tracks
		SET position = $3
		WHERE id = $2 AND playlist_id = $1
	`, playlistID, trackID, newPos)
	if err != nil {
		log.Printf("playlist-service: move track set position: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("playlist-service: move track commit: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
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
		writeError(w, http.StatusUnauthorized, "missing user context")
		return
	}

	playlistID := chi.URLParam(r, "id")
	trackID := chi.URLParam(r, "trackId")
	if playlistID == "" || trackID == "" {
		writeError(w, http.StatusBadRequest, "missing playlist or track id")
		return
	}

	ownerID, isPublic, editMode, err := s.getPlaylistAccessInfo(ctx, playlistID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "playlist not found")
		return
	}
	if err != nil {
		log.Printf("playlist-service: delete track fetch playlist: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	invited := false
	if userID != "" && userID != ownerID {
		invited, err = s.userIsInvited(ctx, playlistID, userID)
		if err != nil {
			log.Printf("playlist-service: delete track invited check: %v", err)
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
	}

	if !isPublic && userID != ownerID && !invited {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if userID != ownerID {
		if editMode == editModeInvited && !invited {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		log.Printf("playlist-service: delete track begin tx: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
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
		writeError(w, http.StatusNotFound, "track not found")
		return
	}
	if err != nil {
		log.Printf("playlist-service: delete track fetch: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM tracks
		WHERE id = $1 AND playlist_id = $2
	`, trackID, playlistID); err != nil {
		log.Printf("playlist-service: delete track delete: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	if _, err := tx.Exec(ctx, `
		UPDATE tracks
		SET position = position - 1
		WHERE playlist_id = $1 AND position > $2
	`, playlistID, pos); err != nil {
		log.Printf("playlist-service: delete track compact: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("playlist-service: delete track commit: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
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
