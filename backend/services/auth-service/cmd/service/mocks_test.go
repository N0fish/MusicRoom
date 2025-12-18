package main

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/stretchr/testify/mock"
)

type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) FindUserByEmail(ctx context.Context, email string) (AuthUser, error) {
	args := m.Called(ctx, email)
	return args.Get(0).(AuthUser), args.Error(1)
}

func (m *MockRepository) FindUserByID(ctx context.Context, id string) (AuthUser, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(AuthUser), args.Error(1)
}

func (m *MockRepository) FindUserByGoogleID(ctx context.Context, googleID string) (AuthUser, error) {
	args := m.Called(ctx, googleID)
	return args.Get(0).(AuthUser), args.Error(1)
}

func (m *MockRepository) FindUserByFTID(ctx context.Context, ftID string) (AuthUser, error) {
	args := m.Called(ctx, ftID)
	return args.Get(0).(AuthUser), args.Error(1)
}

func (m *MockRepository) CreateUserWithPassword(ctx context.Context, email, passwordHash string) (AuthUser, error) {
	args := m.Called(ctx, email, passwordHash)
	return args.Get(0).(AuthUser), args.Error(1)
}

func (m *MockRepository) UpsertUserWithGoogle(ctx context.Context, email, googleID string) (AuthUser, error) {
	args := m.Called(ctx, email, googleID)
	return args.Get(0).(AuthUser), args.Error(1)
}

func (m *MockRepository) UpsertUserWithFT(ctx context.Context, email, ftID string) (AuthUser, error) {
	args := m.Called(ctx, email, ftID)
	if val := args.Get(0); val != nil {
		return val.(AuthUser), args.Error(1)
	}
	return AuthUser{}, args.Error(1)
}

func (m *MockRepository) SetVerificationToken(ctx context.Context, userID, token string) error {
	args := m.Called(ctx, userID, token)
	return args.Error(0)
}

func (m *MockRepository) VerifyEmailByToken(ctx context.Context, token string) (AuthUser, error) {
	args := m.Called(ctx, token)
	return args.Get(0).(AuthUser), args.Error(1)
}

func (m *MockRepository) SetResetToken(ctx context.Context, userID, token string, expiresAt time.Time) error {
	args := m.Called(ctx, userID, token, expiresAt)
	return args.Error(0)
}

func (m *MockRepository) ResetPasswordByToken(ctx context.Context, token, newHash string, now time.Time) (AuthUser, error) {
	args := m.Called(ctx, token, newHash, now)
	return args.Get(0).(AuthUser), args.Error(1)
}

func (m *MockRepository) DeleteUser(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockRepository) UpdateGoogleID(ctx context.Context, userID string, googleID *string) (AuthUser, error) {
	args := m.Called(ctx, userID, googleID)
	return args.Get(0).(AuthUser), args.Error(1)
}

func (m *MockRepository) UpdateFTID(ctx context.Context, userID string, ftID *string) (AuthUser, error) {
	args := m.Called(ctx, userID, ftID)
	return args.Get(0).(AuthUser), args.Error(1)
}

// MockEmailSender
type MockEmailSender struct {
	mock.Mock
}

func (m *MockEmailSender) Send(to, subject, body string) error {
	args := m.Called(to, subject, body)
	return args.Error(0)
}

// MockHTTPClient
type MockHTTPClient struct {
	mock.Mock
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	if val := args.Get(0); val != nil {
		return val.(*http.Response), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockHTTPClient) PostForm(urlStr string, data url.Values) (*http.Response, error) {
	args := m.Called(urlStr, data)
	if val := args.Get(0); val != nil {
		return val.(*http.Response), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockHTTPClient) Get(url string) (*http.Response, error) {
	args := m.Called(url)
	if val := args.Get(0); val != nil {
		return val.(*http.Response), args.Error(1)
	}
	return nil, args.Error(1)
}
