package provider

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestParseISO8601Duration(t *testing.T) {
	tests := []struct {
		input    string
		expected int // ms
	}{
		{"PT3M4S", 184000},
		{"PT1H", 3600000},
		{"PT1H30M", 5400000},
		{"PT1M30S", 90000},
		{"PT45S", 45000},
		{"P1DT1H", 0}, // Days not supported by regex, should return 0 or handle? Regex expects PT
		{"invalid", 0},
		{"", 0},
		{"PT1H1M1S", 3661000},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseISO8601Duration(tt.input)
			if got != tt.expected {
				t.Errorf("parseISO8601Duration(%q) = %d; want %d", tt.input, got, tt.expected)
			}
		})
	}
}

// Mock HTTP Transport
type RoundTripFunc func(req *http.Request) *http.Response

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func NewMockClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: fn,
	}
}

func TestSearchTracks(t *testing.T) {
	mockTransport := RoundTripFunc(func(req *http.Request) *http.Response {
		if strings.Contains(req.URL.Path, "/search") {
			// Return search results
			jsonBody := `{
				"items": [
					{
						"id": { "videoId": "vid1" },
						"snippet": { "title": "Track 1", "channelTitle": "Artist 1", "thumbnails": { "high": { "url": "http://img" } } }
					},
					{
						"id": { "videoId": "vid2" },
						"snippet": { "title": "Track 2", "channelTitle": "Artist 2", "thumbnails": { "high": { "url": "http://img" } } }
					}
				]
			}`
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(jsonBody)),
				Header:     make(http.Header),
			}
		}
		if strings.Contains(req.URL.Path, "/videos") {
			// Return video details with duration
			// vid1: PT3M (180000ms), vid2: PT1M30S (90000ms)
			jsonBody := `{
				"items": [
					{
						"id": "vid1",
						"contentDetails": { "duration": "PT3M" }
					},
					{
						"id": "vid2",
						"contentDetails": { "duration": "PT1M30S" }
					}
				]
			}`
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(jsonBody)),
				Header:     make(http.Header),
			}
		}
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader(""))}
	})

	client := NewYouTubeClient("apikey", "https://mock.com/search")
	client.http = NewMockClient(mockTransport)

	items, err := client.SearchTracks(context.Background(), "query", 10)
	if err != nil {
		t.Fatalf("SearchTracks returned error: %v", err)
	}

	if len(items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(items))
	}

	// Check Item 1
	if items[0].ProviderTrackID != "vid1" {
		t.Errorf("Expected vid1, got %s", items[0].ProviderTrackID)
	}
	if items[0].DurationMs != 180000 {
		t.Errorf("Expected vid1 duration 180000, got %d", items[0].DurationMs)
	}

	// Check Item 2
	if items[1].DurationMs != 90000 {
		t.Errorf("Expected vid2 duration 90000, got %d", items[1].DurationMs)
	}
}
