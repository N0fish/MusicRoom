package main

import (
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
	if req.Bio != nil {
		prof.Bio = strings.TrimSpace(*req.Bio)
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

	resp := PublicUserProfileFromModel(prof)

	isOwner := viewerID == targetUserID
	isFriend := false
	if viewerID != "" && !isOwner {
		isFriend, _ = s.areFriends(r.Context(), viewerID, targetUserID)
	}

	switch prof.Visibility {
	case "private":
		if isOwner {
			resp.Bio = prof.Bio
			resp.AvatarURL = resolveAvatarForViewer(prof, isFriend, isOwner)
		} else {
			resp.Bio = ""
			resp.AvatarURL = defaultAvatarURL()
		}

	case "friends":
		if isOwner || isFriend {
			resp.Bio = prof.Bio
			resp.AvatarURL = resolveAvatarForViewer(prof, isFriend, isOwner)
		} else {
			resp.Bio = ""
			resp.AvatarURL = defaultAvatarURL()
		}

	case "public":
		resp.Bio = prof.Bio
		resp.AvatarURL = resolveAvatarForViewer(prof, isFriend, isOwner)
	}

	writeJSON(w, http.StatusOK, resp)
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

func (s *Server) handleCheckUserExists(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(chi.URLParam(r, "id"))
	if userID == "" {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	_, err := s.findProfileByUserID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrProfileNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		log.Printf("user-service: check exists: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
