package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
)

type MockUser struct {
	ID          string                 `json:"id"`
	DisplayName string                 `json:"displayName"`
	Bio         string                 `json:"bio"`
	Visibility  string                 `json:"visibility"`
	Preferences map[string]any         `json:"preferences"`
}

type MockTrack struct {
	Title  string `json:"title"`
	Artist string `json:"artist"`
	Votes  int    `json:"votes,omitempty"`
}

type MockPlaylist struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	OwnerID    string       `json:"ownerId"`
	Visibility string       `json:"visibility"`
	Tracks     []MockTrack  `json:"tracks"`
}

type MockEvent struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Playlist  MockPlaylist `json:"playlist"`
	StartedAt time.Time    `json:"startedAt"`
}

type InitialData struct {
	User      MockUser       `json:"user"`
	Playlists []MockPlaylist `json:"playlists"`
	Events    []MockEvent    `json:"events"`
}

func main() {
	port := getenv("PORT", "3006")

	r := chi.NewRouter()

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"status":  "ok",
			"service": "mock-service",
		})
	})

	// High level initial data for mobile / frontend
	r.Get("/mock/initial", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(sampleInitial())
	})

	// Separate endpoints if нужно дергать по частям
	r.Get("/mock/user", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(sampleUser())
	})

	r.Get("/mock/users/me", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(sampleUser())
	})

	r.Get("/mock/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		u := sampleUser()
		if id != "" {
			u.ID = id
		}
		json.NewEncoder(w).Encode(u)
	})

	r.Get("/mock/playlists", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(samplePlaylists())
	})

	r.Get("/mock/events", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(sampleEvents())
	})

	log.Printf("mock-service on :%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("listen: %v", err)
	}
}

func sampleUser() MockUser {
	return MockUser{
		ID:          "mock-user-1",
		DisplayName: "Mocked DJ",
		Bio:         "This is a fake profile used for UI demos.",
		Visibility:  "public",
		Preferences: map[string]any{
			"genres": []string{"techno", "house", "lofi"},
		},
	}
}

func samplePlaylists() []MockPlaylist {
	return []MockPlaylist{
		{
			ID:         "mock-pl-1",
			Name:       "Chill Vibes",
			OwnerID:    "mock-user-1",
			Visibility: "public",
			Tracks: []MockTrack{
				{Title: "Lofi Track 1", Artist: "Beat Maker"},
				{Title: "Lofi Track 2", Artist: "Beat Maker"},
			},
		},
		{
			ID:         "mock-pl-2",
			Name:       "Party Starter",
			OwnerID:    "mock-user-1",
			Visibility: "public",
			Tracks: []MockTrack{
				{Title: "Banger 1", Artist: "DJ Boom"},
				{Title: "Banger 2", Artist: "DJ Boom"},
			},
		},
	}
}

func sampleEvents() []MockEvent {
	pls := samplePlaylists()
	return []MockEvent{
		{
			ID:        "mock-ev-1",
			Name:      "Friday Night Mock",
			Playlist:  pls[1],
			StartedAt: time.Now().Add(-15 * time.Minute).UTC(),
		},
	}
}

func sampleInitial() InitialData {
	return InitialData{
		User:      sampleUser(),
		Playlists: samplePlaylists(),
		Events:    sampleEvents(),
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
