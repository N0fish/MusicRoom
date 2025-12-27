package main

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestIssueTokens(t *testing.T) {
	secret := []byte("test-secret")
	server := &Server{
		jwtSecret:  secret,
		accessTTL:  15 * time.Minute,
		refreshTTL: 24 * time.Hour,
	}

	user := AuthUser{
		ID:            "user-123",
		Email:         "test@example.com",
		EmailVerified: true,
	}

	tokens, err := server.issueTokens(user)
	if err != nil {
		t.Fatalf("issueTokens failed: %v", err)
	}

	if tokens.AccessToken == "" {
		t.Error("AccessToken is empty")
	}
	if tokens.RefreshToken == "" {
		t.Error("RefreshToken is empty")
	}

	// Verify Access Token
	accessClaims, err := VerifyToken(tokens.AccessToken, secret)
	if err != nil {
		t.Errorf("VerifyToken(AccessToken) failed: %v", err)
	} else {
		if accessClaims.UserID != user.ID {
			t.Errorf("Access Claim UserID = %s, want %s", accessClaims.UserID, user.ID)
		}
		if accessClaims.TokenType != "access" {
			t.Errorf("Access Claim TokenType = %s, want access", accessClaims.TokenType)
		}
	}

	// Verify Refresh Token
	refreshClaims, err := VerifyToken(tokens.RefreshToken, secret)
	if err != nil {
		t.Errorf("VerifyToken(RefreshToken) failed: %v", err)
	} else {
		if refreshClaims.UserID != user.ID {
			t.Errorf("Refresh Claim UserID = %s, want %s", refreshClaims.UserID, user.ID)
		}
		if refreshClaims.TokenType != "refresh" {
			t.Errorf("Refresh Claim TokenType = %s, want refresh", refreshClaims.TokenType)
		}
	}
}

func TestVerifyToken_Table(t *testing.T) {
	secret := []byte("secret")
	otherSecret := []byte("wrong-secret")

	now := time.Now()

	validClaims := TokenClaims{
		UserID:    "u1",
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "u1",
			ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
		},
	}
	validToken, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims).SignedString(secret)

	expiredClaims := TokenClaims{
		UserID:    "u1",
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "u1",
			ExpiresAt: jwt.NewNumericDate(now.Add(-1 * time.Hour)),
		},
	}
	expiredToken, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims).SignedString(secret)

	tests := []struct {
		name      string
		token     string
		secret    []byte
		wantError bool
	}{
		{
			name:      "Valid Token",
			token:     validToken,
			secret:    secret,
			wantError: false,
		},
		{
			name:      "Expired Token",
			token:     expiredToken,
			secret:    secret,
			wantError: true,
		},
		{
			name:      "Wrong Signature",
			token:     validToken,
			secret:    otherSecret,
			wantError: true,
		},
		{
			name:      "Malformated Token",
			token:     "not.a.token",
			secret:    secret,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := VerifyToken(tt.token, tt.secret)
			if (err != nil) != tt.wantError {
				t.Errorf("VerifyToken() error = %v, wantError %v", err, tt.wantError)
			}
			if !tt.wantError && got == nil {
				t.Error("VerifyToken() returned nil claims for valid token")
			}
			if !tt.wantError && got != nil {
				if got.UserID != "u1" {
					t.Errorf("got UserID %s, want u1", got.UserID)
				}
			}
		})
	}
}
