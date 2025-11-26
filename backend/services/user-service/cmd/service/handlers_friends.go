package main

import (
	"log"
	"net/http"
	"strings"
	"user-service/cmd/service/utils"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleListFriends(w http.ResponseWriter, r *http.Request) {
	me, ok := userIDFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	rows, err := s.db.Query(r.Context(), `
      SELECT p.user_id, p.username, p.display_name,
             p.avatar_url, p.has_custom_avatar, p.visibility
      FROM user_friends f
      JOIN user_profiles p
        ON p.user_id = CASE
            WHEN f.user1_id = $1 THEN f.user2_id
            ELSE f.user1_id
          END
      WHERE f.user1_id = $1 OR f.user2_id = $1
      ORDER BY p.username
  `, me)
	if err != nil {
		log.Printf("user-service: list friends: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	var items []FriendItem
	for rows.Next() {
		var p UserProfile
		if err := rows.Scan(
			&p.UserID,
			&p.Username,
			&p.DisplayName,
			&p.AvatarURL,
			&p.HasCustomAvatar,
			&p.Visibility,
		); err != nil {
			log.Printf("user-service: scan friend: %v", err)
			continue
		}
		items = append(items, FriendItem{
			UserID:      p.UserID,
			Username:    p.Username,
			DisplayName: p.DisplayName,
			AvatarURL:   resolveAvatarForViewer(p, true, false),
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": items,
	})
}

func (s *Server) handleSendFriendRequest(w http.ResponseWriter, r *http.Request) {
	me, ok := userIDFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	target := strings.TrimSpace(chi.URLParam(r, "id"))
	if target == "" || target == me {
		writeError(w, http.StatusBadRequest, "invalid target user")
		return
	}
	if !utils.IsValidUUID(target) {
		writeError(w, http.StatusBadRequest, "invalid target user id")
		return
	}

	ctx := r.Context()

	exists, err := s.userExists(ctx, target)
	if err != nil {
		log.Printf("user-service: userExists: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "target user not found")
		return
	}

	already, err := s.areFriends(ctx, me, target)
	if err != nil {
		log.Printf("user-service: areFriends: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if already {
		writeError(w, http.StatusBadRequest, "already friends")
		return
	}

	var existingID string
	err = s.db.QueryRow(ctx, `
      SELECT id
      FROM friend_requests
      WHERE from_user_id = $1 AND to_user_id = $2 AND status = 'pending'
  `, target, me).Scan(&existingID)
	if err == nil && existingID != "" {
		_, err = s.db.Exec(ctx, `
        UPDATE friend_requests
        SET status = 'accepted', updated_at = now()
        WHERE id = $1
      `, existingID)
		if err != nil {
			log.Printf("user-service: accept reciprocal request: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if err := s.addFriends(ctx, me, target); err != nil {
			log.Printf("user-service: addFriends: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "accepted",
		})
		return
	}

	var resp FriendRequestResponse
	err = s.db.QueryRow(ctx, `
      INSERT INTO friend_requests (from_user_id, to_user_id, status)
      VALUES ($1, $2, 'pending')
      ON CONFLICT (from_user_id, to_user_id) WHERE status = 'pending'
      DO UPDATE SET updated_at = now()
      RETURNING id, from_user_id, to_user_id, status, created_at, updated_at
  `, me, target).Scan(
		&resp.ID,
		&resp.FromUserID,
		&resp.ToUserID,
		&resp.Status,
		&resp.CreatedAt,
		&resp.UpdatedAt,
	)
	if err != nil {
		log.Printf("user-service: send friend request: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAcceptFriendRequest(w http.ResponseWriter, r *http.Request) {
	me, ok := userIDFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	from := strings.TrimSpace(chi.URLParam(r, "id"))
	if from == "" || from == me {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	ctx := r.Context()
	var reqID string
	err := s.db.QueryRow(ctx, `
      SELECT id
      FROM friend_requests
      WHERE from_user_id = $1 AND to_user_id = $2 AND status = 'pending'
  `, from, me).Scan(&reqID)
	if err != nil {
		writeError(w, http.StatusNotFound, "no pending request")
		return
	}

	_, err = s.db.Exec(ctx, `
      UPDATE friend_requests
      SET status = 'accepted', updated_at = now()
      WHERE id = $1
  `, reqID)
	if err != nil {
		log.Printf("user-service: accept friend: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := s.addFriends(ctx, me, from); err != nil {
		log.Printf("user-service: addFriends: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "accepted",
	})
}

func (s *Server) handleRejectFriendRequest(w http.ResponseWriter, r *http.Request) {
	me, ok := userIDFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	from := strings.TrimSpace(chi.URLParam(r, "id"))
	if from == "" || from == me {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	ctx := r.Context()
	res, err := s.db.Exec(ctx, `
      UPDATE friend_requests
      SET status = 'rejected', updated_at = now()
      WHERE from_user_id = $1 AND to_user_id = $2 AND status = 'pending'
  `, from, me)
	if err != nil {
		log.Printf("user-service: reject friend: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	aff := res.RowsAffected()
	if aff == 0 {
		writeError(w, http.StatusNotFound, "no pending request")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "rejected",
	})
}

func (s *Server) handleRemoveFriend(w http.ResponseWriter, r *http.Request) {
	me, ok := userIDFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	friendID := strings.TrimSpace(chi.URLParam(r, "id"))
	if friendID == "" || friendID == me {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	if err := s.removeFriends(r.Context(), me, friendID); err != nil {
		log.Printf("user-service: removeFriends: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "removed",
	})
}
