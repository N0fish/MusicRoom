package playlist

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Server struct {
	db  *pgxpool.Pool
	rdb *redis.Client
}

func NewServer(db *pgxpool.Pool, rdb *redis.Client) *Server {
	return &Server{
		db:  db,
		rdb: rdb,
	}
}

func (s *Server) Router(middlewares ...func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()

	for _, mw := range middlewares {
		r.Use(mw)
	}

	r.Get("/health", s.handleHealth)

	r.Get("/playlists", s.handleListPublicPlaylists)

	r.Group(func(r chi.Router) {
		r.Post("/playlists", s.handleCreatePlaylist)
		r.Patch("/playlists/{id}", s.handlePatchPlaylist)
		r.Get("/playlists/{id}", s.handleGetPlaylist)

		r.Post("/playlists/{id}/tracks", s.handleAddTrack)
		r.Patch("/playlists/{id}/tracks/{trackId}", s.handleMoveTrack)
		r.Delete("/playlists/{id}/tracks/{trackId}", s.handleDeleteTrack)

		r.Get("/playlists/{id}/invites", s.handleListInvites)
		r.Post("/playlists/{id}/invites", s.handleAddInvite)
		r.Delete("/playlists/{id}/invites/{userId}", s.handleDeleteInvite)

		// Playback & Voting
		r.Post("/playlists/{id}/tracks/{trackId}/vote", s.handleVoteTrack)
		r.Post("/playlists/{id}/next", s.handleNextTrack)
	})

	return r
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "playlist-service",
	})
}
