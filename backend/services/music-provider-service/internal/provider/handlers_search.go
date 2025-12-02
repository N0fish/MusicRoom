package provider

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (s *Server) HandleSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	q := strings.TrimSpace(r.URL.Query().Get("query"))
	if q == "" {
		writeError(w, http.StatusBadRequest, "query is required")
		return
	}
	if len(q) > 200 {
		writeError(w, http.StatusBadRequest, "query is too long")
		return
	}

	providerParam := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("provider")))
	if providerParam == "" {
		providerParam = "youtube"
	}
	if providerParam != "youtube" {
		writeError(w, http.StatusBadRequest, "unsupported provider")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 25 {
			limit = v
		}
	}

	// --- CACHE KEY ---
	cacheKey := "music:search:" + q + ":" + strconv.Itoa(limit)

	if s.rdb != nil {
		if cached, err := s.rdb.Get(ctx, cacheKey).Result(); err == nil {
			var resp map[string]any
			if json.Unmarshal([]byte(cached), &resp) == nil {
				writeJSON(w, http.StatusOK, resp)
				return
			}
		}
	}

	items, err := s.yt.SearchTracks(ctx, q, limit)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to query provider")
		return
	}

	resp := map[string]any{"items": items}

	if s.rdb != nil {
		if data, err := json.Marshal(resp); err == nil {
			s.rdb.Set(ctx, cacheKey, data, 3*time.Minute)
		}
	}

	writeJSON(w, http.StatusOK, resp)
}
