package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
)

func TestHandleRegister(t *testing.T) {
	tests := []struct {
		name           string
		body           interface{}
		setupMock      func(*MockRepository, *MockEmailSender)
		expectedStatus int
	}{
		{
			name: "Success",
			body: Credentials{Email: "new@example.com", Password: "password123"},
			setupMock: func(m *MockRepository, e *MockEmailSender) {
				m.On("CreateUserWithPassword", mock.Anything, "new@example.com", mock.Anything).Return(AuthUser{ID: "new-user", Email: "new@example.com"}, nil)
				m.On("SetVerificationToken", mock.Anything, "new-user", mock.Anything).Return(nil)
				e.On("Send", "new@example.com", mock.Anything, mock.Anything).Return(nil)
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "Existing User",
			body: Credentials{Email: "existing@example.com", Password: "password123"},
			setupMock: func(m *MockRepository, e *MockEmailSender) {
				m.On("CreateUserWithPassword", mock.Anything, "existing@example.com", mock.Anything).
					Return(AuthUser{}, errors.New("duplicate key value violates unique constraint"))
			},
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "Invalid JSON",
			body:           "invalid-json", // passing string to trigger json decode error
			setupMock:      func(m *MockRepository, e *MockEmailSender) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Short Password",
			body:           Credentials{Email: "short@example.com", Password: "123"},
			setupMock:      func(m *MockRepository, e *MockEmailSender) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Repo Create Error",
			body: Credentials{Email: "error@example.com", Password: "password123"},
			setupMock: func(m *MockRepository, e *MockEmailSender) {
				m.On("CreateUserWithPassword", mock.Anything, "error@example.com", mock.Anything).
					Return(AuthUser{}, errors.New("db disconnect"))
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "Token Save Error",
			body: Credentials{Email: "new@example.com", Password: "password123"},
			setupMock: func(m *MockRepository, e *MockEmailSender) {
				m.On("CreateUserWithPassword", mock.Anything, "new@example.com", mock.Anything).Return(AuthUser{ID: "new-user"}, nil)
				m.On("SetVerificationToken", mock.Anything, "new-user", mock.Anything).Return(errors.New("redis error"))
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "Email Send Error",
			body: Credentials{Email: "new@example.com", Password: "password123"},
			setupMock: func(m *MockRepository, e *MockEmailSender) {
				m.On("CreateUserWithPassword", mock.Anything, "new@example.com", mock.Anything).Return(AuthUser{ID: "new-user", Email: "new@example.com"}, nil)
				m.On("SetVerificationToken", mock.Anything, "new-user", mock.Anything).Return(nil)
				e.On("Send", "new@example.com", mock.Anything, mock.Anything).Return(errors.New("smtp error"))
			},
			expectedStatus: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(MockRepository)
			mockEmail := new(MockEmailSender)
			tt.setupMock(repo, mockEmail)

			server := &Server{
				repo:        repo,
				emailSender: mockEmail,
				jwtSecret:   []byte("secret"),
				accessTTL:   time.Minute,
				refreshTTL:  time.Minute,
			}

			var bodyBytes []byte
			if s, ok := tt.body.(string); ok && s == "invalid-json" {
				bodyBytes = []byte("invalid-json")
			} else {
				bodyBytes, _ = json.Marshal(tt.body)
			}

			req := httptest.NewRequest("POST", "/auth/register", bytes.NewBuffer(bodyBytes))
			rec := httptest.NewRecorder()

			server.handleRegister(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}

func TestHandleLogin(t *testing.T) {
	secret := []byte("test-secret")
	password := "password123"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	validUser := AuthUser{
		ID:            "user-123",
		Email:         "test@example.com",
		PasswordHash:  string(hash),
		EmailVerified: true,
	}

	tests := []struct {
		name           string
		body           interface{}
		setupMock      func(*MockRepository)
		expectedStatus int
	}{
		{
			name: "Success",
			body: Credentials{Email: "test@example.com", Password: password},
			setupMock: func(m *MockRepository) {
				m.On("FindUserByEmail", mock.Anything, "test@example.com").Return(validUser, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid JSON",
			body:           "invalid-json",
			setupMock:      func(m *MockRepository) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "User Not Found",
			body: Credentials{Email: "unknown@example.com", Password: "password"},
			setupMock: func(m *MockRepository) {
				m.On("FindUserByEmail", mock.Anything, "unknown@example.com").Return(AuthUser{}, ErrUserNotFound)
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Wrong Password",
			body: Credentials{Email: "test@example.com", Password: "wrong"},
			setupMock: func(m *MockRepository) {
				m.On("FindUserByEmail", mock.Anything, "test@example.com").Return(validUser, nil)
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Repo Error",
			body: Credentials{Email: "test@example.com", Password: "password123"},
			setupMock: func(m *MockRepository) {
				m.On("FindUserByEmail", mock.Anything, "test@example.com").Return(AuthUser{}, errors.New("db disconnect"))
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(MockRepository)
			tt.setupMock(repo)

			server := &Server{
				repo:       repo,
				jwtSecret:  secret,
				accessTTL:  time.Minute,
				refreshTTL: time.Minute,
			}

			var bodyBytes []byte
			if s, ok := tt.body.(string); ok && s == "invalid-json" {
				bodyBytes = []byte("invalid-json")
			} else {
				bodyBytes, _ = json.Marshal(tt.body)
			}

			req := httptest.NewRequest("POST", "/auth/login", bytes.NewBuffer(bodyBytes))
			rec := httptest.NewRecorder()

			server.handleLogin(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}

func TestHandleRefresh(t *testing.T) {
	secret := []byte("test-secret")
	server := &Server{
		jwtSecret:  secret,
		accessTTL:  15 * time.Minute,
		refreshTTL: 24 * time.Hour,
	}

	validUser := AuthUser{ID: "user-123", Email: "test@example.com", EmailVerified: true}
	tokens, _ := server.issueTokens(validUser)

	tests := []struct {
		name           string
		refreshToken   string
		setupMock      func(*MockRepository)
		expectedStatus int
	}{
		{
			name:         "Success",
			refreshToken: tokens.RefreshToken,
			setupMock: func(m *MockRepository) {
				m.On("FindUserByID", mock.Anything, "user-123").Return(validUser, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid Token",
			refreshToken:   "invalid.token.string",
			setupMock:      func(m *MockRepository) {},
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(MockRepository)
			tt.setupMock(repo)
			server.repo = repo

			body := map[string]string{"refreshToken": tt.refreshToken}
			bodyBytes, _ := json.Marshal(body)
			req := httptest.NewRequest("POST", "/auth/refresh", bytes.NewBuffer(bodyBytes))
			rec := httptest.NewRecorder()

			server.handleRefresh(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}

func TestHandleMe(t *testing.T) {
	tests := []struct {
		name           string
		claims         *TokenClaims
		setupMock      func(*MockRepository)
		expectedStatus int
	}{
		{
			name:   "Success",
			claims: &TokenClaims{UserID: "u1", TokenType: "access"},
			setupMock: func(m *MockRepository) {
				user := AuthUser{ID: "u1", Email: "test@example.com"}
				m.On("FindUserByID", mock.Anything, "u1").Return(user, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "User Not Found",
			claims: &TokenClaims{UserID: "u1", TokenType: "access"},
			setupMock: func(m *MockRepository) {
				m.On("FindUserByID", mock.Anything, "u1").Return(AuthUser{}, ErrUserNotFound)
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "No Context Claims",
			claims:         nil,
			setupMock:      func(m *MockRepository) {},
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(MockRepository)
			tt.setupMock(repo)
			server := &Server{repo: repo}

			req := httptest.NewRequest("GET", "/auth/me", nil)
			if tt.claims != nil {
				ctx := context.WithValue(req.Context(), ctxClaimsKey{}, tt.claims)
				req = req.WithContext(ctx)
			}
			rec := httptest.NewRecorder()

			server.handleMe(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}

func TestHandleEmailVerification(t *testing.T) {
	// Request Verification
	t.Run("Request Verification - Success", func(t *testing.T) {
		repo := new(MockRepository)
		emailSender := new(MockEmailSender)
		server := &Server{repo: repo, emailSender: emailSender}

		user := AuthUser{ID: "u1", Email: "test@example.com", EmailVerified: false}

		repo.On("FindUserByEmail", mock.Anything, "test@example.com").Return(user, nil)
		repo.On("SetVerificationToken", mock.Anything, "u1", mock.Anything).Return(nil)
		emailSender.On("Send", "test@example.com", mock.Anything, mock.Anything).Return(nil)

		body := map[string]string{"email": "test@example.com"}
		bodyBytes, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/auth/request-email-verification", bytes.NewBuffer(bodyBytes))
		rec := httptest.NewRecorder()

		server.handleRequestEmailVerification(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Want 200, got %d", rec.Code)
		}
	})

	// Verify Email
	t.Run("Verify Email - Success", func(t *testing.T) {
		repo := new(MockRepository)
		server := &Server{repo: repo, frontendBaseURL: "http://front"}

		user := AuthUser{ID: "u1", Email: "test@example.com", EmailVerified: true}
		repo.On("VerifyEmailByToken", mock.Anything, "valid-token").Return(user, nil)

		req := httptest.NewRequest("GET", "/auth/verify-email?token=valid-token", nil)
		rec := httptest.NewRecorder()

		server.handleVerifyEmail(rec, req)

		if rec.Code != http.StatusFound {
			t.Errorf("Want 302, got %d", rec.Code)
		}
	})
}

func TestHandlePasswordReset(t *testing.T) {
	// Forgot Password
	t.Run("Forgot Password - Success", func(t *testing.T) {
		repo := new(MockRepository)
		emailSender := new(MockEmailSender)
		server := &Server{repo: repo, emailSender: emailSender}

		user := AuthUser{ID: "u1", Email: "test@example.com"}
		repo.On("FindUserByEmail", mock.Anything, "test@example.com").Return(user, nil)
		repo.On("SetResetToken", mock.Anything, "u1", mock.Anything, mock.Anything).Return(nil)
		emailSender.On("Send", "test@example.com", mock.Anything, mock.Anything).Return(nil)

		body := map[string]string{"email": "test@example.com"}
		bodyBytes, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/auth/forgot-password", bytes.NewBuffer(bodyBytes))
		rec := httptest.NewRecorder()

		server.handleForgotPassword(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Want 200, got %d", rec.Code)
		}
	})

	// Reset Password
	t.Run("Reset Password - Success", func(t *testing.T) {
		repo := new(MockRepository)
		server := &Server{repo: repo}

		user := AuthUser{ID: "u1", Email: "test@example.com"}
		repo.On("ResetPasswordByToken", mock.Anything, "valid-token", mock.Anything, mock.Anything).
			Return(user, nil)

		body := map[string]string{"token": "valid-token", "newPassword": "newpassword123"}
		bodyBytes, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/auth/reset-password", bytes.NewBuffer(bodyBytes))
		rec := httptest.NewRecorder()

		server.handleResetPassword(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Want 200, got %d", rec.Code)
		}
	})
}

func TestHandleLinkProvider(t *testing.T) {
	secret := []byte("secret")

	createToken := func(userID string, srv *Server) string {
		tokens, _ := srv.issueTokens(AuthUser{ID: userID})
		return tokens.AccessToken
	}

	tests := []struct {
		name            string
		provider        string
		targetUserAge   time.Duration
		targetGoogleID  *string
		targetFTID      *string
		currentGoogleID *string
		currentFTID     *string
		currentUser     string
		targetUser      string
		bodyOverride    interface{}
		mockSetup       func(*MockRepository)
		expectedStatus  int
	}{
		{
			name:            "Success Link Google",
			provider:        "google",
			targetUserAge:   1 * time.Minute,
			targetGoogleID:  strPtr("g-123"),
			currentGoogleID: nil,
			currentUser:     "user-A",
			targetUser:      "user-B",
			mockSetup:       nil, // default success flow treated in body
			expectedStatus:  http.StatusOK,
		},
		{
			name:           "Success Link 42",
			provider:       "42",
			targetUserAge:  1 * time.Minute,
			targetFTID:     strPtr("ft-123"),
			currentFTID:    nil,
			currentUser:    "user-A",
			targetUser:     "user-B",
			mockSetup:      nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Conflict - Target Old",
			provider:       "google",
			targetUserAge:  10 * time.Minute,
			targetGoogleID: strPtr("g-123"),
			currentUser:    "user-A",
			targetUser:     "user-B",
			mockSetup:      nil,
			expectedStatus: http.StatusConflict,
		},
		{
			name:            "Conflict - Current User Already Linked Google",
			provider:        "google",
			targetUserAge:   1 * time.Minute,
			targetGoogleID:  strPtr("g-123"),
			currentGoogleID: strPtr("g-old"),
			currentUser:     "user-A",
			targetUser:      "user-B",
			mockSetup:       nil,
			expectedStatus:  http.StatusConflict,
		},
		{
			name:           "Conflict - Current User Already Linked 42",
			provider:       "42",
			targetUserAge:  1 * time.Minute,
			targetFTID:     strPtr("ft-123"),
			currentFTID:    strPtr("ft-old"),
			currentUser:    "user-A",
			targetUser:     "user-B",
			mockSetup:      nil,
			expectedStatus: http.StatusConflict,
		},
		{
			name:            "Already Linked (Same User)",
			provider:        "google",
			targetUserAge:   1 * time.Minute,
			targetGoogleID:  strPtr("g-123"),
			currentGoogleID: strPtr("g-123"),
			currentUser:     "user-A",
			targetUser:      "user-A",
			mockSetup:       nil,
			expectedStatus:  http.StatusOK,
		},
		{
			name:           "Invalid Provider",
			provider:       "yahoo",
			currentUser:    "user-A",
			targetUser:     "user-B",
			targetUserAge:  1 * time.Minute,
			mockSetup:      func(m *MockRepository) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:          "Target User Not Found",
			provider:      "google",
			currentUser:   "user-A",
			targetUser:    "user-B",
			targetUserAge: 1 * time.Minute,
			mockSetup: func(m *MockRepository) {
				m.On("FindUserByID", mock.Anything, "user-B").Return(AuthUser{}, ErrUserNotFound)
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:          "Current User Not Found",
			provider:      "google",
			currentUser:   "user-A",
			targetUser:    "user-B",
			targetUserAge: 1 * time.Minute,
			mockSetup: func(m *MockRepository) {
				m.On("FindUserByID", mock.Anything, "user-B").Return(AuthUser{ID: "user-B", CreatedAt: time.Now()}, nil)
				m.On("FindUserByID", mock.Anything, "user-A").Return(AuthUser{}, ErrUserNotFound)
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Target Has No Link (Google)",
			provider:       "google",
			currentUser:    "user-A",
			targetUser:     "user-B",
			targetUserAge:  1 * time.Minute,
			targetGoogleID: nil,
			mockSetup:      nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Target Has No Link (42)",
			provider:       "42",
			currentUser:    "user-A",
			targetUser:     "user-B",
			targetUserAge:  1 * time.Minute,
			targetFTID:     nil,
			mockSetup:      nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "DB Error - Unlink Target Fail",
			provider:       "google",
			currentUser:    "user-A",
			targetUser:     "user-B",
			targetUserAge:  1 * time.Minute,
			targetGoogleID: strPtr("g-123"),
			mockSetup: func(m *MockRepository) {
				m.On("FindUserByID", mock.Anything, "user-B").Return(AuthUser{ID: "user-B", CreatedAt: time.Now(), GoogleID: strPtr("g-123")}, nil)
				m.On("FindUserByID", mock.Anything, "user-A").Return(AuthUser{ID: "user-A", CreatedAt: time.Now()}, nil)
				m.On("UpdateGoogleID", mock.Anything, "user-B", (*string)(nil)).Return(AuthUser{}, errors.New("db error"))
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(MockRepository)
			server := &Server{repo: repo, jwtSecret: secret, accessTTL: time.Hour}

			userA := AuthUser{ID: tt.currentUser, Email: "a@test.com", GoogleID: tt.currentGoogleID, FTID: tt.currentFTID, CreatedAt: time.Now()}
			userB := AuthUser{ID: tt.targetUser, Email: "b@test.com", GoogleID: tt.targetGoogleID, FTID: tt.targetFTID, CreatedAt: time.Now().Add(-tt.targetUserAge)}

			mockCtx := mock.Anything

			if tt.mockSetup != nil {
				tt.mockSetup(repo)
			} else {
				if tt.provider == "google" || tt.provider == "42" {
					if tt.currentUser != tt.targetUser {
						repo.On("FindUserByID", mockCtx, tt.targetUser).Return(userB, nil).Maybe()
						repo.On("FindUserByID", mockCtx, tt.currentUser).Return(userA, nil).Maybe()
					}
				}

				if tt.expectedStatus == http.StatusOK && tt.currentUser != tt.targetUser {
					if tt.provider == "google" {
						repo.On("UpdateGoogleID", mockCtx, tt.targetUser, (*string)(nil)).Return(userB, nil)
						repo.On("UpdateGoogleID", mockCtx, tt.currentUser, tt.targetGoogleID).Return(userA, nil)
						repo.On("DeleteUser", mockCtx, tt.targetUser).Return(nil)
					}
					if tt.provider == "42" {
						repo.On("UpdateFTID", mockCtx, tt.targetUser, (*string)(nil)).Return(userB, nil)
						repo.On("UpdateFTID", mockCtx, tt.currentUser, tt.targetFTID).Return(userA, nil)
						repo.On("DeleteUser", mockCtx, tt.targetUser).Return(nil)
					}
				}
			}

			var bodyBytes []byte
			if tt.bodyOverride != nil {
				if s, ok := tt.bodyOverride.(string); ok && s == "invalid-json" {
					bodyBytes = []byte("invalid-json")
				} else {
					bodyBytes, _ = json.Marshal(tt.bodyOverride)
				}
			} else {
				targetToken := createToken(tt.targetUser, server)
				body := map[string]string{"token": targetToken}
				bodyBytes, _ = json.Marshal(body)
			}

			req := httptest.NewRequest("POST", "/auth/link/"+tt.provider, bytes.NewBuffer(bodyBytes))

			claims := &TokenClaims{UserID: tt.currentUser, TokenType: "access"}
			ctx := context.WithValue(req.Context(), ctxClaimsKey{}, claims)
			req = req.WithContext(ctx)

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("provider", tt.provider)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			rec := httptest.NewRecorder()
			server.handleLinkProvider(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}

func TestHandleUnlinkProvider(t *testing.T) {
	tests := []struct {
		name           string
		provider       string
		mockSetup      func(*MockRepository)
		expectedStatus int
	}{
		{
			name:     "Unlink Google - Success",
			provider: "google",
			mockSetup: func(m *MockRepository) {
				m.On("UpdateGoogleID", mock.Anything, "u1", (*string)(nil)).Return(AuthUser{}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:     "Unlink 42 - Success",
			provider: "42",
			mockSetup: func(m *MockRepository) {
				m.On("UpdateFTID", mock.Anything, "u1", (*string)(nil)).Return(AuthUser{}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:     "Invalid Provider",
			provider: "yahoo",
			mockSetup: func(m *MockRepository) {
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:     "DB Error",
			provider: "google",
			mockSetup: func(m *MockRepository) {
				m.On("UpdateGoogleID", mock.Anything, "u1", (*string)(nil)).Return(AuthUser{}, errors.New("db error"))
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(MockRepository)
			server := &Server{repo: repo, jwtSecret: []byte("s")}
			tt.mockSetup(repo)

			req := httptest.NewRequest("DELETE", "/auth/link/"+tt.provider, nil)
			claims := &TokenClaims{UserID: "u1", TokenType: "access"}
			ctx := context.WithValue(req.Context(), ctxClaimsKey{}, claims)
			req = req.WithContext(ctx)

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("provider", tt.provider)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			rec := httptest.NewRecorder()
			server.handleUnlinkProvider(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}

func strPtr(s string) *string {
	return &s
}
