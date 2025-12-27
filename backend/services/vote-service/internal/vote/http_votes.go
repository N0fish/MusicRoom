package vote

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type Row struct {
	Track    string `json:"track"`
	Count    int    `json:"count"`
	IsMyVote bool   `json:"isMyVote"`
}

func (s *HTTPServer) handleVote(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")
	voterID := r.Header.Get("X-User-Id")
	if voterID == "" {
		writeError(w, http.StatusUnauthorized, "missing X-User-Id")
		return
	}

	var body voteRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var lVal, gVal float64
	if body.Lat != nil {
		lVal = *body.Lat
	}
	if body.Lng != nil {
		gVal = *body.Lng
	}
	log.Printf("[DEBUG] VoteRequest: UserID=%s EventID=%s TrackID=%s Lat=%f Lng=%f\n", voterID, eventID, body.TrackID, lVal, gVal)

	if body.TrackID == "" {
		writeError(w, http.StatusBadRequest, "trackId is required")
		return
	}
	resp, err := registerVote(r.Context(), s.store, s.rdb, eventID, voterID, body.TrackID, body.Lat, body.Lng)
	if err != nil {
		writeVoteError(w, err)
		return
	}

	// Forward to playlist-service to update track order
	// We ignore errors here to not block the response, but log them
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		pURL := s.playlistServiceURL + "/playlists/" + eventID + "/tracks/" + body.TrackID + "/vote"
		req, _ := http.NewRequestWithContext(ctx, "POST", pURL, nil)
		req.Header.Set("X-User-Id", voterID)
		if resp, err := s.httpClient.Do(req); err != nil {
			log.Printf("Failed to forward vote to playlist-service: %v", err)
		} else {
			resp.Body.Close()
		}
	}()

	writeJSON(w, http.StatusOK, resp)
}

func (s *HTTPServer) handleRemoveVote(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")
	voterID := r.Header.Get("X-User-Id")
	if voterID == "" {
		writeError(w, http.StatusUnauthorized, "missing X-User-Id")
		return
	}

	var body voteRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.TrackID == "" {
		writeError(w, http.StatusBadRequest, "trackId is required")
		return
	}

	resp, err := removeVote(r.Context(), s.store, s.rdb, eventID, voterID, body.TrackID)
	if err != nil {
		writeVoteError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *HTTPServer) handleTally(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")
	voterID := r.Header.Get("X-User-Id") // Optional? If missing, isMyVote is false.

	out, err := s.store.GetVoteTally(r.Context(), eventID, voterID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, out)
}

func (s *HTTPServer) handleClearVotes(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")
	trackID := r.URL.Query().Get("track")
	// userID := r.Header.Get("X-User-Id") // Potential owner check here

	var err error
	if trackID != "" {
		_, err = s.pool.Exec(r.Context(), "DELETE FROM votes WHERE event_id=$1 AND track=$2", eventID, trackID)
	} else {
		_, err = s.pool.Exec(r.Context(), "DELETE FROM votes WHERE event_id=$1", eventID)
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
}
