package main

import (
	"context"
	"net/http"
	"strings"
)

type ctxUserIDKey struct{}

func currentUserMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-Id")
		if strings.TrimSpace(userID) == "" {
			writeError(w, http.StatusUnauthorized, "missing user id")
			return
		}
		ctx := r.Context()
		ctx = context.WithValue(ctx, ctxUserIDKey{}, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func userIDFromContext(r *http.Request) (string, bool) {
	v := r.Context().Value(ctxUserIDKey{})
	if v == nil {
		return "", false
	}
	s, ok := v.(string)
	return s, ok && s != ""
}
