package vote

import (
	"encoding/json"
	"log"
	"net/http"

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
		pURL := s.playlistServiceURL + "/playlists/" + eventID + "/tracks/" + body.TrackID + "/vote"
		req, _ := http.NewRequest("POST", pURL, nil)
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
