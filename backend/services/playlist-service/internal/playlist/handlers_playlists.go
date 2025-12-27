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

func (s *Server) handleListPlaylists(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := r.Header.Get("X-User-Id")

	// Query: public playlists OR playlists I own OR playlists I'm invited to
	rows, err := s.db.Query(ctx, `
		SELECT p.id, p.owner_id, p.name, p.description, p.is_public, p.edit_mode, p.created_at
		FROM playlists p
		LEFT JOIN playlist_members pm ON p.id = pm.playlist_id AND pm.user_id = $1
		WHERE p.is_public = TRUE
		   OR ($1 <> '' AND p.owner_id = $1)
		   OR ($1 <> '' AND pm.user_id IS NOT NULL)
		ORDER BY p.created_at DESC
		LIMIT 200
	`, userID)
	if err != nil {
		log.Printf("playlist-service: list playlists: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	playlists := []Playlist{}
	for rows.Next() {
		var pl Playlist
		if err := rows.Scan(
			&pl.ID,
			&pl.OwnerID,
			&pl.Name,
			&pl.Description,
			&pl.IsPublic,
			&pl.EditMode,
			&pl.CreatedAt,
		); err != nil {
			log.Printf("playlist-service: list playlists scan: %v", err)
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
		playlists = append(playlists, pl)
	}

	if err := rows.Err(); err != nil {
		log.Printf("playlist-service: list playlists rows: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	writeJSON(w, http.StatusOK, playlists)
}

// handleCreatePlaylist creates a new playlist owned by the current user.
func (s *Server) handleCreatePlaylist(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ownerID := r.Header.Get("X-User-Id")
	if ownerID == "" {
		writeError(w, http.StatusUnauthorized, "missing user context")
		return
	}

	var body struct {
		Name        string  `json:"name"`
		Description string  `json:"description"`
		IsPublic    *bool   `json:"isPublic"`
		EditMode    *string `json:"editMode"` // optional, default "everyone"
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	body.Name = strings.TrimSpace(body.Name)
	body.Description = strings.TrimSpace(body.Description)

	if body.Name == "" || len(body.Name) > 200 {
		writeError(w, http.StatusBadRequest, "name must be between 1 and 200 characters")
		return
	}
	if len(body.Description) > 1000 {
		writeError(w, http.StatusBadRequest, "description is too long")
		return
	}

	isPublic := true
	if body.IsPublic != nil {
		isPublic = *body.IsPublic
	}

	editMode := editModeEveryone
	if body.EditMode != nil {
		em := strings.ToLower(strings.TrimSpace(*body.EditMode))
		if em != editModeEveryone && em != editModeInvited {
			writeError(w, http.StatusBadRequest, `invalid editMode (must be "everyone" or "invited")`)
			return
		}
		editMode = em
	}

	var pl Playlist
	err := s.db.QueryRow(ctx, `
		INSERT INTO playlists (owner_id, name, description, is_public, edit_mode)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, owner_id, name, description, is_public, edit_mode, created_at
	`, ownerID, body.Name, body.Description, isPublic, editMode).Scan(
		&pl.ID,
		&pl.OwnerID,
		&pl.Name,
		&pl.Description,
		&pl.IsPublic,
		&pl.EditMode,
		&pl.CreatedAt,
	)
	if err != nil {
		log.Printf("playlist-service: create playlist: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Notify realtime-service (best-effort).
	event := map[string]any{
		"type": "playlist.created",
		"payload": map[string]any{
			"playlist": pl,
		},
	}
	s.publishEvent(ctx, event)

	writeJSON(w, http.StatusCreated, pl)
}

// handlePatchPlaylist updates playlist metadata and license. Only the owner can update.
func (s *Server) handlePatchPlaylist(w http.ResponseWriter, r *http.Request) {
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

	var body struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		IsPublic    *bool   `json:"isPublic"`
		EditMode    *string `json:"editMode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		log.Printf("playlist-service: begin tx: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer tx.Rollback(ctx)

	var existing Playlist
	err = tx.QueryRow(ctx, `
		SELECT id, owner_id, name, description, is_public, edit_mode, created_at
		FROM playlists
		WHERE id = $1
	`, playlistID).Scan(
		&existing.ID,
		&existing.OwnerID,
		&existing.Name,
		&existing.Description,
		&existing.IsPublic,
		&existing.EditMode,
		&existing.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "playlist not found")
		return
	}
	if err != nil {
		log.Printf("playlist-service: fetch playlist: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	if existing.OwnerID != userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	if body.Name != nil {
		name := strings.TrimSpace(*body.Name)
		if name == "" || len(name) > 200 {
			writeError(w, http.StatusBadRequest, "name must be between 1 and 200 characters")
			return
		}
		existing.Name = name
	}
	if body.Description != nil {
		desc := strings.TrimSpace(*body.Description)
		if len(desc) > 1000 {
			writeError(w, http.StatusBadRequest, "description is too long")
			return
		}
		existing.Description = desc
	}
	if body.IsPublic != nil {
		existing.IsPublic = *body.IsPublic
	}
	if body.EditMode != nil {
		em := strings.ToLower(strings.TrimSpace(*body.EditMode))
		if em != editModeEveryone && em != editModeInvited {
			writeError(w, http.StatusBadRequest, `invalid editMode (must be "everyone" or "invited")`)
			return
		}
		existing.EditMode = em
	}

	_, err = tx.Exec(ctx, `
		UPDATE playlists
		SET name = $2,
			description = $3,
			is_public = $4,
			edit_mode = $5
		WHERE id = $1
	`, existing.ID, existing.Name, existing.Description, existing.IsPublic, existing.EditMode)
	if err != nil {
		log.Printf("playlist-service: update playlist: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("playlist-service: commit tx: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	event := map[string]any{
		"type": "playlist.updated",
		"payload": map[string]any{
			"playlist": existing,
		},
	}
	s.publishEvent(ctx, event)

	writeJSON(w, http.StatusOK, existing)
}

func (s *Server) handleGetPlaylist(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := r.Header.Get("X-User-Id")
	playlistID := chi.URLParam(r, "id")
	if playlistID == "" {
		writeError(w, http.StatusBadRequest, "missing playlist id")
		return
	}

	var pl Playlist
	err := s.db.QueryRow(ctx, `
		SELECT id, owner_id, name, description, is_public, edit_mode, created_at, current_track_id, playing_started_at
		FROM playlists
		WHERE id = $1
	`, playlistID).Scan(
		&pl.ID,
		&pl.OwnerID,
		&pl.Name,
		&pl.Description,
		&pl.IsPublic,
		&pl.EditMode,
		&pl.CreatedAt,
		&pl.CurrentTrackID,
		&pl.PlayingStartedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "playlist not found")
		return
	}
	if err != nil {
		log.Printf("playlist-service: get playlist: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Visibility rule:
	//   - public playlists are visible to everyone;
	//   - private playlists are only visible to the owner or invited users.
	if !pl.IsPublic && userID != pl.OwnerID {
		invited, err := s.userIsInvited(ctx, playlistID, userID)
		if err != nil {
			log.Printf("playlist-service: get playlist invited check: %v", err)
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
		if !invited {
			writeError(w, http.StatusForbidden, "playlist is private")
			return
		}
	}

	rows, err := s.db.Query(ctx, `
    SELECT t.id, t.playlist_id, t.title, t.artist, t.position, t.created_at,
           t.provider, t.provider_track_id, t.thumbnail_url, t.duration_ms, t.vote_count, t.status,
           (tv.user_id IS NOT NULL) as is_voted
    FROM tracks t
    LEFT JOIN track_votes tv ON t.id = tv.track_id AND tv.user_id = $2
    WHERE t.playlist_id = $1
    ORDER BY t.position ASC
  `, playlistID, userID)
	if err != nil {
		log.Printf("playlist-service: list tracks: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	tracks := []Track{}
	for rows.Next() {
		var tr Track
		if err := rows.Scan(
			&tr.ID,
			&tr.PlaylistID,
			&tr.Title,
			&tr.Artist,
			&tr.Position,
			&tr.CreatedAt,
			&tr.Provider,
			&tr.ProviderTrackID,
			&tr.ThumbnailURL,
			&tr.DurationMs,
			&tr.VoteCount,
			&tr.Status,
			&tr.IsVoted,
		); err != nil {
			log.Printf("playlist-service: list tracks scan: %v", err)
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
		tracks = append(tracks, tr)
	}
	if err := rows.Err(); err != nil {
		log.Printf("playlist-service: list tracks rows: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	canEdit := (userID != "" && userID == pl.OwnerID)
	if !canEdit && userID != "" {
		if pl.EditMode == editModeEveryone {
			canEdit = true
		} else {
			invited, _ := s.userIsInvited(ctx, playlistID, userID)
			canEdit = invited
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"playlist": pl,
		"tracks":   tracks,
		"canEdit":  canEdit,
	})
}

// handleDeletePlaylist deletes a playlist. Only the owner can delete.
func (s *Server) handleDeletePlaylist(w http.ResponseWriter, r *http.Request) {
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

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		log.Printf("playlist-service: delete playlist begin tx: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer tx.Rollback(ctx)

	var ownerID string
	err = tx.QueryRow(ctx, "SELECT owner_id FROM playlists WHERE id = $1", playlistID).Scan(&ownerID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "playlist not found")
		return
	}
	if err != nil {
		log.Printf("playlist-service: delete playlist fetch: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	if ownerID != userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	_, err = tx.Exec(ctx, "DELETE FROM playlists WHERE id = $1", playlistID)
	if err != nil {
		log.Printf("playlist-service: delete playlist exec: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("playlist-service: delete playlist commit: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Notify realtime
	event := map[string]any{
		"type":    "playlist.deleted",
		"payload": map[string]any{"playlistId": playlistID},
	}
	s.publishEvent(ctx, event)

	w.WriteHeader(http.StatusNoContent)
}