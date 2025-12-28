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
		writeError(w, http.StatusUnauthorized, "missing user context")
		return
	}
	playlistID := chi.URLParam(r, "id")
	if playlistID == "" {
		writeError(w, http.StatusBadRequest, "missing playlist id")
		return
	}

	ownerID, isPublic, _, err := s.getPlaylistAccessInfo(ctx, playlistID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "playlist not found")
		return
	}
	if err != nil {
		log.Printf("playlist-service: list invites fetch playlist: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Access Rule: Owner OR Public Playlist OR User is Invited
	// For public playlists, anyone can see the participant list (requirements: "list of participants... on which one can click").
	allowed := false
	if userID == ownerID {
		allowed = true
	} else if isPublic {
		allowed = true
	} else {
		// Check if user is already a member/invited
		isMember, err := s.userIsInvited(ctx, playlistID, userID)
		if err != nil {
			log.Printf("playlist-service: list invites check member: %v", err)
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
		if isMember {
			allowed = true
		}
	}

	if !allowed {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	rows, err := s.db.Query(ctx, `
		SELECT user_id, created_at
		FROM playlist_members
		WHERE playlist_id = $1
		ORDER BY created_at ASC
	`, playlistID)
	if err != nil {
		log.Printf("playlist-service: list invites query: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	invites := []PlaylistInvite{}
	for rows.Next() {
		var inv PlaylistInvite
		if err := rows.Scan(&inv.UserID, &inv.CreatedAt); err != nil {
			log.Printf("playlist-service: list invites scan: %v", err)
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
		invites = append(invites, inv)
	}
	if err := rows.Err(); err != nil {
		log.Printf("playlist-service: list invites rows: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	writeJSON(w, http.StatusOK, invites)
}

func (s *Server) handleAddInvite(w http.ResponseWriter, r *http.Request) {
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
		UserID string `json:"userId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	body.UserID = strings.TrimSpace(body.UserID)
	if body.UserID == "" {
		writeError(w, http.StatusBadRequest, "userId is required")
		return
	}

	ownerID, isPublic, _, err := s.getPlaylistAccessInfo(ctx, playlistID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "playlist not found")
		return
	}
	if err != nil {
		log.Printf("playlist-service: add invite fetch playlist: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Permission logic:
	// 1. Owner can invite anyone.
	// 2. Anyone can invite THEMSELVES (join) if the playlist is Public.
	allowed := false
	if userID == ownerID {
		allowed = true
	} else if isPublic && body.UserID == userID {
		allowed = true
	}

	if !allowed {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	_, err = s.db.Exec(ctx, `
		INSERT INTO playlist_members (playlist_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (playlist_id, user_id) DO NOTHING
	`, playlistID, body.UserID)
	if err != nil {
		log.Printf("playlist-service: add invite insert: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
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
		writeError(w, http.StatusUnauthorized, "missing user context")
		return
	}
	playlistID := chi.URLParam(r, "id")
	targetUserID := chi.URLParam(r, "userId")
	if playlistID == "" || targetUserID == "" {
		writeError(w, http.StatusBadRequest, "missing playlist id or user id")
		return
	}

	ownerID, _, _, err := s.getPlaylistAccessInfo(ctx, playlistID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "playlist not found")
		return
	}
	if err != nil {
		log.Printf("playlist-service: delete invite fetch playlist: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if userID != ownerID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	if _, err := s.db.Exec(ctx, `
		DELETE FROM playlist_members
		WHERE playlist_id = $1 AND user_id = $2
	`, playlistID, targetUserID); err != nil {
		log.Printf("playlist-service: delete invite: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
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
