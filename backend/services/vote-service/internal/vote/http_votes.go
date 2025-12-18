package vote

import (
	"encoding/json"
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
	if body.TrackID == "" {
		writeError(w, http.StatusBadRequest, "trackId is required")
		return
	}
	resp, err := registerVote(r.Context(), s.store, s.rdb, eventID, voterID, body.TrackID, body.Lat, body.Lng)
	if err != nil {
		writeVoteError(w, err)
		return
	}

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
