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

// JWT
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
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrTokenSignatureInvalid
				}
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

func jwtAuthOptionalMiddleware(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				next.ServeHTTP(w, r)
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
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrTokenSignatureInvalid
				}
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

// CORS
func corsMiddleware(next http.Handler) http.Handler {
	allowedOrigin := getenv("CORS_ALLOWED_ORIGIN", "*")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		if allowedOrigin == "*" {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		} else if origin == allowedOrigin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		} else if origin == "" {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		}
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")

		if strings.ToUpper(r.Method) == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Body size
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

// Logs
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

// Rate limiting
type rateInfo struct {
	count   int
	resetAt time.Time
}

type rateLimiter struct {
	mu              sync.Mutex
	data            map[string]*rateInfo
	lastCleanup     time.Time
	cleanupInterval time.Duration
}

func newRateLimiter(cleanupInterval time.Duration) *rateLimiter {
	return &rateLimiter{
		data:            map[string]*rateInfo{},
		cleanupInterval: cleanupInterval,
	}
}

func (l *rateLimiter) allow(key string, window time.Duration, limit int) (ok bool, retryAfterSeconds int) {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.lastCleanup.IsZero() || now.Sub(l.lastCleanup) > l.cleanupInterval {
		for k, info := range l.data {
			if now.After(info.resetAt.Add(l.cleanupInterval)) {
				delete(l.data, k)
			}
		}
		l.lastCleanup = now
	}

	ri, exists := l.data[key]
	if !exists || now.After(ri.resetAt) {
		ri = &rateInfo{count: 0, resetAt: now.Add(window)}
		l.data[key] = ri
	}

	ri.count++
	if ri.count > limit {
		sec := int(ri.resetAt.Sub(now).Seconds())
		if sec < 0 {
			sec = 0
		}
		return false, sec
	}
	return true, 0
}

var (
	globalLimiter   = newRateLimiter(5 * time.Minute)
	loginLimiter    = newRateLimiter(10 * time.Minute)
	playlistLimiter = newRateLimiter(10 * time.Minute)
)

func rateKeyUserOrIP(r *http.Request) string {
	if uid := strings.TrimSpace(r.Header.Get("X-User-Id")); uid != "" {
		return "u:" + uid
	}
	return "ip:" + clientIP(r)
}

func rateKeyIP(r *http.Request) string {
	return "ip:" + clientIP(r)
}

func rateLimitMiddleware(rps int, keyFn func(*http.Request) string, scope string) func(http.Handler) http.Handler {
	window := time.Second
	lim := globalLimiter

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := scope + ":" + keyFn(r)
			ok, retry := lim.allow(key, window, rps)
			if !ok {
				w.Header().Set("Retry-After", strconv.Itoa(retry))
				writeError(w, http.StatusTooManyRequests, "too many requests")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func loginRateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ok, _ := loginLimiter.allow(rateKeyIP(r), time.Second, 1)
		if !ok {
			writeError(w, http.StatusTooManyRequests, "too many login attempts")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func playlistCreateRateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ok, _ := playlistLimiter.allow(rateKeyUserOrIP(r), 5*time.Second, 1)
		if !ok {
			writeError(w, http.StatusTooManyRequests, "too many playlist creations")
			return
		}
		next.ServeHTTP(w, r)
	})
}
