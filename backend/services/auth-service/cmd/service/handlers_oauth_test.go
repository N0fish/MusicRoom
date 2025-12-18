package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func mockResponse(statusCode int, body interface{}) *http.Response {
	var bodyReader io.ReadCloser
	if s, ok := body.(string); ok {
		bodyReader = io.NopCloser(strings.NewReader(s))
	} else {
		b, _ := json.Marshal(body)
		bodyReader = io.NopCloser(bytes.NewReader(b))
	}
	return &http.Response{
		StatusCode: statusCode,
		Body:       bodyReader,
	}
}

func TestHandleGoogleLogin(t *testing.T) {
	mockRepo := new(MockRepository)
	mockEmail := new(MockEmailSender)
	mockHTTP := new(MockHTTPClient)
	srv := &Server{
		repo:        mockRepo,
		emailSender: mockEmail,
		httpClient:  mockHTTP,
		googleCfg: GoogleConfig{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			RedirectURL:  "http://localhost:3000/callback",
		},
		frontendURL: "http://frontend",
	}

	req := httptest.NewRequest("GET", "/auth/google/login?redirect=http://custom-redirect", nil)
	w := httptest.NewRecorder()

	srv.handleGoogleLogin(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusFound, resp.StatusCode)
	loc, _ := resp.Location()

	assert.Equal(t, "https", loc.Scheme)
	assert.Equal(t, "accounts.google.com", loc.Host)
	assert.Equal(t, "/o/oauth2/v2/auth", loc.Path)

	q := loc.Query()
	assert.Equal(t, "test-client-id", q.Get("client_id"))
	assert.Equal(t, "http://localhost:3000/callback", q.Get("redirect_uri"))
	// redirect param is state encoded
	assert.Equal(t, url.QueryEscape("http://custom-redirect"), q.Get("state"))
}

func TestHandleGoogleCallback(t *testing.T) {
	tests := []struct {
		name           string
		cfg            GoogleConfig
		queryParams    string
		mockSetup      func(*MockHTTPClient, *MockRepository)
		expectedStatus int
		expectedLoc    string // partial match
	}{
		{
			name:        "Happy Path",
			cfg:         GoogleConfig{ClientID: "id", ClientSecret: "sec", RedirectURL: "red"},
			queryParams: "code=123&state=" + url.QueryEscape("http://frontend/home"),
			mockSetup: func(m *MockHTTPClient, r *MockRepository) {
				// Token exchange
				m.On("PostForm", googleTokenURL, mock.Anything).Return(
					mockResponse(200, googleTokenResponse{AccessToken: "at", IDToken: "it"}), nil)
				// User Info
				m.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.URL.String() == googleUserinfo
				})).Return(mockResponse(200, googleUserInfo{Sub: "sub1", Email: "test@gmail.com"}), nil)
				// Repo
				r.On("UpsertUserWithGoogle", mock.Anything, "test@gmail.com", "sub1").
					Return(AuthUser{ID: "u1", Email: "test@gmail.com"}, nil)
			},
			expectedStatus: http.StatusFound,
			expectedLoc:    "http://frontend/home",
		},
		{
			name:           "Not Configured",
			cfg:            GoogleConfig{},
			queryParams:    "",
			mockSetup:      func(m *MockHTTPClient, r *MockRepository) {},
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			name:           "Google Error Param",
			cfg:            GoogleConfig{ClientID: "id", ClientSecret: "sec", RedirectURL: "red"},
			queryParams:    "error=access_denied",
			mockSetup:      func(m *MockHTTPClient, r *MockRepository) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Missing Code",
			cfg:            GoogleConfig{ClientID: "id", ClientSecret: "sec", RedirectURL: "red"},
			queryParams:    "state=xyz",
			mockSetup:      func(m *MockHTTPClient, r *MockRepository) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:        "Token Exchange Network Error",
			cfg:         GoogleConfig{ClientID: "id", ClientSecret: "sec", RedirectURL: "red"},
			queryParams: "code=123",
			mockSetup: func(m *MockHTTPClient, r *MockRepository) {
				m.On("PostForm", googleTokenURL, mock.Anything).Return(nil, errors.New("net err"))
			},
			expectedStatus: http.StatusBadGateway,
		},
		{
			name:        "Token Exchange Non-200",
			cfg:         GoogleConfig{ClientID: "id", ClientSecret: "sec", RedirectURL: "red"},
			queryParams: "code=123",
			mockSetup: func(m *MockHTTPClient, r *MockRepository) {
				m.On("PostForm", googleTokenURL, mock.Anything).Return(mockResponse(400, "err"), nil)
			},
			expectedStatus: http.StatusBadGateway,
		},
		{
			name:        "Token Exchange Invalid JSON",
			cfg:         GoogleConfig{ClientID: "id", ClientSecret: "sec", RedirectURL: "red"},
			queryParams: "code=123",
			mockSetup: func(m *MockHTTPClient, r *MockRepository) {
				m.On("PostForm", googleTokenURL, mock.Anything).Return(mockResponse(200, "not-json"), nil)
			},
			expectedStatus: http.StatusBadGateway,
		},
		{
			name:        "UserInfo Network Error",
			cfg:         GoogleConfig{ClientID: "id", ClientSecret: "sec", RedirectURL: "red"},
			queryParams: "code=123",
			mockSetup: func(m *MockHTTPClient, r *MockRepository) {
				m.On("PostForm", googleTokenURL, mock.Anything).Return(
					mockResponse(200, googleTokenResponse{AccessToken: "at"}), nil)
				m.On("Do", mock.Anything).Return(nil, errors.New("net err"))
			},
			expectedStatus: http.StatusBadGateway,
		},
		{
			name:        "UserInfo Non-200",
			cfg:         GoogleConfig{ClientID: "id", ClientSecret: "sec", RedirectURL: "red"},
			queryParams: "code=123",
			mockSetup: func(m *MockHTTPClient, r *MockRepository) {
				m.On("PostForm", googleTokenURL, mock.Anything).Return(
					mockResponse(200, googleTokenResponse{AccessToken: "at"}), nil)
				m.On("Do", mock.Anything).Return(mockResponse(401, "unauth"), nil)
			},
			expectedStatus: http.StatusBadGateway,
		},
		{
			name:        "UserInfo Invalid JSON",
			cfg:         GoogleConfig{ClientID: "id", ClientSecret: "sec", RedirectURL: "red"},
			queryParams: "code=123",
			mockSetup: func(m *MockHTTPClient, r *MockRepository) {
				m.On("PostForm", googleTokenURL, mock.Anything).Return(
					mockResponse(200, googleTokenResponse{AccessToken: "at"}), nil)
				m.On("Do", mock.Anything).Return(mockResponse(200, "garbage"), nil)
			},
			expectedStatus: http.StatusBadGateway,
		},
		{
			name:        "UserInfo Missing Email",
			cfg:         GoogleConfig{ClientID: "id", ClientSecret: "sec", RedirectURL: "red"},
			queryParams: "code=123",
			mockSetup: func(m *MockHTTPClient, r *MockRepository) {
				m.On("PostForm", googleTokenURL, mock.Anything).Return(
					mockResponse(200, googleTokenResponse{AccessToken: "at"}), nil)
				m.On("Do", mock.Anything).Return(mockResponse(200, googleUserInfo{Sub: "s"}), nil)
			},
			expectedStatus: http.StatusBadGateway,
		},
		{
			name:        "Repo Error",
			cfg:         GoogleConfig{ClientID: "id", ClientSecret: "sec", RedirectURL: "red"},
			queryParams: "code=123",
			mockSetup: func(m *MockHTTPClient, r *MockRepository) {
				m.On("PostForm", googleTokenURL, mock.Anything).Return(
					mockResponse(200, googleTokenResponse{AccessToken: "at"}), nil)
				m.On("Do", mock.Anything).Return(mockResponse(200, googleUserInfo{Sub: "s", Email: "e"}), nil)
				r.On("UpsertUserWithGoogle", mock.Anything, "e", "s").Return(AuthUser{}, errors.New("db err"))
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockRepository)
			mockHTTP := new(MockHTTPClient)
			// Need EmailSender too even if unused by these specific handlers, to satisfy Server struct
			mockEmail := new(MockEmailSender)

			if tt.mockSetup != nil {
				tt.mockSetup(mockHTTP, mockRepo)
			}

			srv := &Server{
				repo:        mockRepo,
				httpClient:  mockHTTP,
				emailSender: mockEmail,
				googleCfg:   tt.cfg,
				frontendURL: "http://frontend",
				jwtSecret:   []byte("secret"),
				accessTTL:   time.Minute,
				refreshTTL:  time.Minute,
			}

			req := httptest.NewRequest("GET", "/auth/google/callback?"+tt.queryParams, nil)
			w := httptest.NewRecorder()

			srv.handleGoogleCallback(w, req)

			resp := w.Result()
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			if tt.expectedLoc != "" {
				loc, _ := resp.Location()
				assert.Contains(t, loc.String(), tt.expectedLoc)
			}
		})
	}
}

func TestHandleFTLogin(t *testing.T) {
	mockRepo := new(MockRepository)
	mockHTTP := new(MockHTTPClient)
	srv := &Server{
		repo:       mockRepo,
		httpClient: mockHTTP,
		ftCfg: FTConfig{
			ClientID:     "ft-id",
			ClientSecret: "ft-secret",
			RedirectURL:  "http://localhost:3000/ft/callback",
		},
	}

	req := httptest.NewRequest("GET", "/auth/42/login", nil)
	w := httptest.NewRecorder()

	srv.handleFTLogin(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusFound, resp.StatusCode)
	loc, _ := resp.Location()
	assert.Contains(t, loc.String(), "https://api.intra.42.fr/oauth/authorize")
	assert.Contains(t, loc.String(), "client_id=ft-id")
}

func TestHandleFTCallback(t *testing.T) {
	tests := []struct {
		name           string
		cfg            FTConfig
		queryParams    string
		mockSetup      func(*MockHTTPClient, *MockRepository)
		expectedStatus int
	}{
		{
			name:        "Happy Path",
			cfg:         FTConfig{ClientID: "id", ClientSecret: "sec", RedirectURL: "red"},
			queryParams: "code=123",
			mockSetup: func(m *MockHTTPClient, r *MockRepository) {
				m.On("PostForm", ftTokenURL, mock.Anything).Return(
					mockResponse(200, ftTokenResponse{AccessToken: "at"}), nil)
				m.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.URL.String() == ftMeURL
				})).Return(mockResponse(200, ftUserInfo{ID: 42, Email: "f@42.fr"}), nil)
				r.On("UpsertUserWithFT", mock.Anything, "f@42.fr", "ft:42").
					Return(AuthUser{ID: "u1"}, nil)
			},
			expectedStatus: http.StatusFound,
		},
		{
			name:           "Not Configured",
			cfg:            FTConfig{},
			queryParams:    "",
			mockSetup:      func(m *MockHTTPClient, r *MockRepository) {},
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			name:           "Error Param",
			cfg:            FTConfig{ClientID: "id", ClientSecret: "sec", RedirectURL: "red"},
			queryParams:    "error=fail",
			mockSetup:      func(m *MockHTTPClient, r *MockRepository) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:        "Token Exchange Fail",
			cfg:         FTConfig{ClientID: "id", ClientSecret: "sec", RedirectURL: "red"},
			queryParams: "code=123",
			mockSetup: func(m *MockHTTPClient, r *MockRepository) {
				m.On("PostForm", ftTokenURL, mock.Anything).Return(nil, errors.New("err"))
			},
			expectedStatus: http.StatusBadGateway,
		},
		{
			name:        "UserInfo Fail",
			cfg:         FTConfig{ClientID: "id", ClientSecret: "sec", RedirectURL: "red"},
			queryParams: "code=123",
			mockSetup: func(m *MockHTTPClient, r *MockRepository) {
				m.On("PostForm", ftTokenURL, mock.Anything).Return(
					mockResponse(200, ftTokenResponse{AccessToken: "at"}), nil)
				m.On("Do", mock.Anything).Return(nil, errors.New("err"))
			},
			expectedStatus: http.StatusBadGateway,
		},
		{
			name:        "Repo Error",
			cfg:         FTConfig{ClientID: "id", ClientSecret: "sec", RedirectURL: "red"},
			queryParams: "code=123",
			mockSetup: func(m *MockHTTPClient, r *MockRepository) {
				m.On("PostForm", ftTokenURL, mock.Anything).Return(
					mockResponse(200, ftTokenResponse{AccessToken: "at"}), nil)
				m.On("Do", mock.Anything).Return(mockResponse(200, ftUserInfo{ID: 42, Email: "e"}), nil)
				r.On("UpsertUserWithFT", mock.Anything, "e", "ft:42").Return(AuthUser{}, errors.New("db"))
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockRepository)
			mockHTTP := new(MockHTTPClient)
			// Ensure all mocks initialized
			mockEmail := new(MockEmailSender)

			if tt.mockSetup != nil {
				tt.mockSetup(mockHTTP, mockRepo)
			}

			srv := &Server{
				repo:        mockRepo,
				httpClient:  mockHTTP,
				emailSender: mockEmail,
				ftCfg:       tt.cfg,
				frontendURL: "http://front",
				jwtSecret:   []byte("s"),
				accessTTL:   time.Minute,
				refreshTTL:  time.Minute,
			}

			req := httptest.NewRequest("GET", "/auth/42/callback?"+tt.queryParams, nil)
			w := httptest.NewRecorder()

			srv.handleFTCallback(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}
