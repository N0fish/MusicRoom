package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// POST /users/me/avatar/random
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

	avatarURL := randomAvatarURL()

	prof.AvatarURL = avatarURL
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

// POST /users/me/avatar/upload
func (s *Server) handleUploadAvatar(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	const maxUploadSize = 5 * 1024 * 1024 // 5MB
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		writeError(w, http.StatusBadRequest, "file too large or invalid form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp":
	default:
		writeError(w, http.StatusBadRequest, "unsupported file type (allowed: png, jpg, jpeg, webp)")
		return
	}

	customDir := customAvatarDir()
	if err := os.MkdirAll(customDir, 0o755); err != nil {
		log.Printf("user-service: mkdir custom avatars: %v", err)
		writeError(w, http.StatusInternalServerError, "cannot save avatar")
		return
	}

	filename := userID + ext
	dstPath := filepath.Join(customDir, filename)

	dst, err := os.Create(dstPath)
	if err != nil {
		log.Printf("user-service: create avatar file: %v", err)
		writeError(w, http.StatusInternalServerError, "cannot save avatar")
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		log.Printf("user-service: write avatar file: %v", err)
		writeError(w, http.StatusInternalServerError, "cannot save avatar")
		return
	}

	prof, err := s.getOrCreateProfile(r.Context(), userID)
	if err != nil {
		log.Printf("user-service: getOrCreateProfile (upload): %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	prof.AvatarURL = "/avatars/custom/" + filename
	prof.HasCustomAvatar = true
	prof.UpdatedAt = time.Now().UTC()

	if err := s.saveProfile(r.Context(), prof); err != nil {
		log.Printf("user-service: saveProfile (upload): %v", err)
		writeError(w, http.StatusInternalServerError, "cannot save avatar")
		return
	}

	resp := UserProfileResponseFromModel(prof)
	resp.AvatarURL = resolveAvatarForViewer(prof, false, true)

	writeJSON(w, http.StatusOK, resp)
}
