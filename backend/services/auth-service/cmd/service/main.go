package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	db              *pgxpool.Pool
	jwtSecret       []byte
	accessTTL       time.Duration
	refreshTTL      time.Duration
	googleCfg       GoogleConfig
	ftCfg           FTConfig
	frontendURL     string
	frontendBaseURL string

	emailSender         EmailSender
	verificationURLBase string
	resetURLBase        string
}

func main() {
	ctx := context.Background()

	dbURL := getenv("DATABASE_URL", "postgres://musicroom:musicroom@postgres:5432/musicroom?sslmode=disable")
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("auth-service: failed to connect to DB: %v", err)
	}
	defer pool.Close()

	if err := autoMigrate(ctx, pool); err != nil {
		log.Fatalf("auth-service: migrate error: %v", err)
	}

	jwtSecret := []byte(getenv("JWT_SECRET", ""))
	if len(jwtSecret) == 0 {
		log.Fatal("auth-service: JWT_SECRET is required")
	}

	accessTTL := mustParseDuration("ACCESS_TOKEN_TTL", "15m")
	refreshTTL := mustParseDuration("REFRESH_TOKEN_TTL", "720h")

	emailSender, err := NewSMTPSenderFromEnv()
	if err != nil {
		log.Printf("auth-service: SMTP not configured, using LogEmailSender: %v", err)
		emailSender = LogEmailSender{}
	}

	srv := &Server{
		db:              pool,
		jwtSecret:       jwtSecret,
		accessTTL:       accessTTL,
		refreshTTL:      refreshTTL,
		googleCfg:       loadGoogleConfigFromEnv(),
		ftCfg:           loadFTConfigFromEnv(),
		frontendURL:     getenv("OAUTH_FRONTEND_REDIRECT", ""),
		frontendBaseURL: getenv("FRONTEND_BASE_URL", ""),

		emailSender:         emailSender,
		verificationURLBase: getenv("EMAIL_VERIFICATION_URL", ""),
		resetURLBase:        getenv("PASSWORD_RESET_URL", ""),
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Health
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// _, _ = w.Write([]byte(`{"status":"ok","service":"auth-service"}`))
		_, _ = w.Write([]byte("ok"))
	})

	// Email/password auth
	r.Post("/auth/register", srv.handleRegister)
	r.Post("/auth/login", srv.handleLogin)
	r.Post("/auth/refresh", srv.handleRefresh)

	// Email verification and password reset
	r.Post("/auth/request-email-verification", srv.handleRequestEmailVerification)
	r.Get("/auth/verify-email", srv.handleVerifyEmail)
	r.Post("/auth/forgot-password", srv.handleForgotPassword)
	r.Post("/auth/reset-password", srv.handleResetPassword)

	// OAuth flows (Google & 42)
	r.Get("/auth/google/login", srv.handleGoogleLogin)
	r.Get("/auth/google/callback", srv.handleGoogleCallback)

	r.Get("/auth/42/login", srv.handleFTLogin)
	r.Get("/auth/42/callback", srv.handleFTCallback)

	// Minimal technical info about current session
	r.Group(func(r chi.Router) {
		r.Use(srv.authMiddleware)
		r.Get("/auth/me", srv.handleMe)
	})

	port := getenv("PORT", "3001")
	log.Printf("auth-service listening on :%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("auth-service: %v", err)
	}
}

func mustParseDuration(envKey, def string) time.Duration {
	raw := getenv(envKey, def)
	dur, err := time.ParseDuration(raw)
	if err != nil {
		log.Fatalf("auth-service: invalid duration in %s=%s: %v", envKey, raw, err)
	}
	return dur
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
