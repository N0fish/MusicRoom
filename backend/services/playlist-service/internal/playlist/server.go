package playlist

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
)

// DB interface abstracts the database connection (pool or mock).
type DB interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

type Server struct {
	db  DB
	rdb *redis.Client
}

func NewServer(db DB, rdb *redis.Client) *Server {
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

	r.Get("/playlists", s.handleListPlaylists)
	r.Post("/realtime/event", s.handleBroadcastEvent)

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

// POST /realtime/event
// Internal endpoint to broadcast events from other services (e.g. vote-service)
func (s *Server) handleBroadcastEvent(w http.ResponseWriter, r *http.Request) {
	// Decoding arbitrary JSON map
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	s.publishEvent(r.Context(), body)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
