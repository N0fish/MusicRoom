package vote

import (
	"bytes"
	"context"
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

	ev, err := s.store.LoadEvent(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "event not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var role string

	if ev.OwnerID != userID {
		// Strict check for Invited Only
		if ev.LicenseMode == licenseInvited {
			// If InvitedOnly, ONLY Owner can invite contributors.
			// BUT, Public events allow listeners to join.
			// If Self-Join + Public + InvitedOnly -> Guest
			if ev.Visibility == visibilityPublic && body.UserID == userID {
				role = RoleGuest
			} else {
				// Stranger trying to invite someone or join private?
				// Private check is handled by visibility usually?
				// Actually handleCreateInvite: "if ev.OwnerID != userID" ...
				// If not owner, you can only self-join public events.
				// (The previous logic allowed this).
				if ev.Visibility == visibilityPublic && body.UserID == userID {
					// Logic above handles this branch, but let's be explicit
					// This block is redundant if we nested cleanly, but let's follow logic.
				} else {
					writeError(w, http.StatusForbidden, "cannot join invited-only event without invite")
					return
				}
			}
		} else {
			// licenseEveryone or GeoTime
			// Allow self-invite for public events (Joining)
			if ev.Visibility == visibilityPublic && body.UserID == userID {
				role = RoleContributor
			} else {
				writeError(w, http.StatusForbidden, "forbidden")
				return
			}
		}
	} else {
		// Owner is inviting
		role = RoleContributor
	}

	if err := checkUserExists(r.Context(), s.httpClient, s.userServiceURL, body.UserID); err != nil {
		var ie *inviteError
		if errors.As(err, &ie) {
			writeError(w, ie.status, ie.msg)
			return
		}
		log.Printf("vote-service: check user exists: %v", err)
		writeError(w, http.StatusBadGateway, "unable to verify user")
		return
	}

	if err := s.store.CreateInvite(r.Context(), id, body.UserID, role); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Emit event.invited directly to ensure robust realtime delivery
	go s.publishEvent(context.Background(), "event.invited", map[string]any{
		"eventId": id,
		"userId":  body.UserID,
	})

	// Propagate to playlist-service for Realtime events (kept for backward compat or other services)
	go func() {
		// Use a background context with timeout for propagation
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		plReqBody, _ := json.Marshal(map[string]string{"userId": body.UserID})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.playlistServiceURL+"/playlists/"+id+"/invites", bytes.NewReader(plReqBody))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-User-Id", userID)
			resp, err := s.httpClient.Do(req)
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

	ev, err := s.store.LoadEvent(r.Context(), id)
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
	if err := s.store.DeleteInvite(r.Context(), id, invitedID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Propagate to playlist-service
	go func() {
		req, err := http.NewRequest(http.MethodDelete, s.playlistServiceURL+"/playlists/"+id+"/invites/"+invitedID, nil)
		if err == nil {
			req.Header.Set("X-User-Id", userID)
			resp, err := s.httpClient.Do(req)
			if err != nil {
				log.Printf("vote-service: failed to propagate delete invite to playlist-service: %v", err)
			} else {
				resp.Body.Close()
			}
		}
	}()

	go s.publishEvent(context.Background(), "event.left", map[string]string{
		"eventId": id,
		"userId":  invitedID,
	})

	w.WriteHeader(http.StatusNoContent)
}

func (s *HTTPServer) handleListInvites(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := r.Header.Get("X-User-Id")
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "missing X-User-Id")
		return
	}

	ev, err := s.store.LoadEvent(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "event not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if ev.OwnerID != userID {
		invited, err := s.store.IsInvited(r.Context(), id, userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !invited {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
	}

	invites, err := s.store.ListInvites(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, invites)
}
