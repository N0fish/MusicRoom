package playlist

import (
	"time"
)

// Playlist represents a logical playlist used by the Playlist Editor service.
// It intentionally contains only metadata; tracks are modelled separately.
type Playlist struct {
	ID          string    `json:"id"`
	OwnerID     string    `json:"ownerId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsPublic    bool      `json:"isPublic"`
	EditMode    string    `json:"editMode"` // "everyone" | "invited"
	CreatedAt   time.Time `json:"createdAt"`
}

// Track belongs to a playlist. Tracks are ordered by Position (0-based).
type Track struct {
	ID         string    `json:"id"`
	PlaylistID string    `json:"playlistId"`
	Title      string    `json:"title"`
	Artist     string    `json:"artist"`
	Position   int       `json:"position"`
	CreatedAt  time.Time `json:"createdAt"`

	Provider        string `json:"provider,omitempty"`        // "youtube"
	ProviderTrackID string `json:"providerTrackId,omitempty"` // ID ролика/трека у провайдера
	ThumbnailURL    string `json:"thumbnailUrl,omitempty"`    // постер с YouTube
}

// PlaylistInvite represents an invited user to a playlist.
type PlaylistInvite struct {
	UserID    string    `json:"userId"`
	CreatedAt time.Time `json:"createdAt"`
}

const (
	editModeEveryone = "everyone"
	editModeInvited  = "invited"
)
