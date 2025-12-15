package vote

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

func (s *HTTPServer) handleListEvents(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-Id")
	ctx := r.Context()

	var rows pgx.Rows
	var err error
	if userID == "" {
		rows, err = s.pool.Query(ctx, `
            SELECT id, name, visibility, owner_id, license_mode,
                   geo_lat, geo_lng, geo_radius_m, vote_start, vote_end,
                   created_at, updated_at
            FROM events
            WHERE visibility = $1
            ORDER BY created_at DESC
        `, visibilityPublic)
	} else {
		rows, err = s.pool.Query(ctx, `
            SELECT DISTINCT e.id, e.name, e.visibility, e.owner_id, e.license_mode,
                   e.geo_lat, e.geo_lng, e.geo_radius_m, e.vote_start, e.vote_end,
                   e.created_at, e.updated_at
            FROM events e
            LEFT JOIN event_invites i
              ON i.event_id = e.id AND i.user_id = $1
            WHERE e.visibility = $2
               OR e.owner_id = $1
               OR i.user_id IS NOT NULL
            ORDER BY e.created_at DESC
        `, userID, visibilityPublic)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	events := make([]Event, 0)
	for rows.Next() {
		var ev Event
		var geoLat, geoLng *float64
		var geoRadius *int
		var voteStart, voteEnd *time.Time
		if err := rows.Scan(
			&ev.ID, &ev.Name, &ev.Visibility, &ev.OwnerID, &ev.LicenseMode,
			&geoLat, &geoLng, &geoRadius, &voteStart, &voteEnd,
			&ev.CreatedAt, &ev.UpdatedAt,
		); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		ev.GeoLat = geoLat
		ev.GeoLng = geoLng
		ev.GeoRadiusM = geoRadius
		ev.VoteStart = voteStart
		ev.VoteEnd = voteEnd
		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
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
		LicenseMode string   `json:"licenseMode"`
		GeoLat      *float64 `json:"geoLat"`
		GeoLng      *float64 `json:"geoLng"`
		GeoRadiusM  *int     `json:"geoRadiusM"`
		VoteStart   *string  `json:"voteStart"`
		VoteEnd     *string  `json:"voteEnd"`
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
	}
	if body.LicenseMode == "" {
		body.LicenseMode = licenseEveryone
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
	if body.Visibility == visibilityPrivate {
		plReq["isPublic"] = false
		plReq["editMode"] = "invited" // match event logic roughly
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
	respPL, err := http.DefaultClient.Do(reqPL)
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
	ctx := r.Context()
	var id string
	err = s.pool.QueryRow(ctx, `
        INSERT INTO events (id, name, visibility, owner_id, license_mode, geo_lat, geo_lng, geo_radius_m, vote_start, vote_end)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
        RETURNING id
    `, plResp.ID, body.Name, body.Visibility, userID, body.LicenseMode, body.GeoLat, body.GeoLng, body.GeoRadiusM, voteStart, voteEnd).Scan(&id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Fetch the full event (including timestamps) to return to client
	fullEvent, err := loadEvent(ctx, s.pool, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load created event: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, fullEvent)
}

func (s *HTTPServer) handleGetEvent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := r.Header.Get("X-User-Id")

	ev, err := loadEvent(r.Context(), s.pool, id)
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
			invited, err := isInvited(r.Context(), s.pool, ev.ID, userID)
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

	writeJSON(w, http.StatusOK, ev)
}

func (s *HTTPServer) handleDeleteEvent(w http.ResponseWriter, r *http.Request) {
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

	if _, err := s.pool.Exec(r.Context(), `DELETE FROM events WHERE id=$1`, id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *HTTPServer) handlePatchEvent(w http.ResponseWriter, r *http.Request) {
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

	var body struct {
		Name        *string  `json:"name"`
		Visibility  *string  `json:"visibility"`
		LicenseMode *string  `json:"licenseMode"`
		GeoLat      *float64 `json:"geoLat"`
		GeoLng      *float64 `json:"geoLng"`
		GeoRadiusM  *int     `json:"geoRadiusM"`
		VoteStart   *string  `json:"voteStart"`
		VoteEnd     *string  `json:"voteEnd"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	newStart := ev.VoteStart
	newEnd := ev.VoteEnd

	setParts := []string{}
	args := []any{}
	idxArg := 1

	if body.Name != nil && *body.Name != "" {
		setParts = append(setParts, "name = $"+itoa(idxArg))
		args = append(args, *body.Name)
		idxArg++
		ev.Name = *body.Name
	}

	if body.Visibility != nil && *body.Visibility != "" {
		setParts = append(setParts, "visibility = $"+itoa(idxArg))
		args = append(args, *body.Visibility)
		idxArg++
		ev.Visibility = *body.Visibility
	}
	if body.LicenseMode != nil && *body.LicenseMode != "" {
		setParts = append(setParts, "license_mode = $"+itoa(idxArg))
		args = append(args, *body.LicenseMode)
		idxArg++
		ev.LicenseMode = *body.LicenseMode
	}
	if body.GeoLat != nil {
		setParts = append(setParts, "geo_lat = $"+itoa(idxArg))
		args = append(args, *body.GeoLat)
		idxArg++
		ev.GeoLat = body.GeoLat
	}
	if body.GeoLng != nil {
		setParts = append(setParts, "geo_lng = $"+itoa(idxArg))
		args = append(args, *body.GeoLng)
		idxArg++
		ev.GeoLng = body.GeoLng
	}
	if body.GeoRadiusM != nil {
		setParts = append(setParts, "geo_radius_m = $"+itoa(idxArg))
		args = append(args, *body.GeoRadiusM)
		idxArg++
		ev.GeoRadiusM = body.GeoRadiusM
	}
	if body.VoteStart != nil {
		if *body.VoteStart == "" {
			setParts = append(setParts, "vote_start = NULL")
			newStart = nil
		} else {
			t, err := time.Parse(time.RFC3339, *body.VoteStart)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid voteStart")
				return
			}
			setParts = append(setParts, "vote_start = $"+itoa(idxArg))
			args = append(args, t)
			idxArg++
			newStart = &t
		}
	}
	if body.VoteEnd != nil {
		if *body.VoteEnd == "" {
			setParts = append(setParts, "vote_end = NULL")
			newEnd = nil
		} else {
			t, err := time.Parse(time.RFC3339, *body.VoteEnd)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid voteEnd")
				return
			}
			setParts = append(setParts, "vote_end = $"+itoa(idxArg))
			args = append(args, t)
			idxArg++
			newEnd = &t
		}
	}

	if err := validateVotingWindow(newStart, newEnd, time.Now()); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if len(setParts) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	args = append(args, id)
	query := "UPDATE events SET " + join(setParts, ", ") + ", updated_at = now() WHERE id = $" + itoa(idxArg)
	ct, err := s.pool.Exec(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if ct.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "event not found")
		return
	}

	updated, err := loadEvent(r.Context(), s.pool, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, updated)
}
