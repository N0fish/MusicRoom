package main

import (
	"io"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Config struct {
	Port             string
	OpenAPIFile      string
	AuthURL          string
	UserURL          string
	VoteURL          string
	PlaylistURL      string
	MockURL          string
	RealtimeURL      string
	MusicProviderURL string
	JWTSecret        []byte
	RateLimitRPS     int
}

const swaggerUIHTML = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Music Room API Documentation</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" >
    <style>
        html { box-sizing: border-box; overflow: -moz-scrollbars-vertical; overflow-y: scroll; }
        *, *:before, *:after { box-sizing: inherit; }
        body { margin:0; background: #fafafa; }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"> </script>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-standalone-preset.js"> </script>
    <script>
    window.onload = function() {
      const ui = SwaggerUIBundle({
        url: "/docs/openapi.yaml",
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIStandalonePreset
        ],
        plugins: [
          SwaggerUIBundle.plugins.DownloadUrl
        ],
        layout: "StandaloneLayout"
      });
      window.ui = ui;
    };
    </script>
</body>
</html>
`

func main() {
	config := Config{
		Port:             getenv("PORT", "8080"),
		OpenAPIFile:      getenv("OPENAPI_FILE", "openapi.yaml"),
		AuthURL:          getenv("AUTH_SERVICE_URL", "http://auth-service:3001"),
		UserURL:          getenv("USER_SERVICE_URL", "http://user-service:3005"),
		VoteURL:          getenv("VOTE_SERVICE_URL", "http://vote-service:3003"),
		PlaylistURL:      getenv("PLAYLIST_SERVICE_URL", "http://playlist-service:3002"),
		MockURL:          getenv("MOCK_SERVICE_URL", "http://mock-service:3006"),
		RealtimeURL:      getenv("REALTIME_SERVICE_URL", "http://realtime-service:3004"),
		MusicProviderURL: getenv("MUSIC_PROVIDER_SERVICE_URL", "http://music-provider-service:3007"),
		JWTSecret:        []byte(getenv("JWT_SECRET", "")),
		RateLimitRPS:     getenvInt("RATE_LIMIT_RPS", 20),
	}

	if len(config.JWTSecret) == 0 {
		log.Fatal("api-gateway: JWT_SECRET is empty, cannot start without JWT validation")
	}

	r := setupRouter(config)

	log.Printf("api-gateway listening on :%s", config.Port)
	if err := http.ListenAndServe(":"+config.Port, r); err != nil {
		log.Fatalf("api-gateway: %v", err)
	}
}

func setupRouter(cfg Config) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(rateLimitMiddleware(cfg.RateLimitRPS))
	r.Use(corsMiddleware)
	r.Use(requestLogMiddleware)

	// health
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"service": "api-gateway",
		})
	})

	// Audit / Telemetry
	r.Post("/audit/logs", func(w http.ResponseWriter, r *http.Request) {
		platform := r.Header.Get("X-Client-Platform")
		device := r.Header.Get("X-Client-Device")
		version := r.Header.Get("X-Client-App-Version")
		userId := r.Header.Get("X-User-Id")

		body, _ := io.ReadAll(r.Body)
		log.Printf("audit_log: user=%s platform=%s device=%s version=%s body=%s",
			userId, platform, device, version, string(body))

		w.WriteHeader(http.StatusOK)
	})

	// openapi.yaml
	r.Get("/docs/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, cfg.OpenAPIFile)
	})

	r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(swaggerUIHTML))
	})

	// Proxies
	authProxy := mustNewReverseProxy(cfg.AuthURL)
	userProxy := mustNewReverseProxy(cfg.UserURL)
	voteProxy := mustNewReverseProxy(cfg.VoteURL)
	playlistProxy := mustNewReverseProxy(cfg.PlaylistURL)
	mockProxy := mustNewReverseProxy(cfg.MockURL)
	realtimeProxy := mustNewReverseProxy(cfg.RealtimeURL)
	musicProxy := mustNewReverseProxy(cfg.MusicProviderURL)

	// Auth routes (no JWT required)
	r.Method(http.MethodPost, "/auth/register", authProxy)
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

	r.Method(http.MethodGet, "/auth/me", authProxy)
	r.Method(http.MethodPost, "/auth/link/{provider}", authProxy)
	r.Method(http.MethodDelete, "/auth/link/{provider}", authProxy)

	// User-service (JWT required)
	r.Method(http.MethodGet, "/avatars/*", userProxy)
	r.Group(func(r chi.Router) {
		if len(cfg.JWTSecret) != 0 {
			r.Use(jwtAuthMiddleware(cfg.JWTSecret))
		}

		// profile
		r.Method(http.MethodGet, "/users/me", userProxy)
		r.With(
			bodySizeLimitMiddleware(int64(getenvInt("USER_PATCH_BODY_LIMIT", 4096))),
			rateLimitMiddleware(getenvInt("USER_PATCH_RPS", 5)),
		).
			Method(http.MethodPatch, "/users/me", userProxy)

		r.Method(http.MethodPost, "/users/me/avatar/random", userProxy)
		r.With(
			bodySizeLimitMiddleware(6*1024*1024),
			rateLimitMiddleware(getenvInt("AVATAR_UPLOAD_RPS", 1)),
		).
			Method(http.MethodPost, "/users/me/avatar/upload", userProxy)

		// search
		r.Method(http.MethodGet, "/users/search", userProxy)

		// friends
		r.Method(http.MethodGet, "/users/me/friends", userProxy)
		r.Method(http.MethodGet, "/users/me/friends/requests/incoming", userProxy)
		r.With(
			bodySizeLimitMiddleware(2048),
			rateLimitMiddleware(getenvInt("FRIEND_REQUEST_RPS", 5)),
		).
			Method(http.MethodPost, "/users/me/friends/{id}/request", userProxy)
		r.With(
			bodySizeLimitMiddleware(2048),
			rateLimitMiddleware(getenvInt("FRIEND_REQUEST_RPS", 5)),
		).
			Method(http.MethodPost, "/users/me/friends/{id}/accept", userProxy)
		r.With(
			bodySizeLimitMiddleware(2048),
			rateLimitMiddleware(getenvInt("FRIEND_REQUEST_RPS", 5)),
		).
			Method(http.MethodPost, "/users/me/friends/{id}/reject", userProxy)
		r.With(
			bodySizeLimitMiddleware(2048),
			rateLimitMiddleware(getenvInt("FRIEND_REQUEST_RPS", 5)),
		).
			Method(http.MethodDelete, "/users/me/friends/{id}", userProxy)

		// public profile
		r.Method(http.MethodGet, "/users/{id}", userProxy)
	})

	// Playlists
	r.With(jwtAuthOptionalMiddleware(cfg.JWTSecret)).
		Method(http.MethodGet, "/playlists", playlistProxy) // public + owned playlists
	r.Group(func(r chi.Router) {
		if len(cfg.JWTSecret) != 0 {
			r.Use(jwtAuthMiddleware(cfg.JWTSecret))
		}
		r.With(playlistCreateRateLimitMiddleware).
			Method(http.MethodPost, "/playlists", playlistProxy)
		r.Method(http.MethodPatch, "/playlists/{id}", playlistProxy)
		r.Method(http.MethodGet, "/playlists/{id}", playlistProxy)

		r.Method(http.MethodPost, "/playlists/{id}/tracks", playlistProxy)
		r.Method(http.MethodPatch, "/playlists/{id}/tracks/{trackId}", playlistProxy) // move track
		r.Method(http.MethodDelete, "/playlists/{id}/tracks/{trackId}", playlistProxy)

		r.Method(http.MethodPost, "/playlists/{id}/tracks/{trackId}/vote", playlistProxy)
		r.Method(http.MethodPost, "/playlists/{id}/next", playlistProxy)

		r.Method(http.MethodGet, "/playlists/{id}/invites", playlistProxy)
		r.Method(http.MethodPost, "/playlists/{id}/invites", playlistProxy)
		r.Method(http.MethodDelete, "/playlists/{id}/invites/{userId}", playlistProxy)
	})

	// Events & Voting
	r.Group(func(r chi.Router) {
		if len(cfg.JWTSecret) != 0 {
			r.Use(jwtAuthMiddleware(cfg.JWTSecret))
		}
		// Event lifecycle and settings
		r.Method(http.MethodGet, "/events", voteProxy)
		r.Method(http.MethodPost, "/events", voteProxy)
		r.Method(http.MethodGet, "/events/{id}", voteProxy)
		r.Method(http.MethodPatch, "/events/{id}", voteProxy)
		r.Method(http.MethodPost, "/events/{id}/transfer-ownership", voteProxy)
		r.Method(http.MethodDelete, "/events/{id}", voteProxy)
		// Invites
		r.Method(http.MethodGet, "/events/{id}/invites", voteProxy)
		r.Method(http.MethodPost, "/events/{id}/invites", voteProxy)
		r.Method(http.MethodDelete, "/events/{id}/invites/{userId}", voteProxy)
		// Voting
		r.Method(http.MethodPost, "/events/{id}/vote", voteProxy)
		r.Method(http.MethodDelete, "/events/{id}/vote", voteProxy)
		r.Method(http.MethodGet, "/events/{id}/tally", voteProxy)
	})

	// Mock routes
	r.Mount("/mock", mockProxy)

	// Realtime (ws passthrough is handled in realtime-service; here we mostly proxy HTTP control if needed)
	r.Mount("/realtime", realtimeProxy)
	r.HandleFunc("/ws", realtimeProxy.ServeHTTP)

	// Music provider (search for tracks in external SDK)
	r.Group(func(r chi.Router) {
		if len(cfg.JWTSecret) != 0 {
			r.Use(jwtAuthMiddleware(cfg.JWTSecret))
		}
		r.Method(http.MethodGet, "/music/search", musicProxy)
	})

	return r
}
