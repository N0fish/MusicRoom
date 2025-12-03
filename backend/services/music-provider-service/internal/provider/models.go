package provider

type MusicSearchItem struct {
	Title           string `json:"title"`
	Artist          string `json:"artist"`          // channel / artist name
	Provider        string `json:"provider"`        // "youtube"
	ProviderTrackID string `json:"providerTrackId"` // YouTube video ID
	ThumbnailURL    string `json:"thumbnailUrl"`    // best available thumbnail
	DurationSec     int    `json:"durationSec,omitempty"`
}

type SearchResponse struct {
	Items []MusicSearchItem `json:"items"`
}
