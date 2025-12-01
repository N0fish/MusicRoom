package main

import (
	"log"
	"net/http"
	"os"
	"time"

	provider "music-provider-service/internal/provider"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	port := getenv("PORT", "3007")
	ytAPIKey := getenv("YOUTUBE_API_KEY", "")
	if ytAPIKey == "" {
		log.Fatal("YOUTUBE_API_KEY is required")
	}

	// провайдер (внутренний клиент YouTube)
	yt := provider.NewYouTubeClient(ytAPIKey)
	srv := provider.NewServer(yt)

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
