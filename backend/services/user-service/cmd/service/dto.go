package main

import (
	"errors"
	"strings"
	"time"
)

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
	Bio             string         `json:"bio,omitempty"`
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
	Bio         string         `json:"bio,omitempty"`
	Visibility  string         `json:"visibility"`
	Preferences PreferencesDTO `json:"preferences"`
}

type UpdateUserProfileRequest struct {
	DisplayName *string         `json:"displayName,omitempty"`
	AvatarURL   *string         `json:"avatarUrl,omitempty"`
	Bio         *string         `json:"bio,omitempty"`
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
	trimPtr(r.Bio, maxLong)

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

func UserProfileResponseFromModel(p UserProfile) UserProfileResponse {
	return UserProfileResponse{
		ID:              p.ID,
		UserID:          p.UserID,
		Username:        p.Username,
		DisplayName:     p.DisplayName,
		AvatarURL:       p.AvatarURL,
		HasCustomAvatar: p.HasCustomAvatar,
		Bio:             p.Bio,
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
		Bio:         p.Bio,
		Visibility:  p.Visibility,
		Preferences: PreferencesDTO{
			Genres:  append([]string{}, p.Preferences.Genres...),
			Artists: append([]string{}, p.Preferences.Artists...),
			Moods:   append([]string{}, p.Preferences.Moods...),
		},
	}
}
