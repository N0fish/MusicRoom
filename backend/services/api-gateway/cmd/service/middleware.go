package main

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenClaims struct {
	UserID        string `json:"uid"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"emailVerified"`
	TokenType     string `json:"typ"`
	jwt.RegisteredClaims
}

type ctxClaimsKey struct{}

func jwtAuthMiddleware(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				writeError(w, http.StatusUnauthorized, "missing Authorization header")
				return
			}
			parts := strings.SplitN(auth, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeError(w, http.StatusUnauthorized, "invalid Authorization header")
				return
			}
			raw := parts[1]

			claims := &TokenClaims{}
			token, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (interface{}, error) {
				return secret, nil
			})
			if err != nil || !token.Valid || claims.TokenType != "access" {
				writeError(w, http.StatusUnauthorized, "invalid token")
				return
			}

			r.Header.Set("X-User-Id", claims.UserID)
			r.Header.Set("X-User-Email", claims.Email)

			ctx := context.WithValue(r.Context(), ctxClaimsKey{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	allowedOrigin := getenv("CORS_ALLOWED_ORIGIN", "*")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if strings.ToUpper(r.Method) == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bodySizeLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > 0 && r.ContentLength > maxBytes {
				writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

func requestLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		platform := r.Header.Get("X-Client-Platform")
		device := r.Header.Get("X-Client-Device")
		appVersion := r.Header.Get("X-Client-App-Version")

		log.Printf("req: %s %s platform=%s device=%s app=%s ip=%s",
			r.Method, r.URL.Path, platform, device, appVersion, clientIP(r),
		)

		next.ServeHTTP(w, r)
	})
}

// Generic per-IP rate limiting
var (
	rateMu              sync.Mutex
	rateLastCleanup     time.Time
	rateCleanupInterval = 5 * time.Minute
)

func rateLimitMiddleware(rps int) func(http.Handler) http.Handler {
	window := time.Second

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			now := time.Now()

			rateMu.Lock()

			if rateLastCleanup.IsZero() || now.Sub(rateLastCleanup) > rateCleanupInterval {
				for k, info := range rateData {
					if now.After(info.resetAt.Add(rateCleanupInterval)) {
						delete(rateData, k)
					}
				}
				rateLastCleanup = now
			}

			ri, ok := rateData[ip]
			if !ok || now.After(ri.resetAt) {
				ri = &rateInfo{count: 0, resetAt: now.Add(window)}
				rateData[ip] = ri
			}
			ri.count++
			count := ri.count
			reset := ri.resetAt
			rateMu.Unlock()

			if count > rps {
				w.Header().Set("Retry-After", strconv.Itoa(int(reset.Sub(now).Seconds())))
				writeError(w, http.StatusTooManyRequests, "too many requests")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Login rate limit
var (
	loginRateMu          sync.Mutex
	loginLastSeen        = map[string]time.Time{}
	loginLastCleanup     time.Time
	loginCleanupInterval = 10 * time.Minute
)

func loginRateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		now := time.Now()

		loginRateMu.Lock()

		if loginLastCleanup.IsZero() || now.Sub(loginLastCleanup) > loginCleanupInterval {
			for k, t := range loginLastSeen {
				if now.Sub(t) > loginCleanupInterval {
					delete(loginLastSeen, k)
				}
			}
			loginLastCleanup = now
		}

		last, ok := loginLastSeen[ip]
		if ok && now.Sub(last) < time.Second {
			loginRateMu.Unlock()
			writeError(w, http.StatusTooManyRequests, "too many login attempts")
			return
		}
		loginLastSeen[ip] = now
		loginRateMu.Unlock()

		next.ServeHTTP(w, r)
	})
}

// Playlist creation rate limit
var (
	playlistCreateRateMu          sync.Mutex
	playlistCreateLastSeen        = map[string]time.Time{}
	playlistCreateLastCleanup     time.Time
	playlistCreateCleanupInterval = 10 * time.Minute
)

func playlistCreateRateLimitMiddleware(next http.Handler) http.Handler {
	const window = 5 * time.Second

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		now := time.Now()

		playlistCreateRateMu.Lock()

		if playlistCreateLastCleanup.IsZero() || now.Sub(playlistCreateLastCleanup) > playlistCreateCleanupInterval {
			for k, t := range playlistCreateLastSeen {
				if now.Sub(t) > playlistCreateCleanupInterval {
					delete(playlistCreateLastSeen, k)
				}
			}
			playlistCreateLastCleanup = now
		}

		last, ok := playlistCreateLastSeen[ip]
		if ok && now.Sub(last) < window {
			playlistCreateRateMu.Unlock()
			writeError(w, http.StatusTooManyRequests, "too many playlist creations")
			return
		}
		playlistCreateLastSeen[ip] = now
		playlistCreateRateMu.Unlock()

		next.ServeHTTP(w, r)
	})
}

func getenvInt(key string, def int) int {
	raw := getenv(key, "")
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return def
	}
	return v
}
