package vote

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const playlistServiceURL = "http://playlist-service:3002"

type playlistTrack struct {
	ID              string `json:"id"`
	ProviderTrackID string `json:"providerTrackId"`
	Position        int    `json:"position"`
}

func createPlaylist(ctx context.Context, ownerID, name string) (string, error) {
	body := map[string]any{
		"name":        name,
		"description": "Created automatically for event " + name,
		"isPublic":    true,
		"editMode":    "everyone",
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, "POST", playlistServiceURL+"/playlists", bytes.NewReader(b))
	req.Header.Set("X-User-Id", ownerID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		return "", fmt.Errorf("create playlist failed: %d", resp.StatusCode)
	}

	var res struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	return res.ID, nil
}

func addTrackToPlaylist(ctx context.Context, userID, playlistID string, trackInfo map[string]any) error {
	reqBody := map[string]any{
		"title":           trackInfo["title"],
		"artist":          trackInfo["artist"],
		"provider":        trackInfo["provider"],
		"providerTrackId": trackInfo["id"],
		"thumbnailUrl":    trackInfo["thumbnailUrl"],
	}

	b, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/playlists/%s/tracks", playlistServiceURL, playlistID), bytes.NewReader(b))
	req.Header.Set("X-User-Id", userID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		return fmt.Errorf("add track failed: %d", resp.StatusCode)
	}
	return nil
}

func getPlaylistTracks(ctx context.Context, userID, playlistID string) ([]playlistTrack, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/playlists/%s", playlistServiceURL, playlistID), nil)
	req.Header.Set("X-User-Id", userID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("get playlist failed: %d", resp.StatusCode)
	}

	var res struct {
		Tracks []playlistTrack `json:"tracks"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return res.Tracks, nil
}

func moveTrack(ctx context.Context, userID, playlistID, trackID string, newPos int) error {
	body := map[string]any{
		"newPosition": newPos,
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, "PATCH", fmt.Sprintf("%s/playlists/%s/tracks/%s", playlistServiceURL, playlistID, trackID), bytes.NewReader(b))
	req.Header.Set("X-User-Id", userID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("move track failed: %d", resp.StatusCode)
	}
	return nil
}
