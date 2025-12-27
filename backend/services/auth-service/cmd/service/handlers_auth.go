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

	user, err := s.repo.CreateUserWithPassword(r.Context(), email, string(hash))
	if err != nil {
		if errors.Is(err, ErrUserNotFound) || strings.Contains(strings.ToLower(err.Error()), "duplicate key") {
			writeError(w, http.StatusConflict, "email already registered")
			return
		}
		log.Printf("register: createUserWithPassword: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	token := randomToken(32)
	if err := s.repo.SetVerificationToken(r.Context(), user.ID, token); err != nil {
		log.Printf("register: setVerificationToken: %v", err)
	} else {
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

	user, err := s.repo.FindUserByEmail(r.Context(), email)
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

	user, err := s.repo.FindUserByID(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "user not found")
		return
	}

	if user.TokenVersion != claims.Version {
		writeError(w, http.StatusUnauthorized, "token revoked")
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

	user, err := s.repo.FindUserByID(r.Context(), claims.UserID)
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

	user, err := s.repo.FindUserByEmail(r.Context(), email)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	if user.EmailVerified {
		writeError(w, http.StatusBadRequest, "email already verified")
		return
	}

	token := randomToken(32)
	if err := s.repo.SetVerificationToken(r.Context(), user.ID, token); err != nil {
		log.Printf("request-email-verification: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.sendVerificationEmail(user, token)
	writeJSON(w, http.StatusOK, map[string]string{"status": "verification sent"})
}

func (s *Server) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}
	user, err := s.repo.VerifyEmailByToken(r.Context(), token)
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

	user, err := s.repo.FindUserByEmail(r.Context(), email)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "reset link sent"})
		return
	}

	token := randomToken(32)
	expiresAt := time.Now().Add(1 * time.Hour)
	if err := s.repo.SetResetToken(r.Context(), user.ID, token, expiresAt); err != nil {
		log.Printf("forgot-password: %v", err)
		writeJSON(w, http.StatusOK, map[string]string{"status": "reset link sent"})
		return
	}

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

	user, err := s.repo.ResetPasswordByToken(r.Context(), body.Token, string(hash), time.Now())
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

		user, err := s.repo.FindUserByID(r.Context(), claims.UserID)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		if user.TokenVersion != claims.Version {
			writeError(w, http.StatusUnauthorized, "token revoked")
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
	if strings.TrimSpace(body.Token) == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}

	targetClaims := &TokenClaims{}
	token, err := jwt.ParseWithClaims(body.Token, targetClaims, func(t *jwt.Token) (interface{}, error) {
		return s.jwtSecret, nil
	})
	if err != nil || !token.Valid || targetClaims.TokenType != "access" {
		writeError(w, http.StatusBadRequest, "invalid target token")
		return
	}

	if claims.UserID == targetClaims.UserID {
		writeJSON(w, http.StatusOK, map[string]string{"status": "already linked"})
		return
	}

	// Find target user (User B)
	targetUser, err := s.repo.FindUserByID(r.Context(), targetClaims.UserID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "target user not found")
		return
	}
	if targetUser.TokenVersion != targetClaims.Version {
		writeError(w, http.StatusBadRequest, "invalid target token")
		return
	}

	// Find current user (User A)
	currentUser, err := s.repo.FindUserByID(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "user not found")
		return
	}

	if time.Since(targetUser.CreatedAt) > 5*time.Minute {
		writeError(w, http.StatusConflict, "account associated with another user")
		return
	}

	// IMPORTANT: make the move atomic when possible to avoid leaving the system
	// in an inconsistent state (e.g. provider unlinked from B but not linked to A).
	//
	// In unit tests we use a MockRepository without transaction support; in that
	// case we just run the same logic without a transaction.
	var runInTx = func(ctx context.Context, fn func(repo Repository) error) error {
		if txCapable, ok := s.repo.(interface {
			WithTx(context.Context, func(Repository) error) error
		}); ok {
			return txCapable.WithTx(ctx, fn)
		}
		return fn(s.repo)
	}

	if err := runInTx(r.Context(), func(txRepo Repository) error {
		if provider == "google" {
			if targetUser.GoogleID == nil {
				return errors.New("target account has no google link")
			}
			if currentUser.GoogleID != nil {
				return errors.New("current user already linked to google")
			}
			if _, err := txRepo.UpdateGoogleID(r.Context(), targetUser.ID, nil); err != nil {
				return err
			}
			if _, err := txRepo.UpdateGoogleID(r.Context(), currentUser.ID, targetUser.GoogleID); err != nil {
				return err
			}
		} else {
			if targetUser.FTID == nil {
				return errors.New("target account has no 42 link")
			}
			if currentUser.FTID != nil {
				return errors.New("current user already linked to 42")
			}
			if _, err := txRepo.UpdateFTID(r.Context(), targetUser.ID, nil); err != nil {
				return err
			}
			if _, err := txRepo.UpdateFTID(r.Context(), currentUser.ID, targetUser.FTID); err != nil {
				return err
			}
		}
		// Delete target user (User B)
		return txRepo.DeleteUser(r.Context(), targetUser.ID)
	}); err != nil {
		// Map expected conflicts to HTTP codes; everything else is a 500.
		msg := strings.ToLower(err.Error())
		switch {
		case strings.Contains(msg, "no google") || strings.Contains(msg, "no 42"):
			writeError(w, http.StatusBadRequest, err.Error())
			return
		case strings.Contains(msg, "already linked"):
			writeError(w, http.StatusConflict, err.Error())
			return
		default:
			log.Printf("link provider: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
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

	user, err := s.repo.FindUserByID(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "user not found")
		return
	}

	if user.PasswordHash == "" {
		remainingGoogle := user.GoogleID != nil
		remaining42 := user.FTID != nil

		if provider == "google" {
			remainingGoogle = false
		} else {
			remaining42 = false
		}

		if !remainingGoogle && !remaining42 {
			writeError(w, http.StatusConflict, "cannot unlink last login method")
			return
		}
	}

	// Собственно unlink
	if provider == "google" {
		if _, err := s.repo.UpdateGoogleID(r.Context(), claims.UserID, nil); err != nil {
			log.Printf("unlink google: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	} else {
		if _, err := s.repo.UpdateFTID(r.Context(), claims.UserID, nil); err != nil {
			log.Printf("unlink 42: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "unlinked"})
}
