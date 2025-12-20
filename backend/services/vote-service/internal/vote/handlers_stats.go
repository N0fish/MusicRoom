package vote

import (
	"log"
	"net/http"
)

func (s *HTTPServer) handleGetStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := r.Header.Get("X-User-Id")
	if userID == "" {
		http.Error(w, "missing user id", http.StatusUnauthorized)
		return
	}

	stats, err := s.store.GetUserStats(ctx, userID)
	if err != nil {
		log.Printf("get stats error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, stats)
}
