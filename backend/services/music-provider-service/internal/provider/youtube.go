package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type YouTubeClient struct {
	apiKey string
	http   *http.Client
}

func NewYouTubeClient(apiKey string) *YouTubeClient {
	return &YouTubeClient{
		apiKey: apiKey,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type MusicTrack struct {
	Provider        string `json:"provider"`
	ProviderTrackID string `json:"providerTrackId"`
	Title           string `json:"title"`
	Artist          string `json:"artist"`
	ThumbnailURL    string `json:"thumbnailUrl"`
	DurationSec     int    `json:"durationSec,omitempty"`
}

func (c *YouTubeClient) SearchTracks(ctx context.Context, query string, limit int) ([]MusicTrack, error) {
	v := url.Values{}
	v.Set("part", "snippet")
	v.Set("type", "video")
	v.Set("maxResults", fmt.Sprint(limit))
	v.Set("q", query)
	v.Set("key", c.apiKey)

	u := "https://www.googleapis.com/youtube/v3/search?" + v.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("youtube status %d", resp.StatusCode)
	}

	var raw struct {
		Items []struct {
			ID struct {
				VideoID string `json:"videoId"`
			} `json:"id"`
			Snippet struct {
				Title        string `json:"title"`
				ChannelTitle string `json:"channelTitle"`
				Thumbnails   struct {
					Default struct {
						URL string `json:"url"`
					} `json:"default"`
					Medium struct {
						URL string `json:"url"`
					} `json:"medium"`
					High struct {
						URL string `json:"url"`
					} `json:"high"`
				} `json:"thumbnails"`
			} `json:"snippet"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	out := make([]MusicTrack, 0, len(raw.Items))
	for _, it := range raw.Items {
		thumb := it.Snippet.Thumbnails.High.URL
		if thumb == "" {
			thumb = it.Snippet.Thumbnails.Medium.URL
		}
		if thumb == "" {
			thumb = it.Snippet.Thumbnails.Default.URL
		}

		out = append(out, MusicTrack{
			Provider:        "youtube",
			ProviderTrackID: it.ID.VideoID,
			Title:           it.Snippet.Title,
			Artist:          it.Snippet.ChannelTitle,
			ThumbnailURL:    thumb,
		})
	}

	return out, nil
}
