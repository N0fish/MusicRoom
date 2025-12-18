package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
	PostForm(urlStr string, data url.Values) (*http.Response, error)
	Get(urlStr string) (*http.Response, error)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": msg,
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// fallback to insecure but should never really happen
		return hex.EncodeToString(b)
	}
	return hex.EncodeToString(b)
}
