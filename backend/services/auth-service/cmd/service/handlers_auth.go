package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthMeResponse struct {
	UserID          string   `json:"userId"`
	Email           string   `json:"email"`
	EmailVerified   bool     `json:"emailVerified"`
	LinkedProviders []string `json:"linkedProviders"`
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
		if errors.Is(err, ErrUserNotFound) || strings.Contains(strings.ToLower(err.Error()), "duplicate key") {
			writeError(w, http.StatusConflict, "email already registered")
			return
		}
		log.Printf("register: createUserWithPassword: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// РАСКОМЕНТИРОВАТЬ БЛОГ КОГДА ЗАКОНЧИТСЯ ДЕВ РАЗРАБОТКА - ЭТО АВТОМАТИЧЕСКАЯ ОТПРАВКА ЕМЕЙЛА ПРИ РЕГИСТРАЦИИ
	token := randomToken(32)
	if err := s.setVerificationToken(r.Context(), user.ID, token); err != nil {
		log.Printf("register: setVerificationToken: %v", err)
	} else {
		// 	log.Printf("[auth-service] email verification for %s: %s", user.Email, verificationURL)
		// 	verificationURL := s.frontendURL + "?mode=verify-email&token=" + token
		s.sendVerificationEmail(user, token)
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

	user, err := s.findUserByID(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "user not found")
		return
	}

	resp := AuthMeResponse{
		UserID:        user.ID,
		Email:         user.Email,
		EmailVerified: user.EmailVerified,
	}
	if user.GoogleID != nil {
		resp.LinkedProviders = append(resp.LinkedProviders, "google")
	}
	if user.FTID != nil {
		resp.LinkedProviders = append(resp.LinkedProviders, "42")
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

	if user.EmailVerified {
		writeError(w, http.StatusBadRequest, "email already verified")
		return
	}

	token := randomToken(32)
	if err := s.setVerificationToken(r.Context(), user.ID, token); err != nil {
		log.Printf("request-email-verification: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	// verificationURL := s.frontendURL + "?mode=verify-email&token=" + token
	// log.Printf("[auth-service] email verification for %s: %s", user.Email, verificationURL)
	s.sendVerificationEmail(user, token)
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
	http.Redirect(w, r, s.frontendBaseURL+"/auth?verification_success=true", http.StatusFound)

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

	// resetURL := s.frontendURL + "?mode=reset-password&token=" + token
	// log.Printf("[auth-service] password reset for %s: %s", user.Email, resetURL)
	s.sendResetPasswordEmail(user, token)
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

func (s *Server) handleLinkProvider(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(ctxClaimsKey{}).(*TokenClaims)
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	provider := chi.URLParam(r, "provider")
	if provider != "google" && provider != "42" {
		writeError(w, http.StatusBadRequest, "invalid provider")
		return
	}

	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// Parse valid token (User B)
	targetClaims := &TokenClaims{}
	token, err := jwt.ParseWithClaims(body.Token, targetClaims, func(t *jwt.Token) (interface{}, error) {
		return s.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		writeError(w, http.StatusBadRequest, "invalid target token")
		return
	}

	if claims.UserID == targetClaims.UserID {
		// Already logged in as same user, nothing to link
		// Just return current profile
		writeJSON(w, http.StatusOK, map[string]string{"status": "already linked"})
		return
	}

	// Find target user (User B)
	targetUser, err := s.findUserByID(r.Context(), targetClaims.UserID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "target user not found")
		return
	}

	// Find current user (User A)
	currentUser, err := s.findUserByID(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "user not found")
		return
	}

	if provider == "google" {
		if targetUser.GoogleID == nil {
			writeError(w, http.StatusBadRequest, "target account has no google link")
			return
		}
		if currentUser.GoogleID != nil {
			writeError(w, http.StatusConflict, "current user already linked to google")
			return
		}
		// Move GoogleID from B to A
		// First: Unlink from B to avoid unique constraint violation
		if _, err := s.updateGoogleID(r.Context(), targetUser.ID, nil); err != nil {
			log.Printf("link google: unlink target: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		// Second: Link to A
		if _, err := s.updateGoogleID(r.Context(), currentUser.ID, targetUser.GoogleID); err != nil {
			log.Printf("link google: link current: %v", err)
			// Attempt to restore? For now just fail. User B is now unlinked but distinct.
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	} else if provider == "42" {
		if targetUser.FTID == nil {
			writeError(w, http.StatusBadRequest, "target account has no 42 link")
			return
		}
		if currentUser.FTID != nil {
			writeError(w, http.StatusConflict, "current user already linked to 42")
			return
		}
		// Move FTID from B to A
		// First: Unlink from B to avoid unique constraint violation
		if _, err := s.updateFTID(r.Context(), targetUser.ID, nil); err != nil {
			log.Printf("link 42: unlink target: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		// Second: Link to A
		if _, err := s.updateFTID(r.Context(), currentUser.ID, targetUser.FTID); err != nil {
			log.Printf("link 42: link current: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	// Delete target user (User B)
	if err := s.deleteUser(r.Context(), targetUser.ID); err != nil {
		// Log but don't fail, since link succeeded
		log.Printf("link provider: failed to delete temp user %s: %v", targetUser.ID, err)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "linked"})
}

func (s *Server) handleUnlinkProvider(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(ctxClaimsKey{}).(*TokenClaims)
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	provider := chi.URLParam(r, "provider")
	if provider != "google" && provider != "42" {
		writeError(w, http.StatusBadRequest, "invalid provider")
		return
	}

	if provider == "google" {
		if _, err := s.updateGoogleID(r.Context(), claims.UserID, nil); err != nil {
			log.Printf("unlink google: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	} else if provider == "42" {
		if _, err := s.updateFTID(r.Context(), claims.UserID, nil); err != nil {
			log.Printf("unlink 42: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "unlinked"})
}
