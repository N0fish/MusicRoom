package main

import (
	"log"
	"net/http"
	"os"
	"time"

	provider "music-provider-service/internal/provider"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"
)

func main() {
	port := getenv("PORT", "3007")
	ytAPIKey := getenv("YOUTUBE_API_KEY", "")
	if ytAPIKey == "" {
		log.Fatal("YOUTUBE_API_KEY is required")
	}

	redisURL := getenv("REDIS_URL", "redis://redis:6379")
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("invalid REDIS_URL: %v", err)
	}
	rdb := redis.NewClient(opt)
	defer rdb.Close()

	searchBaseURL := getenv("YOUTUBE_SEARCH_URL", "https://www.googleapis.com/youtube/v3/search")

	yt := provider.NewYouTubeClient(ytAPIKey, searchBaseURL)
	srv := provider.NewServer(yt, rdb)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))

	r.Get("/health", srv.HandleHealth)
	r.Get("/music/search", srv.HandleSearch)

	log.Printf("music-provider-service listening on :%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("music-provider-service: %v", err)
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
