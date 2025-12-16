package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
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
	var videoIDs []string

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
		videoIDs = append(videoIDs, it.ID.VideoID)
	}

	// Fetch durations
	if len(videoIDs) > 0 {
		durations, err := c.fetchDurations(ctx, videoIDs)
		if err == nil {
			for i := range out {
				if d, ok := durations[out[i].ProviderTrackID]; ok {
					out[i].DurationMs = d
				}
			}
		} else {
			// Log error but return results without duration
			log.Printf("youtube fetch durations error: %v\n", err)
		}
	}

	return out, nil
}

type ytVideosResponse struct {
	Items []struct {
		ID             string `json:"id"`
		ContentDetails struct {
			Duration string `json:"duration"`
		} `json:"contentDetails"`
	} `json:"items"`
}

func (c *YouTubeClient) fetchDurations(ctx context.Context, ids []string) (map[string]int, error) {
	val := url.Values{}
	val.Set("part", "contentDetails")
	val.Set("id", strings.Join(ids, ","))
	val.Set("key", c.apiKey)

	// Construct base URL for videos endpoint
	// Assuming searchURL is like "https://www.googleapis.com/youtube/v3/search"
	// We need "https://www.googleapis.com/youtube/v3/videos"
	baseURL := "https://www.googleapis.com/youtube/v3/videos"
	if len(c.searchURL) > 7 && c.searchURL[len(c.searchURL)-7:] == "/search" {
		baseURL = c.searchURL[:len(c.searchURL)-7] + "/videos"
	}

	reqURL := baseURL + "?" + val.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("youtube videos status %d", resp.StatusCode)
	}

	var body ytVideosResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	durations := make(map[string]int)
	for _, item := range body.Items {
		durations[item.ID] = parseISO8601Duration(item.ContentDetails.Duration)
	}
	return durations, nil
}

func parseISO8601Duration(duration string) int {
	// Simple regex parser for PT#M#S or PT#H#M#S
	re := regexp.MustCompile(`PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?`)
	matches := re.FindStringSubmatch(duration)

	if len(matches) < 4 {
		return 0
	}

	var h, m, s int
	fmt.Sscanf(matches[1], "%d", &h)
	fmt.Sscanf(matches[2], "%d", &m)
	fmt.Sscanf(matches[3], "%d", &s)

	return (h*3600 + m*60 + s) * 1000
}
