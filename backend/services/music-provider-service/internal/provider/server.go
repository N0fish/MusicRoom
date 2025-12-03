package provider

import (
	"net/http"

	"github.com/redis/go-redis/v9"
)

type Server struct {
	yt  *YouTubeClient
	rdb *redis.Client
}

func NewServer(yt *YouTubeClient, rdb *redis.Client) *Server {
	return &Server{
		yt:  yt,
		rdb: rdb,
	}
}

func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "music-provider-service",
	})
}
