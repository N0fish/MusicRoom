package vote

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Row struct {
	Track string `json:"track"`
	Count int    `json:"count"`
}

func (s *HTTPServer) handleVote(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")
	voterID := r.Header.Get("X-User-Id")
	if voterID == "" {
		http.Error(w, "missing X-User-Id", http.StatusUnauthorized)
		return
	}

	var body voteRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if body.TrackID == "" {
		http.Error(w, "trackId is required", http.StatusBadRequest)
		return
	}
	resp, err := registerVote(r.Context(), s.pool, s.rdb, eventID, voterID, body.TrackID, body.Lat, body.Lng)
	if err != nil {
		writeVoteError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *HTTPServer) handleTally(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")
	rows, err := s.pool.Query(r.Context(), `
        SELECT track, COUNT(*) AS c
        FROM votes
        WHERE event_id = $1
        GROUP BY track
        ORDER BY c DESC, track ASC
    `, eventID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var out []Row
	for rows.Next() {
		var row Row
		if err := rows.Scan(&row.Track, &row.Count); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		out = append(out, row)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
