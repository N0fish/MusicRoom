package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	playlist "playlist-service/internal/playlist"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	ctx := context.Background()

	port := getenv("PORT", "3002")
	dsn := getenv("DATABASE_URL", "postgres://musicroom:musicroom@postgres:5432/musicroom?sslmode=disable")
	redisURL := getenv("REDIS_URL", "redis://redis:6379")

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("playlist-service: unable to connect to database: %v", err)
	}
	defer pool.Close()

	if err := playlist.AutoMigrate(ctx, pool); err != nil {
		log.Fatalf("playlist-service: database migration failed: %v", err)
	}

	redisOpts, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("playlist-service: invalid REDIS_URL: %v", err)
	}
	rdb := redis.NewClient(redisOpts)
	defer rdb.Close()

	s := playlist.NewServer(pool, rdb)

	r := s.Router(
		middleware.RequestID,
		middleware.RealIP,
		middleware.Logger,
		middleware.Recoverer,
		middleware.Timeout(60*time.Second),
	)

	log.Printf("playlist-service listening on :%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("playlist-service: %v", err)
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
