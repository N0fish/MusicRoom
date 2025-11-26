package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	db *pgxpool.Pool
}

func main() {
	ctx := context.Background()

	dbURL := getenv("DATABASE_URL", "postgres://musicroom:musicroom@postgres:5432/musicroom?sslmode=disable")
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("user-service: failed to connect to DB: %v", err)
	}
	defer pool.Close()

	if err := autoMigrate(ctx, pool); err != nil {
		log.Fatalf("user-service: migrate error: %v", err)
	}

	srv := &Server{db: pool}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)

	// Health
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"user-service"}`))
	})

	// Authenticated user routes (gateway must set X-User-Id)
	r.Group(func(r chi.Router) {
		r.Use(currentUserMiddleware)

		// profile
		r.Get("/users/me", srv.handleGetMe)
		r.Patch("/users/me", srv.handlePatchMe)

		// avatar
		r.Post("/users/me/avatar/random", srv.handleGenerateRandomAvatar)

		// search
		r.Get("/users/search", srv.handleSearchUsers)

		// friends
		r.Get("/users/me/friends", srv.handleListFriends)
		r.Post("/users/me/friends/{id}/request", srv.handleSendFriendRequest)
		r.Post("/users/me/friends/{id}/accept", srv.handleAcceptFriendRequest)
		r.Post("/users/me/friends/{id}/reject", srv.handleRejectFriendRequest)
		r.Delete("/users/me/friends/{id}", srv.handleRemoveFriend)

		// viewing other profiles with visibility rules
		r.Get("/users/{id}", srv.handleGetPublicProfile)
	})

	port := getenv("PORT", "3005")
	log.Printf("user-service listening on :%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("user-service: %v", err)
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
