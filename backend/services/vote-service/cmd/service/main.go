package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	vote "vote-service/cmd/service/vote"
)

func main() {
	port := getenv("PORT", "3003")
	dsn := getenv("DATABASE_URL", "postgres://musicroom:musicroom@postgres:5432/musicroom?sslmode=disable")
	redisURL := getenv("REDIS_URL", "redis://redis:6379")
	userServiceURL := getenv("USER_SERVICE_URL", "http://user-service:3005")

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("vote-service: pgxpool: %v", err)
	}
	defer pool.Close()

	if err := vote.AutoMigrate(ctx, pool); err != nil {
		log.Fatalf("vote-service: migrate: %v", err)
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("vote-service: redis parse: %v", err)
	}
	rdb := redis.NewClient(opt)
	defer rdb.Close()

	router := vote.NewRouter(pool, rdb, userServiceURL)

	log.Printf("vote-service listening on :%s", port)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatalf("vote-service: %v", err)
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
