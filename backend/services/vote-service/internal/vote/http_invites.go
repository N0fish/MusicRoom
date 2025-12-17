package vote

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type Invite struct {
	UserID    string    `json:"userId"`
	CreatedAt time.Time `json:"createdAt"`
}

func (s *HTTPServer) handleCreateInvite(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := r.Header.Get("X-User-Id")
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "missing X-User-Id")
		return
	}

	var body struct {
		UserID string `json:"userId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.UserID == "" {
		writeError(w, http.StatusBadRequest, "userId is required")
		return
	}

	ev, err := loadEvent(r.Context(), s.pool, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "event not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if ev.OwnerID != userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	if err := checkUserExists(r.Context(), s.userServiceURL, body.UserID); err != nil {
		var ie *inviteError
		if errors.As(err, &ie) {
			writeError(w, ie.status, ie.msg)
			return
		}
		log.Printf("vote-service: check user exists: %v", err)
		writeError(w, http.StatusBadGateway, "unable to verify user")
		return
	}

	if _, err := s.pool.Exec(r.Context(), `
        INSERT INTO event_invites(event_id, user_id)
        VALUES($1,$2) ON CONFLICT DO NOTHING
    `, id, body.UserID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Propagate to playlist-service for Realtime events
	// Ignore errors here? Or fail? Best to log and ignore to not break logic if data is improved,
	// BUT for realtime sync it's critical.
	// We'll log error but return success since DB write succeeded.
	go func() {
		// Use a detached context or similar since request context might be cancelled
		// For simplicity using background context with timeout
		// (In prod, use proper queue/worker)
		// Construct request
		plReqBody, _ := json.Marshal(map[string]string{"userId": body.UserID})
		req, err := http.NewRequest(http.MethodPost, s.playlistServiceURL+"/playlists/"+id+"/invites", bytes.NewReader(plReqBody))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-User-Id", userID)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				log.Printf("vote-service: failed to propagate invite to playlist-service: %v", err)
			} else {
				resp.Body.Close()
			}
		}
	}()

	w.WriteHeader(http.StatusNoContent)
}

func (s *HTTPServer) handleDeleteInvite(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := r.Header.Get("X-User-Id")
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "missing X-User-Id")
		return
	}

	ev, err := loadEvent(r.Context(), s.pool, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "event not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	invitedID := chi.URLParam(r, "userId")
	if ev.OwnerID != userID && userID != invitedID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if _, err := s.pool.Exec(r.Context(), `DELETE FROM event_invites WHERE event_id=$1 AND user_id=$2`, id, invitedID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Propagate to playlist-service
	go func() {
		req, err := http.NewRequest(http.MethodDelete, s.playlistServiceURL+"/playlists/"+id+"/invites/"+invitedID, nil)
		if err == nil {
			req.Header.Set("X-User-Id", userID)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				log.Printf("vote-service: failed to propagate delete invite to playlist-service: %v", err)
			} else {
				resp.Body.Close()
			}
		}
	}()

	w.WriteHeader(http.StatusNoContent)
}

func (s *HTTPServer) handleListInvites(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := r.Header.Get("X-User-Id")
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "missing X-User-Id")
		return
	}

	ev, err := loadEvent(r.Context(), s.pool, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "event not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if ev.OwnerID != userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	rows, err := s.pool.Query(r.Context(), `SELECT user_id, created_at FROM event_invites WHERE event_id=$1 ORDER BY created_at`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var invites []Invite
	for rows.Next() {
		var inv Invite
		if err := rows.Scan(&inv.UserID, &inv.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		invites = append(invites, inv)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, invites)
}
