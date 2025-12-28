package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestHealthCheck(t *testing.T) {
	cfg := Config{
		RateLimitRPS: 100,
		JWTSecret:    []byte("x"),
	}
	r := setupRouter(cfg)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func makeAccessTokenForTest(t *testing.T, secret []byte, userID string) string {
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

func TestAuthMe_RequiresJWT_AndForwardsHeaders(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-User-Id") != "user-123" {
			t.Fatalf("expected X-User-Id=user-123, got %q", r.Header.Get("X-User-Id"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	secret := []byte("test-secret")
	cfg := Config{
		AuthURL:          backend.URL,
		UserURL:          backend.URL,
		PlaylistURL:      backend.URL,
		VoteURL:          backend.URL,
		MockURL:          backend.URL,
		RealtimeURL:      backend.URL,
		MusicProviderURL: backend.URL,
		RateLimitRPS:     1000,
		JWTSecret:        secret,
	}
	r := setupRouter(cfg)

	// 1) without JWT -> 401
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	// 2) with valid JWT -> 200
	token := makeAccessTokenForTest(t, secret, "user-123")
	req = httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestStaticDocs(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	content := "openapi: 3.0.0"
	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		OpenAPIFile:  tmpfile.Name(),
		RateLimitRPS: 100,
		JWTSecret:    []byte("x"),
	}
	r := setupRouter(cfg)

	req := httptest.NewRequest(http.MethodGet, "/docs/openapi.yaml", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if w.Body.String() != content {
		t.Errorf("expected content %q, got %q", content, w.Body.String())
	}
}

func TestProxyErrorHandling(t *testing.T) {
	cfg := Config{
		AuthURL:      "http://localhost:12345",
		RateLimitRPS: 100,
		JWTSecret:    []byte("x"),
	}
	r := setupRouter(cfg)

	req := httptest.NewRequest(http.MethodPost, "/auth/register", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected status 502, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] != "upstream service unavailable" {
		t.Errorf("unexpected error message: %s", resp["error"])
	}
}
