package provider

import (
	"context"
	"net/http"

	"github.com/redis/go-redis/v9"
)

type Provider interface {
	SearchTracks(ctx context.Context, query string, limit int) ([]MusicSearchItem, error)
}

type Server struct {
	provider Provider
	rdb      *redis.Client
}

func NewServer(p Provider, rdb *redis.Client) *Server {
	return &Server{
		provider: p,
		rdb:      rdb,
	}
}

func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "music-provider-service",
	})
}
