package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestAuthMiddleware(t *testing.T) {
	secret := []byte("test-secret")
	repo := new(MockRepository)
	server := &Server{
		repo:      repo,
		jwtSecret: secret,
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(ctxClaimsKey{}).(*TokenClaims)
		if !ok || claims == nil {
			t.Error("AuthMiddleware did not set claims in context")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if claims.UserID != "user-123" {
			t.Errorf("Ctx UserID = %s, want user-123", claims.UserID)
		}
		w.WriteHeader(http.StatusOK)
	})

	middleware := server.authMiddleware(nextHandler)

	t.Run("Missing Header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Want 401, got %d", rec.Code)
		}
	})

	t.Run("Invalid Header Format", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "InvalidFormat")
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Want 401, got %d", rec.Code)
		}
	})

	t.Run("Invalid Token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Want 401, got %d", rec.Code)
		}
	})

	t.Run("Valid Token", func(t *testing.T) {
		now := time.Now()
		claims := TokenClaims{
			UserID:    "user-123",
			TokenType: "access",
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "user-123",
				ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
			},
		}
		token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Want 200, got %d", rec.Code)
		}
	})

	t.Run("Expired Token", func(t *testing.T) {
		now := time.Now()
		claims := TokenClaims{
			UserID:    "user-123",
			TokenType: "access",
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "user-123",
				ExpiresAt: jwt.NewNumericDate(now.Add(-1 * time.Hour)),
			},
		}
		token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Want 401, got %d", rec.Code)
		}
	})

	t.Run("Wrong Token Type", func(t *testing.T) {
		now := time.Now()
		claims := TokenClaims{
			UserID:    "user-123",
			TokenType: "refresh", // Wrong type
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "user-123",
				ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
			},
		}
		token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Want 401, got %d", rec.Code)
		}
	})
}
