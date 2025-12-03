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
	apiKey    string
	searchURL string
	http      *http.Client
}

func NewYouTubeClient(apiKey, searchURL string) *YouTubeClient {
	return &YouTubeClient{
		apiKey:    apiKey,
		searchURL: searchURL,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type ytSearchResponse struct {
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

func (c *YouTubeClient) SearchTracks(ctx context.Context, query string, limit int) ([]MusicSearchItem, error) {
	if limit <= 0 || limit > 25 {
		limit = 10
	}

	val := url.Values{}
	val.Set("part", "snippet")
	val.Set("type", "video")
	val.Set("maxResults", fmt.Sprint(limit))
	val.Set("q", query)
	val.Set("key", c.apiKey)

	url := c.searchURL + "?" + val.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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

	var body ytSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	out := make([]MusicSearchItem, 0, len(body.Items))
	for _, it := range body.Items {
		thumbs := it.Snippet.Thumbnails
		thumb := thumbs.High.URL
		if thumb == "" {
			thumb = thumbs.Medium.URL
		}
		if thumb == "" {
			thumb = thumbs.Default.URL
		}

		out = append(out, MusicSearchItem{
			Title:           it.Snippet.Title,
			Artist:          it.Snippet.ChannelTitle,
			Provider:        "youtube",
			ProviderTrackID: it.ID.VideoID,
			ThumbnailURL:    thumb,
		})
	}

	return out, nil
}
