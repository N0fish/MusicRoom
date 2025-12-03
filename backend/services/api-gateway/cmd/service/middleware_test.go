package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	// main "/cmd/services/"

	"github.com/golang-jwt/jwt/v5"
)

// ---------- helpers ----------

func makeTestAccessToken(t *testing.T, secret []byte, userID string) string {
	t.Helper()
	claims := &TokenClaims{
		UserID:    userID,
		Email:     "user@example.com",
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return signed
}

// ---------- jwtAuthMiddleware ----------

func TestJWTAuthMiddleware_ValidToken(t *testing.T) {
	secret := []byte("test-secret")
	token := makeTestAccessToken(t, secret, "user-123")

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true

		if got := r.Header.Get("X-User-Id"); got != "user-123" {
			t.Errorf("expected X-User-Id=user-123, got %q", got)
		}
		if got := r.Header.Get("X-User-Email"); got != "user@example.com" {
			t.Errorf("expected X-User-Email=user@example.com, got %q", got)
		}

		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	mw := jwtAuthMiddleware(secret)
	mw(next).ServeHTTP(rr, req)

	if !called {
		t.Fatalf("next handler was not called")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
}

func TestJWTAuthMiddleware_MissingHeader(t *testing.T) {
	secret := []byte("test-secret")

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("next handler must not be called when Authorization is missing")
	})

	req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	rr := httptest.NewRecorder()

	mw := jwtAuthMiddleware(secret)
	mw(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rr.Code)
	}
}

func TestJWTAuthMiddleware_InvalidType(t *testing.T) {
	secret := []byte("test-secret")

	claims := &TokenClaims{
		UserID:    "user-123",
		Email:     "user@example.com",
		TokenType: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("next handler must not be called for non-access token")
	})

	req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	rr := httptest.NewRecorder()

	mw := jwtAuthMiddleware(secret)
	mw(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rr.Code)
	}
}

// ---------- corsMiddleware ----------

func TestCORSMiddleware_OptionsPreflight(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGIN", "http://localhost:5175")

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	req := httptest.NewRequest(http.MethodOptions, "/playlists", nil)
	req.Header.Set("Origin", "http://localhost:5175")

	rr := httptest.NewRecorder()

	mw := corsMiddleware(next)
	mw.ServeHTTP(rr, req)

	if nextCalled {
		t.Fatalf("next handler must not be called for OPTIONS preflight")
	}

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rr.Code)
	}

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5175" {
		t.Errorf("expected Allow-Origin=http://localhost:5175, got %q", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Errorf("expected Access-Control-Allow-Headers to be set")
	}
	if got := rr.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Errorf("expected Access-Control-Allow-Methods to be set")
	}
}

func TestCORSMiddleware_NormalRequest(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGIN", "http://localhost:5175")

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "http://localhost:5175")

	rr := httptest.NewRecorder()

	mw := corsMiddleware(next)
	mw.ServeHTTP(rr, req)

	if !nextCalled {
		t.Fatalf("next handler was not called for normal request")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5175" {
		t.Errorf("expected Allow-Origin=http://localhost:5175, got %q", got)
	}
}

// ---------- bodySizeLimitMiddleware ----------

func TestBodySizeLimitMiddleware_TooLarge(t *testing.T) {
	const limit = int64(10)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("next handler must not be called when body is too large")
	})

	body := bytes.NewBufferString("small body")
	req := httptest.NewRequest(http.MethodPost, "/users/me", body)
	req.ContentLength = limit + 1

	rr := httptest.NewRecorder()

	mw := bodySizeLimitMiddleware(limit)
	mw(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413, got %d", rr.Code)
	}
}

func TestBodySizeLimitMiddleware_Allowed(t *testing.T) {
	const limit = int64(1024)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	body := bytes.NewBufferString("ok")
	req := httptest.NewRequest(http.MethodPost, "/users/me", body)
	req.ContentLength = int64(body.Len())

	rr := httptest.NewRecorder()

	mw := bodySizeLimitMiddleware(limit)
	mw(next).ServeHTTP(rr, req)

	if !called {
		t.Fatalf("next handler was not called for allowed body size")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
}

// ---------- rateLimitMiddleware (global per-IP) ----------

func TestRateLimitMiddleware_TooManyRequests(t *testing.T) {
	rateMu.Lock()
	rateData = map[string]*rateInfo{}
	rateLastCleanup = time.Time{}
	rateMu.Unlock()

	mw := rateLimitMiddleware(1)

	calledCount := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledCount++
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr1 := httptest.NewRecorder()
	rr2 := httptest.NewRecorder()

	mw(next).ServeHTTP(rr1, req)
	if rr1.Code != http.StatusOK {
		t.Fatalf("first request expected 200, got %d", rr1.Code)
	}

	mw(next).ServeHTTP(rr2, req)
	if rr2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request expected 429, got %d", rr2.Code)
	}

	if calledCount != 1 {
		t.Fatalf("next handler should be called once, got %d", calledCount)
	}
}
