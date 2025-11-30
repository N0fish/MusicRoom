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

func (s *Server) handleListInvites(w http.ResponseWriter, r *http.Request) {
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

	ownerID, _, _, err := s.getPlaylistAccessInfo(ctx, playlistID)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "playlist not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: list invites fetch playlist: %v", err)
		return
	}

	if userID != ownerID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	rows, err := s.db.Query(ctx, `
		SELECT user_id, created_at
		FROM playlist_members
		WHERE playlist_id = $1
		ORDER BY created_at ASC
	`, playlistID)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: list invites query: %v", err)
		return
	}
	defer rows.Close()

	var invites []PlaylistInvite
	for rows.Next() {
		var inv PlaylistInvite
		if err := rows.Scan(&inv.UserID, &inv.CreatedAt); err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			log.Printf("playlist-service: list invites scan: %v", err)
			return
		}
		invites = append(invites, inv)
	}
	if rows.Err() != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: list invites rows: %v", rows.Err())
		return
	}

	writeJSON(w, http.StatusOK, invites)
}

func (s *Server) handleAddInvite(w http.ResponseWriter, r *http.Request) {
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
		UserID string `json:"userId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	body.UserID = strings.TrimSpace(body.UserID)
	if body.UserID == "" {
		http.Error(w, "userId is required", http.StatusBadRequest)
		return
	}

	ownerID, _, _, err := s.getPlaylistAccessInfo(ctx, playlistID)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "playlist not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: add invite fetch playlist: %v", err)
		return
	}
	if userID != ownerID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	_, err = s.db.Exec(ctx, `
		INSERT INTO playlist_members (playlist_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (playlist_id, user_id) DO NOTHING
	`, playlistID, body.UserID)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: add invite insert: %v", err)
		return
	}

	event := map[string]any{
		"type": "playlist.invited",
		"payload": map[string]any{
			"playlistId": playlistID,
			"userId":     body.UserID,
		},
	}
	s.publishEvent(ctx, event)

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteInvite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := r.Header.Get("X-User-Id")
	if userID == "" {
		http.Error(w, "missing user context", http.StatusUnauthorized)
		return
	}
	playlistID := chi.URLParam(r, "id")
	targetUserID := chi.URLParam(r, "userId")
	if playlistID == "" || targetUserID == "" {
		http.Error(w, "missing playlist id or user id", http.StatusBadRequest)
		return
	}

	ownerID, _, _, err := s.getPlaylistAccessInfo(ctx, playlistID)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "playlist not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: delete invite fetch playlist: %v", err)
		return
	}
	if userID != ownerID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if _, err := s.db.Exec(ctx, `
		DELETE FROM playlist_members
		WHERE playlist_id = $1 AND user_id = $2
	`, playlistID, targetUserID); err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		log.Printf("playlist-service: delete invite: %v", err)
		return
	}

	event := map[string]any{
		"type": "playlist.invite_removed",
		"payload": map[string]any{
			"playlistId": playlistID,
			"userId":     targetUserID,
		},
	}
	s.publishEvent(ctx, event)

	w.WriteHeader(http.StatusNoContent)
}
