package main

import (
	"log"
	"net/http"
	"strings"
)

func (s *Server) handleSearchUsers(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("query"))
	if q == "" {
		writeError(w, http.StatusBadRequest, "query is required")
		return
	}
	if len(q) > 50 {
		writeError(w, http.StatusBadRequest, "query too long")
		return
	}

	pattern := "%" + q + "%"

	rows, err := s.db.Query(r.Context(), `
      SELECT id, user_id, display_name, username,
             avatar_url, has_custom_avatar,
             bio,
             visibility, preferences, is_premium,
             created_at, updated_at
      FROM user_profiles
      WHERE LOWER(username) LIKE LOWER($1)
         OR LOWER(display_name) LIKE LOWER($1)
      ORDER BY username
      LIMIT 20
  `, pattern)
	if err != nil {
		log.Printf("user-service: search users: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	var res []FriendItem
	for rows.Next() {
		p, err := scanUserProfile(rows)
		if err != nil {
			log.Printf("user-service: scan search user: %v", err)
			continue
		}
		res = append(res, FriendItem{
			UserID:      p.UserID,
			Username:    p.Username,
			DisplayName: p.DisplayName,
			AvatarURL:   resolveAvatarForViewer(p, false, false),
		})
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": res,
	})
}
