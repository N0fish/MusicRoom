package provider

type MusicSearchItem struct {
	Title           string `json:"title"`
	Artist          string `json:"artist"`          // channel / artist name
	Provider        string `json:"provider"`        // "youtube"
	ProviderTrackID string `json:"providerTrackId"` // YouTube video ID
	ThumbnailURL    string `json:"thumbnailUrl"`    // best available thumbnail
	DurationMs      int    `json:"durationMs,omitempty"`
}

type SearchResponse struct {
	Items []MusicSearchItem `json:"items"`
}
