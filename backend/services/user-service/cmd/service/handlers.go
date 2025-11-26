package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
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

type PreferencesDTO struct {
	Genres  []string `json:"genres,omitempty"`
	Artists []string `json:"artists,omitempty"`
	Moods   []string `json:"moods,omitempty"`
}

type UserProfileResponse struct {
	ID              string         `json:"id"`
	UserID          string         `json:"userId"`
	Username        string         `json:"username"`
	DisplayName     string         `json:"displayName"`
	AvatarURL       string         `json:"avatarUrl,omitempty"`
	HasCustomAvatar bool           `json:"hasCustomAvatar"`
	PublicBio       string         `json:"publicBio,omitempty"`
	FriendsBio      string         `json:"friendsBio,omitempty"`
	PrivateBio      string         `json:"privateBio,omitempty"`
	Visibility      string         `json:"visibility"`
	Preferences     PreferencesDTO `json:"preferences"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
}

type PublicUserProfileResponse struct {
	UserID      string         `json:"userId"`
	Username    string         `json:"username"`
	DisplayName string         `json:"displayName"`
	AvatarURL   string         `json:"avatarUrl,omitempty"`
	Visibility  string         `json:"visibility"`
	Preferences PreferencesDTO `json:"preferences"`
}

type UpdateUserProfileRequest struct {
	DisplayName *string         `json:"displayName,omitempty"`
	AvatarURL   *string         `json:"avatarUrl,omitempty"`
	PublicBio   *string         `json:"publicBio,omitempty"`
	FriendsBio  *string         `json:"friendsBio,omitempty"`
	PrivateBio  *string         `json:"privateBio,omitempty"`
	Visibility  *string         `json:"visibility,omitempty"`
	Preferences *PreferencesDTO `json:"preferences,omitempty"`
}

func (r *UpdateUserProfileRequest) Validate() error {
	if r.AvatarURL != nil {
		return errors.New("avatarUrl cannot be updated directly; use /users/me/avatar/random")
	}
	if r.Visibility != nil {
		v := strings.ToLower(strings.TrimSpace(*r.Visibility))
		switch v {
		case "public", "friends", "private":
			*r.Visibility = v
		default:
			return errors.New("invalid visibility, must be one of: public, friends, private")
		}
	}

	const maxShort = 100
	const maxLong = 400

	trimPtr := func(p *string, max int) {
		if p == nil {
			return
		}
		s := strings.TrimSpace(*p)
		if len(s) > max {
			s = s[:max]
		}
		*p = s
	}

	trimPtr(r.DisplayName, maxShort)
	trimPtr(r.PublicBio, maxLong)
	trimPtr(r.FriendsBio, maxLong)
	trimPtr(r.PrivateBio, maxLong)

	return nil
}

type FriendItem struct {
	UserID      string `json:"userId"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl,omitempty"`
}

type FriendRequestResponse struct {
	ID         string    `json:"id"`
	FromUserID string    `json:"fromUserId"`
	ToUserID   string    `json:"toUserId"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	var out []string
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		vLower := strings.ToLower(v)
		if _, ok := seen[vLower]; ok {
			continue
		}
		seen[vLower] = struct{}{}
		out = append(out, v)
	}
	return out
}

func defaultAvatarURL() string {
	return getenv("DEFAULT_AVATAR_URL", "/static/avatars/default.svg")
}

func resolveAvatarForViewer(p UserProfile, viewerIsFriend bool, viewerIsOwner bool) string {
	if p.Visibility == "private" && !viewerIsOwner {
		return defaultAvatarURL()
	}
	if p.Visibility == "friends" && !viewerIsFriend && !viewerIsOwner {
		return defaultAvatarURL()
	}
	if p.HasCustomAvatar && strings.TrimSpace(p.AvatarURL) != "" {
		return p.AvatarURL
	}
	return defaultAvatarURL()
}

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
	resp.AvatarURL = resolveAvatarForViewer(prof, false, true)

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handlePatchMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req UpdateUserProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	prof, err := s.getOrCreateProfile(r.Context(), userID)
	if err != nil {
		log.Printf("user-service: getOrCreateProfile: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	now := time.Now().UTC()

	if req.DisplayName != nil {
		prof.DisplayName = strings.TrimSpace(*req.DisplayName)
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
	resp.AvatarURL = resolveAvatarForViewer(prof, false, true)

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetPublicProfile(w http.ResponseWriter, r *http.Request) {
	targetUserID := strings.TrimSpace(chi.URLParam(r, "id"))
	if targetUserID == "" {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	viewerID, _ := userIDFromContext(r)

	prof, err := s.getOrCreateProfile(r.Context(), targetUserID)
	if err != nil {
		if errors.Is(err, ErrProfileNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		log.Printf("user-service: get profile: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	isFriend := false
	if viewerID != "" && viewerID != targetUserID {
		isFriend, _ = s.areFriends(r.Context(), viewerID, targetUserID)
	}
	isOwner := viewerID == targetUserID

	resp := PublicUserProfileFromModel(prof)

	switch prof.Visibility {
	case "private":
		if !isOwner {
			resp.DisplayName = ""
			resp.Preferences = PreferencesDTO{}
			resp.AvatarURL = defaultAvatarURL()
		} else {
			resp.AvatarURL = resolveAvatarForViewer(prof, isFriend, isOwner)
		}
	case "friends":
		if !isFriend && !isOwner {
			resp.DisplayName = ""
			resp.Preferences = PreferencesDTO{}
			resp.AvatarURL = defaultAvatarURL()
		} else {
			resp.AvatarURL = resolveAvatarForViewer(prof, isFriend, isOwner)
		}
	case "public":
		resp.AvatarURL = resolveAvatarForViewer(prof, isFriend, isOwner)
	default:
		resp.AvatarURL = resolveAvatarForViewer(prof, isFriend, isOwner)
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleSearchUsers(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("query"))
	if q == "" {
		writeError(w, http.StatusBadRequest, "query is required")
		return
	}
	if len(q) > 50 {
		writeError(w, http.StatusBadRequest, "query too long")
		return
	}

	pattern := "%" + q + "%"

	rows, err := s.db.Query(r.Context(), `
      SELECT id, user_id, display_name, username,
             avatar_url, has_custom_avatar,
             public_bio, friends_bio, private_bio,
             visibility, preferences,
             created_at, updated_at
      FROM user_profiles
      WHERE LOWER(username) LIKE LOWER($1)
         OR LOWER(display_name) LIKE LOWER($1)
      ORDER BY username
      LIMIT 20
  `, pattern)
	if err != nil {
		log.Printf("user-service: search users: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	var res []FriendItem
	for rows.Next() {
		p, err := scanUserProfile(rows)
		if err != nil {
			log.Printf("user-service: scan search user: %v", err)
			continue
		}
		res = append(res, FriendItem{
			UserID:      p.UserID,
			Username:    p.Username,
			DisplayName: p.DisplayName,
			AvatarURL:   resolveAvatarForViewer(p, false, false),
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": res,
	})
}

func (s *Server) handleListFriends(w http.ResponseWriter, r *http.Request) {
	me, ok := userIDFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	rows, err := s.db.Query(r.Context(), `
      SELECT p.user_id, p.username, p.display_name,
             p.avatar_url, p.has_custom_avatar, p.visibility
      FROM user_friends f
      JOIN user_profiles p
        ON p.user_id = CASE
            WHEN f.user1_id = $1 THEN f.user2_id
            ELSE f.user1_id
          END
      WHERE f.user1_id = $1 OR f.user2_id = $1
      ORDER BY p.username
  `, me)
	if err != nil {
		log.Printf("user-service: list friends: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	var items []FriendItem
	for rows.Next() {
		var p UserProfile
		if err := rows.Scan(
			&p.UserID,
			&p.Username,
			&p.DisplayName,
			&p.AvatarURL,
			&p.HasCustomAvatar,
			&p.Visibility,
		); err != nil {
			log.Printf("user-service: scan friend: %v", err)
			continue
		}
		items = append(items, FriendItem{
			UserID:      p.UserID,
			Username:    p.Username,
			DisplayName: p.DisplayName,
			AvatarURL:   resolveAvatarForViewer(p, true, false),
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": items,
	})
}

func (s *Server) handleSendFriendRequest(w http.ResponseWriter, r *http.Request) {
	me, ok := userIDFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	target := strings.TrimSpace(chi.URLParam(r, "id"))
	if target == "" || target == me {
		writeError(w, http.StatusBadRequest, "invalid target user")
		return
	}

	ctx := r.Context()

	already, err := s.areFriends(ctx, me, target)
	if err != nil {
		log.Printf("user-service: areFriends: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if already {
		writeError(w, http.StatusBadRequest, "already friends")
		return
	}

	var existingID string
	err = s.db.QueryRow(ctx, `
      SELECT id
      FROM friend_requests
      WHERE from_user_id = $1 AND to_user_id = $2 AND status = 'pending'
  `, target, me).Scan(&existingID)
	if err == nil && existingID != "" {
		_, err = s.db.Exec(ctx, `
        UPDATE friend_requests
        SET status = 'accepted', updated_at = now()
        WHERE id = $1
      `, existingID)
		if err != nil {
			log.Printf("user-service: accept reciprocal request: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if err := s.addFriends(ctx, me, target); err != nil {
			log.Printf("user-service: addFriends: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "accepted",
		})
		return
	}

	var resp FriendRequestResponse
	err = s.db.QueryRow(ctx, `
      INSERT INTO friend_requests (from_user_id, to_user_id, status)
      VALUES ($1, $2, 'pending')
      ON CONFLICT (from_user_id, to_user_id) WHERE status = 'pending'
      DO UPDATE SET updated_at = now()
      RETURNING id, from_user_id, to_user_id, status, created_at, updated_at
  `, me, target).Scan(
		&resp.ID,
		&resp.FromUserID,
		&resp.ToUserID,
		&resp.Status,
		&resp.CreatedAt,
		&resp.UpdatedAt,
	)
	if err != nil {
		log.Printf("user-service: send friend request: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAcceptFriendRequest(w http.ResponseWriter, r *http.Request) {
	me, ok := userIDFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	from := strings.TrimSpace(chi.URLParam(r, "id"))
	if from == "" || from == me {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	ctx := r.Context()
	var reqID string
	err := s.db.QueryRow(ctx, `
      SELECT id
      FROM friend_requests
      WHERE from_user_id = $1 AND to_user_id = $2 AND status = 'pending'
  `, from, me).Scan(&reqID)
	if err != nil {
		writeError(w, http.StatusNotFound, "no pending request")
		return
	}

	_, err = s.db.Exec(ctx, `
      UPDATE friend_requests
      SET status = 'accepted', updated_at = now()
      WHERE id = $1
  `, reqID)
	if err != nil {
		log.Printf("user-service: accept friend: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := s.addFriends(ctx, me, from); err != nil {
		log.Printf("user-service: addFriends: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "accepted",
	})
}

func (s *Server) handleRejectFriendRequest(w http.ResponseWriter, r *http.Request) {
	me, ok := userIDFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	from := strings.TrimSpace(chi.URLParam(r, "id"))
	if from == "" || from == me {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	ctx := r.Context()
	res, err := s.db.Exec(ctx, `
      UPDATE friend_requests
      SET status = 'rejected', updated_at = now()
      WHERE from_user_id = $1 AND to_user_id = $2 AND status = 'pending'
  `, from, me)
	if err != nil {
		log.Printf("user-service: reject friend: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	aff := res.RowsAffected()
	if aff == 0 {
		writeError(w, http.StatusNotFound, "no pending request")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "rejected",
	})
}

func (s *Server) handleRemoveFriend(w http.ResponseWriter, r *http.Request) {
	me, ok := userIDFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	friendID := strings.TrimSpace(chi.URLParam(r, "id"))
	if friendID == "" || friendID == me {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	if err := s.removeFriends(r.Context(), me, friendID); err != nil {
		log.Printf("user-service: removeFriends: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "removed",
	})
}

func (s *Server) handleGenerateRandomAvatar(w http.ResponseWriter, r *http.Request) {
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

	seed := fmt.Sprintf("%s-%d", userID, time.Now().UnixNano())
	svg := generateIdenticonSVG(seed)
	dataURL := "data:image/svg+xml;utf8," + url.QueryEscape(svg)

	prof.AvatarURL = dataURL
	prof.HasCustomAvatar = true
	prof.UpdatedAt = time.Now().UTC()

	if err := s.saveProfile(r.Context(), prof); err != nil {
		log.Printf("user-service: saveProfile avatar: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	resp := UserProfileResponseFromModel(prof)
	resp.AvatarURL = resolveAvatarForViewer(prof, false, true)

	writeJSON(w, http.StatusOK, resp)
}

func UserProfileResponseFromModel(p UserProfile) UserProfileResponse {
	return UserProfileResponse{
		ID:              p.ID,
		UserID:          p.UserID,
		Username:        p.Username,
		DisplayName:     p.DisplayName,
		AvatarURL:       p.AvatarURL,
		HasCustomAvatar: p.HasCustomAvatar,
		PublicBio:       p.PublicBio,
		FriendsBio:      p.FriendsBio,
		PrivateBio:      p.PrivateBio,
		Visibility:      p.Visibility,
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
		Username:    p.Username,
		DisplayName: p.DisplayName,
		AvatarURL:   p.AvatarURL,
		Visibility:  p.Visibility,
		Preferences: PreferencesDTO{
			Genres:  append([]string{}, p.Preferences.Genres...),
			Artists: append([]string{}, p.Preferences.Artists...),
			Moods:   append([]string{}, p.Preferences.Moods...),
		},
	}
}
