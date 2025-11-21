package main

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	googleAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL = "https://oauth2.googleapis.com/token"
	googleUserinfo = "https://openidconnect.googleapis.com/v1/userinfo"
	googleScope    = "openid email profile"
)

type googleTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
	TokenType   string `json:"token_type"`
	IDToken     string `json:"id_token"`
}

type googleUserInfo struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
}

func (s *Server) handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	if s.googleCfg.ClientID == "" || s.googleCfg.ClientSecret == "" || s.googleCfg.RedirectURL == "" {
		writeError(w, http.StatusServiceUnavailable, "google oauth not configured")
		return
	}

	redirect := r.URL.Query().Get("redirect")
	if redirect == "" {
		redirect = s.frontendURL
	}

	state := url.QueryEscape(redirect)
	v := url.Values{}
	v.Set("client_id", s.googleCfg.ClientID)
	v.Set("redirect_uri", s.googleCfg.RedirectURL)
	v.Set("response_type", "code")
	v.Set("scope", googleScope)
	v.Set("state", state)
	v.Set("access_type", "online")
	authURL := googleAuthURL + "?" + v.Encode()

	http.Redirect(w, r, authURL, http.StatusFound)
}

func (s *Server) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	if s.googleCfg.ClientID == "" || s.googleCfg.ClientSecret == "" || s.googleCfg.RedirectURL == "" {
		log.Printf(s.googleCfg.ClientID, s.googleCfg.ClientSecret, s.googleCfg.RedirectURL)
		writeError(w, http.StatusServiceUnavailable, "google oauth not configured")
		return
	}

	q := r.URL.Query()
	if errStr := q.Get("error"); errStr != "" {
		writeError(w, http.StatusBadRequest, "google error: "+errStr)
		return
	}

	code := q.Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}

	state := q.Get("state")
	redirect := s.frontendURL
	if state != "" {
		if decoded, err := url.QueryUnescape(state); err == nil && decoded != "" {
			redirect = decoded
		}
	}

	// exchange code for token
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", s.googleCfg.ClientID)
	form.Set("client_secret", s.googleCfg.ClientSecret)
	form.Set("redirect_uri", s.googleCfg.RedirectURL)
	form.Set("grant_type", "authorization_code")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.PostForm(googleTokenURL, form)
	if err != nil {
		log.Printf("google callback: token exchange error: %v", err)
		writeError(w, http.StatusBadGateway, "google token exchange failed")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		writeError(w, http.StatusBadGateway, "google token exchange failed")
		return
	}

	var tr googleTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		writeError(w, http.StatusBadGateway, "invalid google token response")
		return
	}

	// fetch userinfo
	req, err := http.NewRequest(http.MethodGet, googleUserinfo, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	req.Header.Set("Authorization", "Bearer "+tr.AccessToken)

	uResp, err := client.Do(req)
	if err != nil {
		log.Printf("google callback: userinfo error: %v", err)
		writeError(w, http.StatusBadGateway, "google userinfo failed")
		return
	}
	defer uResp.Body.Close()

	if uResp.StatusCode != http.StatusOK {
		writeError(w, http.StatusBadGateway, "google userinfo failed")
		return
	}

	var ui googleUserInfo
	if err := json.NewDecoder(uResp.Body).Decode(&ui); err != nil {
		writeError(w, http.StatusBadGateway, "invalid google userinfo response")
		return
	}

	email := strings.TrimSpace(strings.ToLower(ui.Email))
	if email == "" || ui.Sub == "" {
		writeError(w, http.StatusBadGateway, "google userinfo missing email or sub")
		return
	}

	user, err := s.upsertUserWithGoogle(r.Context(), email, ui.Sub)
	if err != nil {
		log.Printf("google callback: upsertUserWithGoogle: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	tokens, err := s.issueTokens(user)
	if err != nil {
		log.Printf("google callback: issueTokens: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Return tokens as JSON if requested explicitly
	if r.URL.Query().Get("mode") == "json" {
		writeJSON(w, http.StatusOK, tokens)
		return
	}

	// Default: redirect back to frontend with tokens in URL fragment
	redirectURL, err := url.Parse(redirect)
	if err != nil {
		redirectURL, _ = url.Parse(s.frontendURL)
	}
	fragment := url.Values{}
	fragment.Set("accessToken", tokens.AccessToken)
	fragment.Set("refreshToken", tokens.RefreshToken)
	redirectURL.Fragment = fragment.Encode()

	http.Redirect(w, r, redirectURL.String(), http.StatusFound)
}
