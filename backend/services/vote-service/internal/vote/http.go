package vote

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type HTTPServer struct {
	pool               *pgxpool.Pool
	rdb                *redis.Client
	userServiceURL     string
	playlistServiceURL string
	realtimeServiceURL string
}

func NewRouter(pool *pgxpool.Pool, rdb *redis.Client, userServiceURL, playlistServiceURL, realtimeServiceURL string) http.Handler {
	s := &HTTPServer{
		pool:               pool,
		rdb:                rdb,
		userServiceURL:     userServiceURL,
		playlistServiceURL: playlistServiceURL,
		realtimeServiceURL: realtimeServiceURL,
	}

	r := chi.NewRouter()

	// health
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"service": "vote-service",
		})
	})

	// events
	r.Get("/events", s.handleListEvents)
	r.Post("/events", s.handleCreateEvent)
	r.Get("/events/{id}", s.handleGetEvent)
	r.Delete("/events/{id}", s.handleDeleteEvent)
	r.Patch("/events/{id}", s.handlePatchEvent)

	// invites
	r.Post("/events/{id}/invites", s.handleCreateInvite)
	r.Delete("/events/{id}/invites/{userId}", s.handleDeleteInvite)
	r.Get("/events/{id}/invites", s.handleListInvites)

	// voting
	r.Post("/events/{id}/vote", s.handleVote)
	r.Get("/events/{id}/tally", s.handleTally)

	return r
}
