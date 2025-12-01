package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	realtime "realtime-service/internal/realtime"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"
)

func main() {
	ctx := context.Background()

	port := getenv("PORT", "3004")
	redisURL := getenv("REDIS_URL", "redis://redis:6379")
	frontendBaseURL := getenv("FRONTEND_BASE_URL", "")

	// Redis
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("realtime-service: invalid REDIS_URL: %v", err)
	}
	rdb := redis.NewClient(opt)
	defer rdb.Close()

	// Hub + сервер
	hub := realtime.NewHub()
	srv := realtime.NewServer(hub, rdb, ctx, frontendBaseURL)

	// Запускаем фоновые горутины (hub + подписка на Redis)
	go hub.Run()
	go srv.RunRedisSubscriber()

	// HTTP router с базовыми middleware
	r := srv.Router(
		middleware.RequestID,
		middleware.RealIP,
		middleware.Logger,
		middleware.Recoverer,
		middleware.Timeout(60*time.Second),
	)

	log.Printf("realtime-service listening on :%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("realtime-service: %v", err)
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
