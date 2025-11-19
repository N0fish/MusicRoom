package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthMeResponse struct {
	UserID        string `json:"userId"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"emailVerified"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var creds Credentials
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	email := strings.TrimSpace(strings.ToLower(creds.Email))
	if email == "" || creds.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}
	if len(creds.Password) < 6 {
		writeError(w, http.StatusBadRequest, "password must be at least 6 characters")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(creds.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("register: hash error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	user, err := s.createUserWithPassword(r.Context(), email, string(hash))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate key") {
			writeError(w, http.StatusConflict, "email already registered")
			return
		}
		log.Printf("register: createUserWithPassword: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	token := randomToken(32)
	if err := s.setVerificationToken(r.Context(), user.ID, token); err != nil {
		log.Printf("register: setVerificationToken: %v", err)
	} else {
		verificationURL := s.frontendURL + "?mode=verify-email&token=" + token
		log.Printf("[auth-service] email verification for %s: %s", user.Email, verificationURL)
	}

	tokens, err := s.issueTokens(user)
	if err != nil {
		log.Printf("register: issueTokens: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, tokens)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var creds Credentials
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	email := strings.TrimSpace(strings.ToLower(creds.Email))
	if email == "" || creds.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	user, err := s.findUserByEmail(r.Context(), email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		log.Printf("login: findUserByEmail: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if user.PasswordHash == "" {
		writeError(w, http.StatusBadRequest, "password login not available for this account")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(creds.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	tokens, err := s.issueTokens(user)
	if err != nil {
		log.Printf("login: issueTokens: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, tokens)
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "refreshToken is required")
		return
	}

	claims := &TokenClaims{}
	token, err := jwt.ParseWithClaims(body.RefreshToken, claims, func(t *jwt.Token) (interface{}, error) {
		return s.jwtSecret, nil
	})
	if err != nil || !token.Valid || claims.TokenType != "refresh" {
		writeError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	user, err := s.findUserByID(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "user not found")
		return
	}

	tokens, err := s.issueTokens(user)
	if err != nil {
		log.Printf("refresh: issueTokens: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, tokens)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(ctxClaimsKey{}).(*TokenClaims)
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	resp := AuthMeResponse{
		UserID:        claims.UserID,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleRequestEmailVerification(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}
	email := strings.TrimSpace(strings.ToLower(body.Email))

	user, err := s.findUserByEmail(r.Context(), email)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	token := randomToken(32)
	if err := s.setVerificationToken(r.Context(), user.ID, token); err != nil {
		log.Printf("request-email-verification: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	verificationURL := s.frontendURL + "?mode=verify-email&token=" + token
	log.Printf("[auth-service] email verification for %s: %s", user.Email, verificationURL)
	writeJSON(w, http.StatusOK, map[string]string{"status": "verification sent"})
}

func (s *Server) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}
	user, err := s.verifyEmailByToken(r.Context(), token)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			writeError(w, http.StatusBadRequest, "invalid token")
			return
		}
		log.Printf("verify-email: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	log.Printf("[auth-service] email verified for %s", user.Email)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "email verified",
		"emailVerified": true,
	})
}

func (s *Server) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}
	email := strings.TrimSpace(strings.ToLower(body.Email))

	user, err := s.findUserByEmail(r.Context(), email)
	if err != nil {
		// Do not reveal whether user exists
		writeJSON(w, http.StatusOK, map[string]string{"status": "reset link sent"})
		return
	}

	token := randomToken(32)
	expiresAt := time.Now().Add(1 * time.Hour)
	if err := s.setResetToken(r.Context(), user.ID, token, expiresAt); err != nil {
		log.Printf("forgot-password: %v", err)
		writeJSON(w, http.StatusOK, map[string]string{"status": "reset link sent"})
		return
	}

	resetURL := s.frontendURL + "?mode=reset-password&token=" + token
	log.Printf("[auth-service] password reset for %s: %s", user.Email, resetURL)
	writeJSON(w, http.StatusOK, map[string]string{"status": "reset link sent"})
}

func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token       string `json:"token"`
		NewPassword string `json:"newPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Token == "" || body.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "token and newPassword are required")
		return
	}
	if len(body.NewPassword) < 6 {
		writeError(w, http.StatusBadRequest, "password must be at least 6 characters")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("reset-password: hash error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	user, err := s.resetPasswordByToken(r.Context(), body.Token, string(hash), time.Now())
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			writeError(w, http.StatusBadRequest, "invalid or expired token")
			return
		}
		log.Printf("reset-password: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	log.Printf("[auth-service] password reset for %s", user.Email)
	writeJSON(w, http.StatusOK, map[string]string{"status": "password updated"})
}

type ctxClaimsKey struct{}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
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
			return s.jwtSecret, nil
		})
		if err != nil || !token.Valid || claims.TokenType != "access" {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		ctx := context.WithValue(r.Context(), ctxClaimsKey{}, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
