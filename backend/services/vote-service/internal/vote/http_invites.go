package vote

import (
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
		http.Error(w, "missing X-User-Id", http.StatusUnauthorized)
		return
	}

	var body struct {
		UserID string `json:"userId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if body.UserID == "" {
		http.Error(w, "userId is required", http.StatusBadRequest)
		return
	}

	ev, err := loadEvent(r.Context(), s.pool, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "event not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if ev.OwnerID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := checkUserExists(r.Context(), s.userServiceURL, body.UserID); err != nil {
		var ie *inviteError
		if errors.As(err, &ie) {
			http.Error(w, ie.msg, ie.status)
			return
		}
		log.Printf("vote-service: check user exists: %v", err)
		http.Error(w, "unable to verify user", http.StatusBadGateway)
		return
	}

	if _, err := s.pool.Exec(r.Context(), `
        INSERT INTO event_invites(event_id, user_id)
        VALUES($1,$2) ON CONFLICT DO NOTHING
    `, id, body.UserID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *HTTPServer) handleDeleteInvite(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := r.Header.Get("X-User-Id")
	if userID == "" {
		http.Error(w, "missing X-User-Id", http.StatusUnauthorized)
		return
	}

	ev, err := loadEvent(r.Context(), s.pool, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "event not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if ev.OwnerID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	invitedID := chi.URLParam(r, "userId")
	if _, err := s.pool.Exec(r.Context(), `DELETE FROM event_invites WHERE event_id=$1 AND user_id=$2`, id, invitedID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *HTTPServer) handleListInvites(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := r.Header.Get("X-User-Id")
	if userID == "" {
		http.Error(w, "missing X-User-Id", http.StatusUnauthorized)
		return
	}

	ev, err := loadEvent(r.Context(), s.pool, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "event not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if ev.OwnerID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	rows, err := s.pool.Query(r.Context(), `SELECT user_id, created_at FROM event_invites WHERE event_id=$1 ORDER BY created_at`, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var invites []Invite
	for rows.Next() {
		var inv Invite
		if err := rows.Scan(&inv.UserID, &inv.CreatedAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		invites = append(invites, inv)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(invites)
}
