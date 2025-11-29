package vote

import (
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		ev.GeoLat = geoLat
		ev.GeoLng = geoLng
		ev.GeoRadiusM = geoRadius
		ev.VoteStart = voteStart
		ev.VoteEnd = voteEnd
		events = append(events, ev)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(events)
}

func (s *HTTPServer) handleCreateEvent(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-Id")
	if userID == "" {
		http.Error(w, "missing X-User-Id", http.StatusUnauthorized)
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
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if body.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
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
			http.Error(w, "invalid voteStart (must be RFC3339)", http.StatusBadRequest)
			return
		}
		voteStart = &t
	}
	if body.VoteEnd != nil && *body.VoteEnd != "" {
		t, err := time.Parse(time.RFC3339, *body.VoteEnd)
		if err != nil {
			http.Error(w, "invalid voteEnd (must be RFC3339)", http.StatusBadRequest)
			return
		}
		voteEnd = &t
	}

	if err := validateVotingWindow(voteStart, voteEnd, time.Now()); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	var id string
	err := s.pool.QueryRow(ctx, `
        INSERT INTO events (name, visibility, owner_id, license_mode, geo_lat, geo_lng, geo_radius_m, vote_start, vote_end)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
        RETURNING id
    `, body.Name, body.Visibility, userID, body.LicenseMode, body.GeoLat, body.GeoLng, body.GeoRadiusM, voteStart, voteEnd).Scan(&id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	evt := map[string]any{
		"type": "event.created",
		"payload": map[string]any{
			"id":          id,
			"name":        body.Name,
			"visibility":  body.Visibility,
			"ownerId":     userID,
			"licenseMode": body.LicenseMode,
		},
	}
	if b, err := json.Marshal(evt); err == nil {
		_ = s.rdb.Publish(ctx, "broadcast", string(b)).Err()
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":          id,
		"name":        body.Name,
		"visibility":  body.Visibility,
		"ownerId":     userID,
		"licenseMode": body.LicenseMode,
	})
}

func (s *HTTPServer) handleGetEvent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := r.Header.Get("X-User-Id")

	ev, err := loadEvent(r.Context(), s.pool, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "event not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if ev.Visibility == visibilityPrivate {
		if userID == "" {
			http.Error(w, "missing X-User-Id", http.StatusUnauthorized)
			return
		}

		if ev.OwnerID != userID {
			invited, err := isInvited(r.Context(), s.pool, ev.ID, userID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if !invited {
				http.Error(w, "event is private, invite required", http.StatusForbidden)
				return
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ev)
}

func (s *HTTPServer) handleDeleteEvent(w http.ResponseWriter, r *http.Request) {
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

	if _, err := s.pool.Exec(r.Context(), `DELETE FROM events WHERE id=$1`, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *HTTPServer) handlePatchEvent(w http.ResponseWriter, r *http.Request) {
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

	var body struct {
		Visibility  *string  `json:"visibility"`
		LicenseMode *string  `json:"licenseMode"`
		GeoLat      *float64 `json:"geoLat"`
		GeoLng      *float64 `json:"geoLng"`
		GeoRadiusM  *int     `json:"geoRadiusM"`
		VoteStart   *string  `json:"voteStart"`
		VoteEnd     *string  `json:"voteEnd"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	newStart := ev.VoteStart
	newEnd := ev.VoteEnd

	setParts := []string{}
	args := []any{}
	idxArg := 1

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
				http.Error(w, "invalid voteStart", http.StatusBadRequest)
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
				http.Error(w, "invalid voteEnd", http.StatusBadRequest)
				return
			}
			setParts = append(setParts, "vote_end = $"+itoa(idxArg))
			args = append(args, t)
			idxArg++
			newEnd = &t
		}
	}

	if err := validateVotingWindow(newStart, newEnd, time.Now()); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if ct.RowsAffected() == 0 {
		http.Error(w, "event not found", http.StatusNotFound)
		return
	}

	updated, err := loadEvent(r.Context(), s.pool, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(updated)
}
