package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	port := getenv("GATEWAY_PORT", "8080")
	openapiFile := getenv("OPENAPI_FILE", "openapi.yaml")

	authURL := getenv("AUTH_SERVICE_URL", "http://auth-service:3001")
	userURL := getenv("USER_SERVICE_URL", "http://user-service:3005")
	playlistURL := getenv("PLAYLIST_SERVICE_URL", "http://playlist-service:3002")
	voteURL := getenv("VOTE_SERVICE_URL", "http://vote-service:3003")
	mockURL := getenv("MOCK_SERVICE_URL", "http://mock-service:3006")
	realtimeURL := getenv("REALTIME_SERVICE_URL", "http://realtime-service:3004")

	jwtSecret := []byte(getenv("JWT_SECRET", ""))
	if len(jwtSecret) == 0 {
		log.Println("api-gateway: WARNING: JWT_SECRET is empty, JWT validation disabled")
	}

	rps := getenvInt("RATE_LIMIT_RPS", 20)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(rateLimitMiddleware(rps))
	r.Use(corsMiddleware)

	// health
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// _, _ = w.Write([]byte(`{"status":"ok","service":"api-gateway"}`))
		_, _ = w.Write([]byte("ok"))
	})

	// openapi.yaml
	r.Get("/docs/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, openapiFile)
	})

	// Proxies
	authProxy := mustNewReverseProxy(authURL)
	userProxy := mustNewReverseProxy(userURL)
	playlistProxy := mustNewReverseProxy(playlistURL)
	voteProxy := mustNewReverseProxy(voteURL)
	mockProxy := mustNewReverseProxy(mockURL)
	realtimeProxy := mustNewReverseProxy(realtimeURL)

	// Auth routes (no JWT required)
	r.Method(http.MethodPost, "/auth/register", authProxy)
	// r.Method(http.MethodPost, "/auth/login", authProxy)
	r.Group(func(r chi.Router) {
		r.Use(loginRateLimitMiddleware)
		r.Method(http.MethodPost, "/auth/login", authProxy)
	})
	r.Method(http.MethodPost, "/auth/refresh", authProxy)
	r.Method(http.MethodPost, "/auth/forgot-password", authProxy)
	r.Method(http.MethodPost, "/auth/reset-password", authProxy)
	r.Method(http.MethodPost, "/auth/request-email-verification", authProxy)
	r.Method(http.MethodGet, "/auth/verify-email", authProxy)

	r.Method(http.MethodGet, "/auth/google/login", authProxy)
	r.Method(http.MethodGet, "/auth/google/callback", authProxy)
	r.Method(http.MethodGet, "/auth/42/login", authProxy)
	r.Method(http.MethodGet, "/auth/42/callback", authProxy)

	// /auth/me можно либо через прямой proxy, либо через gateway JWT
	r.Method(http.MethodGet, "/auth/me", authProxy)

	// User-service (JWT required)
	r.Group(func(r chi.Router) {
		if len(jwtSecret) != 0 {
			r.Use(jwtAuthMiddleware(jwtSecret))
		}
		r.Method(http.MethodGet, "/users/me", userProxy)
		r.Method(http.MethodGet, "/users/{id}", userProxy)
		r.Method(http.MethodPatch, "/users/me", userProxy)
	})

	// Playlists
	r.Method(http.MethodGet, "/playlists", playlistProxy) // public playlists
	r.Group(func(r chi.Router) {
		if len(jwtSecret) != 0 {
			r.Use(jwtAuthMiddleware(jwtSecret))
		}
		r.With(playlistCreateRateLimitMiddleware).
			Method(http.MethodPost, "/playlists", playlistProxy)
		r.Method(http.MethodPatch, "/playlists/{id}", playlistProxy)
	})

	// Voting
	r.Group(func(r chi.Router) {
		if len(jwtSecret) != 0 {
			r.Use(jwtAuthMiddleware(jwtSecret))
		}
		r.Method(http.MethodPost, "/events/{id}/vote", voteProxy)
	})

	// Mock routes
	r.Mount("/mock", mockProxy)

	// Realtime (ws passthrough is handled in realtime-service; here we mostly proxy HTTP control if needed)
	r.Mount("/realtime", realtimeProxy)

	log.Printf("api-gateway listening on :%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("api-gateway: %v", err)
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
