package provider

import "net/http"

type Server struct {
	yt *YouTubeClient
}

func NewServer(yt *YouTubeClient) *Server {
	return &Server{yt: yt}
}

func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "music-provider-service",
	})
}
