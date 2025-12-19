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

func (s *HTTPServer) handleListEvents(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-Id")
	ctx := r.Context()

	var events []Event
	var err error
	if userID == "" {
		events, err = s.store.ListEvents(ctx, "", visibilityPublic)
	} else {
		events, err = s.store.ListEvents(ctx, userID, visibilityPublic)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, events)
}

func (s *HTTPServer) handleCreateEvent(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-Id")
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "missing X-User-Id")
		return
	}

	var body struct {
		Name        string   `json:"name"`
		Visibility  string   `json:"visibility"`
		LicenseMode string   `json:"license_mode"`
		GeoLat      *float64 `json:"geo_lat"`
		GeoLng      *float64 `json:"geo_lng"`
		GeoRadiusM  *int     `json:"geo_radius_m"`
		VoteStart   *string  `json:"vote_start"`
		VoteEnd     *string  `json:"vote_end"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if body.Visibility == "" {
		body.Visibility = visibilityPublic
	} else if body.Visibility != visibilityPublic && body.Visibility != visibilityPrivate {
		writeError(w, http.StatusBadRequest, "invalid visibility")
		return
	}

	if body.LicenseMode == "" {
		body.LicenseMode = licenseEveryone
	} else if body.LicenseMode != licenseEveryone && body.LicenseMode != licenseInvited && body.LicenseMode != licenseGeoTime {
		writeError(w, http.StatusBadRequest, "invalid license mode")
		return
	}

	var voteStart, voteEnd *time.Time
	if body.VoteStart != nil && *body.VoteStart != "" {
		t, err := time.Parse(time.RFC3339, *body.VoteStart)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid voteStart (must be RFC3339)")
			return
		}
		voteStart = &t
	}
	if body.VoteEnd != nil && *body.VoteEnd != "" {
		t, err := time.Parse(time.RFC3339, *body.VoteEnd)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid voteEnd (must be RFC3339)")
			return
		}
		voteEnd = &t
	}

	if err := validateVotingWindow(voteStart, voteEnd, time.Now()); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 1. Create Playlist in playlist-service to get a synchronized ID
	plReq := map[string]any{
		"name":        body.Name,
		"description": "Event Playlist for " + body.Name,
		"isPublic":    true, // Events are public by default logic here? Or match visibility?
		"editMode":    "everyone",
	}
	if body.Visibility == visibilityPrivate || body.LicenseMode == licenseInvited {
		plReq["isPublic"] = (body.Visibility == visibilityPublic)
		plReq["editMode"] = "invited" // license_invited or private visibility triggers invited edit mode
	}

	plBody, _ := json.Marshal(plReq)
	reqPL, err := http.NewRequestWithContext(r.Context(), http.MethodPost, s.playlistServiceURL+"/playlists", bytes.NewReader(plBody))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create request: "+err.Error())
		return
	}
	reqPL.Header.Set("Content-Type", "application/json")
	reqPL.Header.Set("X-User-Id", userID) // Pass through auth

	// We use a default http client here since we didn't inject one, assuming s.httpClient or http.DefaultClient
	// For now using http.DefaultClient but strictly we should add HttpClient to HTTPServer.
	// TIMEOUT is crucial but for MVP using DefaultClient
	respPL, err := s.httpClient.Do(reqPL)
	if err != nil {
		writeError(w, http.StatusBadGateway, "playlist-service unavailable: "+err.Error())
		return
	}
	defer respPL.Body.Close()

	if respPL.StatusCode != http.StatusCreated && respPL.StatusCode != http.StatusOK {
		writeError(w, http.StatusBadGateway, "playlist-service failed")
		return
	}

	var plResp struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(respPL.Body).Decode(&plResp); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to decode playlist response")
		return
	}

	// 2. Insert Event with the SAME ID
	// Prepare event object
	newEvent := &Event{
		ID:          plResp.ID,
		Name:        body.Name,
		Visibility:  body.Visibility,
		OwnerID:     userID,
		LicenseMode: body.LicenseMode,
		GeoLat:      body.GeoLat,
		GeoLng:      body.GeoLng,
		GeoRadiusM:  body.GeoRadiusM,
		VoteStart:   voteStart,
		VoteEnd:     voteEnd,
	}

	id, err := s.store.CreateEvent(r.Context(), newEvent)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Fetch the full event (including timestamps) to return to client
	fullEvent, err := s.store.LoadEvent(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load created event: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, fullEvent)
}

func (s *HTTPServer) handleGetEvent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := r.Header.Get("X-User-Id")

	ev, err := s.store.LoadEvent(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "event not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if ev.Visibility == visibilityPrivate {
		if userID == "" {
			writeError(w, http.StatusUnauthorized, "missing X-User-Id")
			return
		}

		if ev.OwnerID != userID {
			invited, err := s.store.IsInvited(r.Context(), ev.ID, userID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !invited {
				writeError(w, http.StatusForbidden, "event is private, invite required")
				return
			}
		}
	}

	if userID != "" {
		if ev.OwnerID == userID {
			ev.IsJoined = true
		} else {
			invited, err := s.store.IsInvited(r.Context(), ev.ID, userID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			ev.IsJoined = invited
		}
	}

	writeJSON(w, http.StatusOK, ev)
}

func (s *HTTPServer) handleDeleteEvent(w http.ResponseWriter, r *http.Request) {
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
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	if err := s.store.DeleteEvent(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	go s.publishEvent(context.Background(), "event.deleted", map[string]string{"id": id})

	w.WriteHeader(http.StatusNoContent)
}

func (s *HTTPServer) handlePatchEvent(w http.ResponseWriter, r *http.Request) {
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
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	var body struct {
		Name        *string  `json:"name"`
		Visibility  *string  `json:"visibility"`
		LicenseMode *string  `json:"license_mode"`
		GeoLat      *float64 `json:"geo_lat"`
		GeoLng      *float64 `json:"geo_lng"`
		GeoRadiusM  *int     `json:"geo_radius_m"`
		VoteStart   *string  `json:"vote_start"`
		VoteEnd     *string  `json:"vote_end"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	newStart := ev.VoteStart
	newEnd := ev.VoteEnd

	updates := make(map[string]any)
	if body.Name != nil && *body.Name != "" {
		updates["name"] = *body.Name
		ev.Name = *body.Name
	}
	if body.Visibility != nil && *body.Visibility != "" {
		if *body.Visibility != visibilityPublic && *body.Visibility != visibilityPrivate {
			writeError(w, http.StatusBadRequest, "invalid visibility")
			return
		}
		updates["visibility"] = *body.Visibility
		ev.Visibility = *body.Visibility
	}
	if body.LicenseMode != nil && *body.LicenseMode != "" {
		if *body.LicenseMode != licenseEveryone && *body.LicenseMode != licenseInvited && *body.LicenseMode != licenseGeoTime {
			writeError(w, http.StatusBadRequest, "invalid license mode")
			return
		}
		updates["license_mode"] = *body.LicenseMode
		ev.LicenseMode = *body.LicenseMode
	}
	if body.GeoLat != nil {
		updates["geo_lat"] = *body.GeoLat
		ev.GeoLat = body.GeoLat
	}
	if body.GeoLng != nil {
		updates["geo_lng"] = *body.GeoLng
		ev.GeoLng = body.GeoLng
	}
	if body.GeoRadiusM != nil {
		updates["geo_radius_m"] = *body.GeoRadiusM
		ev.GeoRadiusM = body.GeoRadiusM
	}
	if body.VoteStart != nil {
		if *body.VoteStart == "" {
			updates["vote_start"] = nil
			newStart = nil
		} else {
			t, err := time.Parse(time.RFC3339, *body.VoteStart)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid voteStart")
				return
			}
			updates["vote_start"] = t
			newStart = &t
		}
	}
	if body.VoteEnd != nil {
		if *body.VoteEnd == "" {
			updates["vote_end"] = nil
			newEnd = nil
		} else {
			t, err := time.Parse(time.RFC3339, *body.VoteEnd)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid voteEnd")
				return
			}
			updates["vote_end"] = t
			newEnd = &t
		}
	}

	if err := validateVotingWindow(newStart, newEnd, time.Now()); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Propagate changes to playlist-service if name, visibility or license_mode changed
	if body.Name != nil || body.Visibility != nil || body.LicenseMode != nil {
		plUpdate := make(map[string]any)
		if body.Name != nil {
			plUpdate["name"] = *body.Name
		}
		if body.Visibility != nil {
			plUpdate["isPublic"] = (*body.Visibility == visibilityPublic)
		}
		if body.LicenseMode != nil || body.Visibility != nil {
			// Recalculate editMode
			vis := ev.Visibility
			if body.Visibility != nil {
				vis = *body.Visibility
			}
			lic := ev.LicenseMode
			if body.LicenseMode != nil {
				lic = *body.LicenseMode
			}

			if vis == visibilityPrivate || lic == licenseInvited {
				plUpdate["editMode"] = "invited"
			} else {
				plUpdate["editMode"] = "everyone"
			}
		}

		if s.httpClient != nil {
			go func() {
				plBody, _ := json.Marshal(plUpdate)
				req, err := http.NewRequest(http.MethodPatch, s.playlistServiceURL+"/playlists/"+id, bytes.NewReader(plBody))
				if err == nil {
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("X-User-Id", userID)
					resp, err := s.httpClient.Do(req)
					if err != nil {
						log.Printf("vote-service: failed to propagate patch to playlist-service: %v", err)
					} else {
						resp.Body.Close()
					}
				}
			}()
		}
	}

	if len(updates) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := s.store.UpdateEvent(r.Context(), id, updates); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "event not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// ct check removed as UpdateEvent handles no rows error

	updated, err := s.store.LoadEvent(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

func (s *HTTPServer) handleTransferOwnership(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := r.Header.Get("X-User-Id")
	log.Printf("DEBUG: handleTransferOwnership id=%s userID=%s", id, userID)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "missing X-User-Id")
		return
	}

	var body struct {
		NewOwnerID string `json:"newOwnerId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.NewOwnerID == "" {
		writeError(w, http.StatusBadRequest, "newOwnerId is required")
		return
	}

	// 1. Load event and verify current ownership
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
		writeError(w, http.StatusForbidden, "only owner can transfer ownership")
		return
	}

	if ev.OwnerID == body.NewOwnerID {
		writeError(w, http.StatusBadRequest, "already owner")
		return
	}

	// 2. Perform Transfer
	// ideally we check if new owner exists or is valid user, but we trust the ID for now
	// or we check if they are a participant (optional strictness)

	// Update event owner
	if err := s.store.TransferOwnership(r.Context(), id, body.NewOwnerID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update owner: "+err.Error())
		return
	}

	// Ensure old owner is now a participant (invited)
	if err := s.store.CreateInvite(r.Context(), id, userID); err != nil {
		// Log error but don't fail the request as transfer already happened
		log.Printf("ERROR: failed to add old owner %s as participant: %v", userID, err)
	}

	// 3. Notify updates
	// Publish event updated message
	go s.publishEvent(context.Background(), "event.updated", map[string]string{"id": id})
	// Also specifically notify about ownership change if we had a specific event type,
	// but "event.updated" should trigger re-fetch on clients.

	w.WriteHeader(http.StatusOK)
}
