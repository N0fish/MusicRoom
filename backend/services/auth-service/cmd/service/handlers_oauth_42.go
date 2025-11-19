package main

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	ftAuthURL  = "https://api.intra.42.fr/oauth/authorize"
	ftTokenURL = "https://api.intra.42.fr/oauth/token"
	ftMeURL    = "https://api.intra.42.fr/v2/me"
)

type ftTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
	Scope       string `json:"scope"`
	CreatedAt   int64  `json:"created_at"`
}

type ftUserInfo struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Login string `json:"login"`
}

func (s *Server) handleFTLogin(w http.ResponseWriter, r *http.Request) {
	if s.ftCfg.ClientID == "" || s.ftCfg.ClientSecret == "" || s.ftCfg.RedirectURL == "" {
		writeError(w, http.StatusServiceUnavailable, "42 oauth not configured")
		return
	}

	redirect := r.URL.Query().Get("redirect")
	if redirect == "" {
		redirect = s.frontendURL
	}

	state := url.QueryEscape(redirect)
	v := url.Values{}
	v.Set("client_id", s.ftCfg.ClientID)
	v.Set("redirect_uri", s.ftCfg.RedirectURL)
	v.Set("response_type", "code")
	v.Set("scope", "public")
	v.Set("state", state)
	authURL := ftAuthURL + "?" + v.Encode()

	http.Redirect(w, r, authURL, http.StatusFound)
}

func (s *Server) handleFTCallback(w http.ResponseWriter, r *http.Request) {
	if s.ftCfg.ClientID == "" || s.ftCfg.ClientSecret == "" || s.ftCfg.RedirectURL == "" {
		writeError(w, http.StatusServiceUnavailable, "42 oauth not configured")
		return
	}

	q := r.URL.Query()
	if errStr := q.Get("error"); errStr != "" {
		writeError(w, http.StatusBadRequest, "42 error: "+errStr)
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

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", s.ftCfg.ClientID)
	form.Set("client_secret", s.ftCfg.ClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", s.ftCfg.RedirectURL)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.PostForm(ftTokenURL, form)
	if err != nil {
		log.Printf("42 callback: token exchange error: %v", err)
		writeError(w, http.StatusBadGateway, "42 token exchange failed")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		writeError(w, http.StatusBadGateway, "42 token exchange failed")
		return
	}

	var tr ftTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		writeError(w, http.StatusBadGateway, "invalid 42 token response")
		return
	}

	req, err := http.NewRequest(http.MethodGet, ftMeURL, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	req.Header.Set("Authorization", "Bearer "+tr.AccessToken)

	uResp, err := client.Do(req)
	if err != nil {
		log.Printf("42 callback: me error: %v", err)
		writeError(w, http.StatusBadGateway, "42 userinfo failed")
		return
	}
	defer uResp.Body.Close()

	if uResp.StatusCode != http.StatusOK {
		writeError(w, http.StatusBadGateway, "42 userinfo failed")
		return
	}

	var ui ftUserInfo
	if err := json.NewDecoder(uResp.Body).Decode(&ui); err != nil {
		writeError(w, http.StatusBadGateway, "invalid 42 userinfo response")
		return
	}

	email := strings.TrimSpace(strings.ToLower(ui.Email))
	if email == "" || ui.ID == 0 {
		writeError(w, http.StatusBadGateway, "42 userinfo missing email or id")
		return
	}

	ftID := formatFTID(ui.ID)
	user, err := s.upsertUserWithFT(r.Context(), email, ftID)
	if err != nil {
		log.Printf("42 callback: upsertUserWithFT: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	tokens, err := s.issueTokens(user)
	if err != nil {
		log.Printf("42 callback: issueTokens: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if r.URL.Query().Get("mode") == "json" {
		writeJSON(w, http.StatusOK, tokens)
		return
	}

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

func formatFTID(id int64) string {
	return "ft:" + strconv.FormatInt(id, 10)
}
