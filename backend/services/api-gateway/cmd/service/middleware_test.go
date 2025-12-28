package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

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
}

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

func TestRateLimitMiddleware_TooManyRequests(t *testing.T) {
	// reset limiters for deterministic tests
	globalLimiter = newRateLimiter(5 * time.Minute)

	mw := rateLimitMiddleware(1, rateKeyIP, "test")

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

func TestRateLimitMiddleware_ScopedSeparately(t *testing.T) {
	// reset limiters for deterministic tests
	globalLimiter = newRateLimiter(5 * time.Minute)

	mwA := rateLimitMiddleware(1, rateKeyIP, "scopeA")
	mwB := rateLimitMiddleware(1, rateKeyIP, "scopeB")

	calledA := 0
	calledB := 0

	nextA := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledA++
		w.WriteHeader(http.StatusOK)
	})
	nextB := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledB++
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	rrA := httptest.NewRecorder()
	mwA(nextA).ServeHTTP(rrA, req)
	if rrA.Code != http.StatusOK {
		t.Fatalf("scopeA first request expected 200, got %d", rrA.Code)
	}

	rrB := httptest.NewRecorder()
	mwB(nextB).ServeHTTP(rrB, req)
	if rrB.Code != http.StatusOK {
		t.Fatalf("scopeB first request expected 200, got %d", rrB.Code)
	}

	if calledA != 1 || calledB != 1 {
		t.Fatalf("expected each scope to be called once, got A=%d B=%d", calledA, calledB)
	}
}

func TestClientIP_DoesNotTrustXFF_WhenPeerNotTrusted(t *testing.T) {
	pfx := mustPrefixes(t, "10.0.0.0/8") // 127.0.0.1 не trusted
	setTrustedProxyCIDRs(pfx)

	req := httptest.NewRequest("GET", "http://example.com/health", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")

	ip := clientIP(req)
	if ip != "127.0.0.1" {
		t.Fatalf("expected 127.0.0.1, got %s", ip)
	}
}

func TestClientIP_TrustsXFF_WhenPeerTrusted(t *testing.T) {
	pfx := mustPrefixes(t, "127.0.0.1/32")
	setTrustedProxyCIDRs(pfx)

	req := httptest.NewRequest("GET", "http://example.com/health", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")

	ip := clientIP(req)
	if ip != "1.2.3.4" {
		t.Fatalf("expected 1.2.3.4, got %s", ip)
	}
}

func mustPrefixes(t *testing.T, cidrs ...string) []netip.Prefix {
	t.Helper()
	out := make([]netip.Prefix, 0, len(cidrs))
	for _, c := range cidrs {
		p, err := netip.ParsePrefix(c)
		if err != nil {
			t.Fatalf("bad cidr %s: %v", c, err)
		}
		out = append(out, p)
	}
	return out
}
