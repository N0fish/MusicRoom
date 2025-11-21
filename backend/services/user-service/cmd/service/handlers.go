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
)

type ctxUserIDKey struct{}

// currentUserMiddleware expects X-User-Id header from api-gateway.
// If header is missing, returns 401 (gateway должен всегда его ставить
// для защищённых маршрутов).
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

// handleGetMe returns full profile for the current user.
func (s *Server) handleGetMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	prof, err := s.getOrCreateProfile(r.Context(), userID)
	if err != nil {
		log.Printf("user-service: getOrCreateProfile: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	resp := UserProfileResponseFromModel(prof)
	writeJSON(w, http.StatusOK, resp)
}

// handlePatchMe updates profile fields for the current user.
func (s *Server) handlePatchMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req UpdateUserProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Load existing or create default profile
	prof, err := s.getOrCreateProfile(r.Context(), userID)
	if err != nil {
		log.Printf("user-service: getOrCreateProfile: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Apply updates
	now := time.Now()

	if req.DisplayName != nil {
		prof.DisplayName = strings.TrimSpace(*req.DisplayName)
	}
	if req.AvatarURL != nil {
		prof.AvatarURL = strings.TrimSpace(*req.AvatarURL)
	}
	if req.PublicBio != nil {
		prof.PublicBio = strings.TrimSpace(*req.PublicBio)
	}
	if req.FriendsBio != nil {
		prof.FriendsBio = strings.TrimSpace(*req.FriendsBio)
	}
	if req.PrivateBio != nil {
		prof.PrivateBio = strings.TrimSpace(*req.PrivateBio)
	}
	if req.Visibility != nil {
		prof.Visibility = *req.Visibility
	}
	if req.Preferences != nil {
		prof.Preferences = Preferences{
			Genres:  uniqueStrings(req.Preferences.Genres),
			Artists: uniqueStrings(req.Preferences.Artists),
			Moods:   uniqueStrings(req.Preferences.Moods),
		}
	}

	prof.UpdatedAt = now

	if err := s.saveProfile(r.Context(), prof); err != nil {
		log.Printf("user-service: saveProfile: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	resp := UserProfileResponseFromModel(prof)
	writeJSON(w, http.StatusOK, resp)
}

// handleGetPublicProfile returns public view of a user's profile by auth user ID.
func (s *Server) handleGetPublicProfile(w http.ResponseWriter, r *http.Request) {
	targetUserID := chi.URLParam(r, "id")
	targetUserID = strings.TrimSpace(targetUserID)
	if targetUserID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	prof, err := s.findProfileByUserID(r.Context(), targetUserID)
	if err != nil {
		if errors.Is(err, ErrProfileNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		log.Printf("user-service: findProfileByUserID: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// TODO: Сервис дружбы ?? он нужен вообще по сабжекту ??
	// пока считаю, что внешний клиент всегда "незнакомец".
	// Поэтому возвращаю только public-часть + общую видимость.
	resp := PublicUserProfileFromModel(prof)
	writeJSON(w, http.StatusOK, resp)
}

func uniqueStrings(in []string) []string {
	m := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		lv := strings.ToLower(v)
		if _, ok := m[lv]; ok {
			continue
		}
		m[lv] = struct{}{}
		out = append(out, v)
	}
	return out
}

// --- DTOs / JSON models ---

type PreferencesDTO struct {
	Genres  []string `json:"genres,omitempty"`
	Artists []string `json:"artists,omitempty"`
	Moods   []string `json:"moods,omitempty"`
}

type UserProfileResponse struct {
	ID          string         `json:"id"`
	UserID      string         `json:"userId"`
	DisplayName string         `json:"displayName"`
	AvatarURL   string         `json:"avatarUrl,omitempty"`
	PublicBio   string         `json:"publicBio,omitempty"`
	FriendsBio  string         `json:"friendsBio,omitempty"`
	PrivateBio  string         `json:"privateBio,omitempty"`
	Visibility  string         `json:"visibility"`
	Preferences PreferencesDTO `json:"preferences"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

type PublicUserProfileResponse struct {
	UserID      string         `json:"userId"`
	DisplayName string         `json:"displayName"`
	AvatarURL   string         `json:"avatarUrl,omitempty"`
	PublicBio   string         `json:"publicBio,omitempty"`
	Visibility  string         `json:"visibility"`
	Preferences PreferencesDTO `json:"preferences"`
}

type UpdateUserProfileRequest struct {
	DisplayName *string         `json:"displayName,omitempty"`
	AvatarURL   *string         `json:"avatarUrl,omitempty"`
	PublicBio   *string         `json:"publicBio,omitempty"`
	FriendsBio  *string         `json:"friendsBio,omitempty"`
	PrivateBio  *string         `json:"privateBio,omitempty"`
	Visibility  *string         `json:"visibility,omitempty"` // public|friends|private
	Preferences *PreferencesDTO `json:"preferences,omitempty"`
}

func (r *UpdateUserProfileRequest) Validate() error {
	if r.Visibility != nil {
		v := strings.ToLower(strings.TrimSpace(*r.Visibility))
		switch v {
		case "public", "friends", "private":
			*r.Visibility = v
		default:
			return errors.New("invalid visibility, must be one of: public, friends, private")
		}
	}
	return nil
}

func UserProfileResponseFromModel(p UserProfile) UserProfileResponse {
	return UserProfileResponse{
		ID:          p.ID,
		UserID:      p.UserID,
		DisplayName: p.DisplayName,
		AvatarURL:   p.AvatarURL,
		PublicBio:   p.PublicBio,
		FriendsBio:  p.FriendsBio,
		PrivateBio:  p.PrivateBio,
		Visibility:  p.Visibility,
		Preferences: PreferencesDTO{
			Genres:  append([]string{}, p.Preferences.Genres...),
			Artists: append([]string{}, p.Preferences.Artists...),
			Moods:   append([]string{}, p.Preferences.Moods...),
		},
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}

func PublicUserProfileFromModel(p UserProfile) PublicUserProfileResponse {
	return PublicUserProfileResponse{
		UserID:      p.UserID,
		DisplayName: p.DisplayName,
		AvatarURL:   p.AvatarURL,
		PublicBio:   p.PublicBio,
		Visibility:  p.Visibility,
		Preferences: PreferencesDTO{
			Genres:  append([]string{}, p.Preferences.Genres...),
			Artists: append([]string{}, p.Preferences.Artists...),
			Moods:   append([]string{}, p.Preferences.Moods...),
		},
	}
}
