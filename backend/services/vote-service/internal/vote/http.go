package vote

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type HTTPServer struct {
	pool               *pgxpool.Pool
	store              Store
	rdb                *redis.Client
	httpClient         *http.Client
	userServiceURL     string
	playlistServiceURL string
	realtimeServiceURL string
}

func NewRouter(pool *pgxpool.Pool, rdb *redis.Client, userServiceURL, playlistServiceURL, realtimeServiceURL string) http.Handler {
	s := &HTTPServer{
		pool:               pool,
		store:              NewPostgresStore(pool),
		rdb:                rdb,
		httpClient:         &http.Client{Timeout: 15 * time.Second},
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
	r.Post("/events/{id}/transfer-ownership", s.handleTransferOwnership)

	// invites
	r.Post("/events/{id}/invites", s.handleCreateInvite)
	r.Delete("/events/{id}/invites/{userId}", s.handleDeleteInvite)
	r.Get("/events/{id}/invites", s.handleListInvites)

	// voting
	r.Post("/events/{id}/vote", s.handleVote)
	r.Delete("/events/{id}/vote", s.handleRemoveVote)
	r.Get("/events/{id}/tally", s.handleTally)
	r.Delete("/events/{id}/votes", s.handleClearVotes)

	// stats
	r.Get("/stats", s.handleGetStats)

	return r
}
