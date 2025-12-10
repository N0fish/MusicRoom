package main

import (
	"math/rand"
	"os"
	"path/filepath"
	"strings"
)

func defaultAvatarURL() string {
	return getenv("DEFAULT_AVATAR_URL", "/avatars/default.svg")
}

func avatarDir() string {
	return getenv("AVATAR_DIR", "./avatars")
}

func customAvatarDir() string {
	return filepath.Join(avatarDir(), "custom")
}

func listAvatarFiles() ([]string, error) {
	dir := avatarDir()

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		switch ext {
		case ".svg", ".png", ".jpg", ".jpeg", ".webp":
			files = append(files, name)
		}
	}

	return files, nil
}

func randomAvatarURL() string {
	files, err := listAvatarFiles()
	if err != nil || len(files) == 0 {
		return defaultAvatarURL()
	}

	// rand.Seed(time.Now().UnixNano())
	name := files[rand.Intn(len(files))]
	return "/avatars/" + name
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
