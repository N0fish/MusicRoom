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

func (s *Server) handleListPublicPlaylists(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := s.db.Query(ctx, `
		SELECT id, owner_id, name, description, is_public, edit_mode, created_at
		FROM playlists
		WHERE is_public = TRUE
		ORDER BY created_at DESC
		LIMIT 200
	`)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: list playlists: %v", err)
		return
	}
	defer rows.Close()

	var playlists []Playlist
	for rows.Next() {
		var pl Playlist
		if err := rows.Scan(&pl.ID, &pl.OwnerID, &pl.Name, &pl.Description, &pl.IsPublic, &pl.EditMode, &pl.CreatedAt); err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			log.Printf("playlist-service: list playlists scan: %v", err)
			return
		}
		playlists = append(playlists, pl)
	}

	if rows.Err() != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: list playlists rows: %v", rows.Err())
		return
	}

	writeJSON(w, http.StatusOK, playlists)
}

// handleCreatePlaylist creates a new playlist owned by the current user.
func (s *Server) handleCreatePlaylist(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ownerID := r.Header.Get("X-User-Id")
	if ownerID == "" {
		http.Error(w, "missing user context", http.StatusUnauthorized)
		return
	}

	var body struct {
		Name        string  `json:"name"`
		Description string  `json:"description"`
		IsPublic    *bool   `json:"isPublic"`
		EditMode    *string `json:"editMode"` // optional, default "everyone"
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	body.Name = strings.TrimSpace(body.Name)
	body.Description = strings.TrimSpace(body.Description)

	if body.Name == "" || len(body.Name) > 200 {
		http.Error(w, "name must be between 1 and 200 characters", http.StatusBadRequest)
		return
	}
	if len(body.Description) > 1000 {
		http.Error(w, "description is too long", http.StatusBadRequest)
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
			http.Error(w, "invalid editMode (must be \"everyone\" or \"invited\")", http.StatusBadRequest)
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
		&pl.ID, &pl.OwnerID, &pl.Name, &pl.Description, &pl.IsPublic, &pl.EditMode, &pl.CreatedAt,
	)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: create playlist: %v", err)
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
		http.Error(w, "missing user context", http.StatusUnauthorized)
		return
	}

	playlistID := chi.URLParam(r, "id")
	if playlistID == "" {
		http.Error(w, "missing playlist id", http.StatusBadRequest)
		return
	}

	var body struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		IsPublic    *bool   `json:"isPublic"`
		EditMode    *string `json:"editMode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: begin tx: %v", err)
		return
	}
	defer tx.Rollback(ctx)

	var existing Playlist
	err = tx.QueryRow(ctx, `
		SELECT id, owner_id, name, description, is_public, edit_mode, created_at
		FROM playlists
		WHERE id = $1
	`, playlistID).Scan(
		&existing.ID, &existing.OwnerID, &existing.Name, &existing.Description, &existing.IsPublic, &existing.EditMode, &existing.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "playlist not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: fetch playlist: %v", err)
		return
	}

	if existing.OwnerID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if body.Name != nil {
		name := strings.TrimSpace(*body.Name)
		if name == "" || len(name) > 200 {
			http.Error(w, "name must be between 1 and 200 characters", http.StatusBadRequest)
			return
		}
		existing.Name = name
	}
	if body.Description != nil {
		desc := strings.TrimSpace(*body.Description)
		if len(desc) > 1000 {
			http.Error(w, "description is too long", http.StatusBadRequest)
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
			http.Error(w, "invalid editMode (must be \"everyone\" or \"invited\")", http.StatusBadRequest)
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
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: update playlist: %v", err)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: commit tx: %v", err)
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
		http.Error(w, "missing playlist id", http.StatusBadRequest)
		return
	}

	var pl Playlist
	err := s.db.QueryRow(ctx, `
		SELECT id, owner_id, name, description, is_public, edit_mode, created_at
		FROM playlists
		WHERE id = $1
	`, playlistID).Scan(
		&pl.ID, &pl.OwnerID, &pl.Name, &pl.Description, &pl.IsPublic, &pl.EditMode, &pl.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "playlist not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: get playlist: %v", err)
		return
	}

	// Visibility rule:
	//   - public playlists are visible to everyone;
	//   - private playlists are only visible to the owner or invited users.
	if !pl.IsPublic && userID != pl.OwnerID {
		invited, err := s.userIsInvited(ctx, playlistID, userID)
		if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			log.Printf("playlist-service: get playlist invited check: %v", err)
			return
		}
		if !invited {
			http.Error(w, "playlist is private", http.StatusForbidden)
			return
		}
	}

	rows, err := s.db.Query(ctx, `
		SELECT id, playlist_id, title, artist, position, created_at
		FROM tracks
		WHERE playlist_id = $1
		ORDER BY position ASC
	`, playlistID)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: list tracks: %v", err)
		return
	}
	defer rows.Close()

	var tracks []Track
	for rows.Next() {
		var tr Track
		if err := rows.Scan(&tr.ID, &tr.PlaylistID, &tr.Title, &tr.Artist, &tr.Position, &tr.CreatedAt); err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			log.Printf("playlist-service: list tracks scan: %v", err)
			return
		}
		tracks = append(tracks, tr)
	}
	if rows.Err() != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: list tracks rows: %v", rows.Err())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"playlist": pl,
		"tracks":   tracks,
	})
}
