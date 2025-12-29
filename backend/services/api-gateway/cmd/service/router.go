package main

import (
	"io"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

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

func stripTrustedHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent header spoofing by clients
		r.Header.Del("X-User-Id")
		r.Header.Del("X-User-Email")
		next.ServeHTTP(w, r)
	})
}

func setupRouter(cfg Config) *chi.Mux {
	// Configure trusted proxies for clientIP()
	setTrustedProxyCIDRs(cfg.TrustedProxyCIDRs)

	r := chi.NewRouter()

	r.Use(corsMiddleware)
	r.Use(middleware.RequestID)
	r.Use(stripTrustedHeadersMiddleware)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Global rate limit for public traffic (IP-based)
	r.Use(rateLimitMiddleware(cfg.RateLimitRPS, rateKeyIP, "global"))
	r.Use(requestLogMiddleware)

	// Proxies
	authProxy := mustNewReverseProxy(cfg.AuthURL)
	userProxy := mustNewReverseProxy(cfg.UserURL)
	voteProxy := mustNewReverseProxy(cfg.VoteURL)
	playlistProxy := mustNewReverseProxy(cfg.PlaylistURL)
	mockProxy := mustNewReverseProxy(cfg.MockURL)
	realtimeProxy := mustNewReverseProxy(cfg.RealtimeURL)
	musicProxy := mustNewReverseProxy(cfg.MusicProviderURL)

	// Websocket / realtime
	r.Mount("/realtime", realtimeProxy)
	r.HandleFunc("/ws", realtimeProxy.ServeHTTP)

	api := chi.NewRouter()
	api.Use(middleware.Timeout(30 * time.Second))

	// health
	api.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"service": "api-gateway",
		})
	})

	// openapi.yaml
	api.Get("/docs/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, cfg.OpenAPIFile)
	})

	// swagger ui
	api.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(swaggerUIHTML))
	})

	// Audit / Telemetry
	api.Post("/audit/logs", func(w http.ResponseWriter, r *http.Request) {
		platform := r.Header.Get("X-Client-Platform")
		device := r.Header.Get("X-Client-Device")
		version := r.Header.Get("X-Client-App-Version")
		userId := r.Header.Get("X-User-Id")

		body, _ := io.ReadAll(r.Body)
		log.Printf("audit_log: user=%s platform=%s device=%s version=%s body=%s",
			userId, platform, device, version, string(body),
		)

		w.WriteHeader(http.StatusOK)
	})

	// Auth routes (public)
	api.Method(http.MethodPost, "/auth/register", authProxy)
	api.Group(func(r chi.Router) {
		r.Use(loginRateLimitMiddleware)
		r.Method(http.MethodPost, "/auth/login", authProxy)
	})
	api.Method(http.MethodPost, "/auth/refresh", authProxy)
	api.Method(http.MethodPost, "/auth/forgot-password", authProxy)
	api.Method(http.MethodPost, "/auth/reset-password", authProxy)
	api.Get("/auth/reset-password", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		http.Redirect(w, r, getenv("FRONTEND_BASE_URL", "")+"/auth?mode=reset-password&token="+token, http.StatusFound)
	})
	api.Method(http.MethodPost, "/auth/request-email-verification", authProxy)
	api.Method(http.MethodGet, "/auth/verify-email", authProxy)

	api.Method(http.MethodGet, "/auth/google/login", authProxy)
	api.Method(http.MethodGet, "/auth/google/callback", authProxy)
	api.Method(http.MethodGet, "/auth/42/login", authProxy)
	api.Method(http.MethodGet, "/auth/42/callback", authProxy)

	// Auth routes (protected)
	api.Group(func(r chi.Router) {
		r.Use(jwtAuthMiddleware(cfg.JWTSecret))
		r.Method(http.MethodGet, "/auth/me", authProxy)
		r.Method(http.MethodPost, "/auth/link/{provider}", authProxy)
		r.Method(http.MethodDelete, "/auth/link/{provider}", authProxy)
	})

	// User-service
	api.Method(http.MethodGet, "/avatars/*", userProxy)

	api.Group(func(r chi.Router) {
		r.Use(jwtAuthMiddleware(cfg.JWTSecret))
		r.Use(rateLimitMiddleware(getenvInt("AUTHED_RPS", 30), rateKeyUserOrIP, "authed"))

		r.Method(http.MethodGet, "/users/me", userProxy)

		r.With(
			bodySizeLimitMiddleware(int64(getenvInt("USER_PATCH_BODY_LIMIT", 4096))),
			rateLimitMiddleware(getenvInt("USER_PATCH_RPS", 5), rateKeyUserOrIP, "user_patch"),
		).Method(http.MethodPatch, "/users/me", userProxy)

		r.Method(http.MethodPost, "/users/me/premium", userProxy)

		r.Method(http.MethodPost, "/users/me/avatar/random", userProxy)
		r.With(
			bodySizeLimitMiddleware(6*1024*1024),
			rateLimitMiddleware(getenvInt("AVATAR_UPLOAD_RPS", 1), rateKeyUserOrIP, "avatar_upload"),
		).Method(http.MethodPost, "/users/me/avatar/upload", userProxy)

		r.Method(http.MethodGet, "/users/search", userProxy)

		r.Method(http.MethodGet, "/users/me/friends", userProxy)
		r.Method(http.MethodGet, "/users/me/friends/requests/incoming", userProxy)

		r.With(bodySizeLimitMiddleware(2048), rateLimitMiddleware(getenvInt("FRIEND_REQUEST_RPS", 5), rateKeyUserOrIP, "friend_request")).
			Method(http.MethodPost, "/users/me/friends/{id}/request", userProxy)
		r.With(bodySizeLimitMiddleware(2048), rateLimitMiddleware(getenvInt("FRIEND_REQUEST_RPS", 5), rateKeyUserOrIP, "friend_request")).
			Method(http.MethodPost, "/users/me/friends/{id}/accept", userProxy)
		r.With(bodySizeLimitMiddleware(2048), rateLimitMiddleware(getenvInt("FRIEND_REQUEST_RPS", 5), rateKeyUserOrIP, "friend_request")).
			Method(http.MethodPost, "/users/me/friends/{id}/reject", userProxy)
		r.With(bodySizeLimitMiddleware(2048), rateLimitMiddleware(getenvInt("FRIEND_REQUEST_RPS", 5), rateKeyUserOrIP, "friend_request")).
			Method(http.MethodDelete, "/users/me/friends/{id}", userProxy)

		r.Method(http.MethodGet, "/users/{id}", userProxy)
	})

	// Playlists
	api.With(jwtAuthOptionalMiddleware(cfg.JWTSecret)).
		Method(http.MethodGet, "/playlists", playlistProxy)

	api.Group(func(r chi.Router) {
		r.Use(jwtAuthMiddleware(cfg.JWTSecret))
		r.Use(rateLimitMiddleware(getenvInt("PLAYLIST_AUTHED_RPS", 30), rateKeyUserOrIP, "playlist_authed"))

		r.With(playlistCreateRateLimitMiddleware).
			Method(http.MethodPost, "/playlists", playlistProxy)

		r.Method(http.MethodPatch, "/playlists/{id}", playlistProxy)
		r.Method(http.MethodGet, "/playlists/{id}", playlistProxy)
		r.Method(http.MethodDelete, "/playlists/{id}", playlistProxy)

		r.Method(http.MethodPost, "/playlists/{id}/tracks", playlistProxy)
		r.Method(http.MethodPatch, "/playlists/{id}/tracks/{trackId}", playlistProxy)
		r.Method(http.MethodDelete, "/playlists/{id}/tracks/{trackId}", playlistProxy)

		r.Method(http.MethodPost, "/playlists/{id}/tracks/{trackId}/vote", playlistProxy)
		r.Method(http.MethodPost, "/playlists/{id}/next", playlistProxy)

		r.Method(http.MethodGet, "/playlists/{id}/invites", playlistProxy)
		r.Method(http.MethodPost, "/playlists/{id}/invites", playlistProxy)
		r.Method(http.MethodDelete, "/playlists/{id}/invites/{userId}", playlistProxy)
	})

	// Events & Voting
	api.Group(func(r chi.Router) {
		r.Use(jwtAuthMiddleware(cfg.JWTSecret))
		r.Use(rateLimitMiddleware(getenvInt("VOTE_AUTHED_RPS", 30), rateKeyUserOrIP, "vote_authed"))

		r.Method(http.MethodGet, "/events", voteProxy)
		r.Method(http.MethodPost, "/events", voteProxy)
		r.Method(http.MethodGet, "/events/{id}", voteProxy)
		r.Method(http.MethodPatch, "/events/{id}", voteProxy)

		r.Method(http.MethodPost, "/events/{id}/transfer-ownership", voteProxy)
		r.Method(http.MethodDelete, "/events/{id}", voteProxy)

		r.Method(http.MethodGet, "/events/{id}/invites", voteProxy)
		r.Method(http.MethodPost, "/events/{id}/invites", voteProxy)
		r.Method(http.MethodDelete, "/events/{id}/invites/{userId}", voteProxy)

		r.Method(http.MethodPost, "/events/{id}/vote", voteProxy)
		r.Method(http.MethodDelete, "/events/{id}/vote", voteProxy)
		r.Method(http.MethodDelete, "/events/{id}/votes", voteProxy)
		r.Method(http.MethodGet, "/events/{id}/tally", voteProxy)

		r.Method(http.MethodGet, "/stats", voteProxy)
	})

	// Mock routes
	api.Mount("/mock", mockProxy)

	// Music provider
	api.Group(func(r chi.Router) {
		r.Use(jwtAuthMiddleware(cfg.JWTSecret))
		r.Use(rateLimitMiddleware(getenvInt("MUSIC_AUTHED_RPS", 30), rateKeyUserOrIP, "music_authed"))
		r.Method(http.MethodGet, "/music/search", musicProxy)
	})

	r.Mount("/", api)
	return r
}
