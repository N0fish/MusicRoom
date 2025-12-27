package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCurrentUserMiddleware(t *testing.T) {
	t.Run("ValidHeader", func(t *testing.T) {
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := userIDFromContext(r)
			assert.True(t, ok)
			assert.Equal(t, "user-123", userID)
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-User-Id", "user-123")
		w := httptest.NewRecorder()

		handler := currentUserMiddleware(nextHandler)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("MissingHeader", func(t *testing.T) {
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Should not be called
			t.Error("next handler should not be called")
		})

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		handler := currentUserMiddleware(nextHandler)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "missing user id")
	})

	t.Run("EmptyHeader", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-User-Id", "   ")
		w := httptest.NewRecorder()

		handler := currentUserMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestUserIDFromContext(t *testing.T) {
	t.Run("Found", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), ctxUserIDKey{}, "123")
		req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
		id, ok := userIDFromContext(req)
		assert.True(t, ok)
		assert.Equal(t, "123", id)
	})

	t.Run("NotFound", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		id, ok := userIDFromContext(req)
		assert.False(t, ok)
		assert.Empty(t, id)
	})

	t.Run("WrongType", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), ctxUserIDKey{}, 123) // int instead of string
		req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
		id, ok := userIDFromContext(req)
		assert.False(t, ok)
		assert.Empty(t, id)
	})
}
