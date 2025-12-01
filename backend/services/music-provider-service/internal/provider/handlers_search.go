package provider

import (
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) HandleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("query"))
	if q == "" {
		writeError(w, http.StatusBadRequest, "query is required")
		return
	}
	if len(q) > 200 {
		writeError(w, http.StatusBadRequest, "query is too long")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 25 {
			limit = v
		}
	}

	items, err := s.yt.SearchTracks(r.Context(), q, limit)
	if err != nil {
		// upstream YouTube error
		writeError(w, http.StatusBadGateway, "failed to query provider")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}
