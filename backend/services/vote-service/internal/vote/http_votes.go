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
	resp, err := registerVote(r.Context(), s.pool, s.rdb, eventID, voterID, body.TrackID, body.Lat, body.Lng)
	if err != nil {
		writeVoteError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
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
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	out := []Row{}
	for rows.Next() {
		var row Row
		if err := rows.Scan(&row.Track, &row.Count); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, out)
}
