package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealth(t *testing.T) {
	router := SetupRouter()
	req, _ := http.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]any
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "ok", response["status"])
	assert.Equal(t, "mock-service", response["service"])
}

func TestMockInitial(t *testing.T) {
	router := SetupRouter()
	req, _ := http.NewRequest("GET", "/mock/initial", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var data InitialData
	err := json.Unmarshal(rr.Body.Bytes(), &data)
	assert.NoError(t, err)
	assert.NotEmpty(t, data.User.ID)
	assert.NotEmpty(t, data.Playlists)
	assert.NotEmpty(t, data.Events)
}

func TestMockUser(t *testing.T) {
	router := SetupRouter()

	endpoints := []string{"/mock/user", "/mock/users/me"}

	for _, path := range endpoints {
		t.Run(path, func(t *testing.T) {
			req, _ := http.NewRequest("GET", path, nil)
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var user MockUser
			err := json.Unmarshal(rr.Body.Bytes(), &user)
			assert.NoError(t, err)
			assert.Equal(t, "mock-user-1", user.ID)
		})
	}
}

func TestMockUsersByID(t *testing.T) {
	router := SetupRouter()
	testID := "custom-id-123"
	req, _ := http.NewRequest("GET", "/mock/users/"+testID, nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var user MockUser
	err := json.Unmarshal(rr.Body.Bytes(), &user)
	assert.NoError(t, err)
	assert.Equal(t, testID, user.ID)
}

func TestMockPlaylists(t *testing.T) {
	router := SetupRouter()
	req, _ := http.NewRequest("GET", "/mock/playlists", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var playlists []MockPlaylist
	err := json.Unmarshal(rr.Body.Bytes(), &playlists)
	assert.NoError(t, err)
	assert.True(t, len(playlists) > 0)
}

func TestMockEvents(t *testing.T) {
	router := SetupRouter()
	req, _ := http.NewRequest("GET", "/mock/events", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var events []MockEvent
	err := json.Unmarshal(rr.Body.Bytes(), &events)
	assert.NoError(t, err)
	assert.True(t, len(events) > 0)
}
