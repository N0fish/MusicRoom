package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockProvider struct {
	mock.Mock
}

func (m *MockProvider) SearchTracks(ctx context.Context, query string, limit int) ([]MusicSearchItem, error) {
	args := m.Called(ctx, query, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]MusicSearchItem), args.Error(1)
}

func TestHandleSearch(t *testing.T) {
	t.Run("success youtube", func(t *testing.T) {
		mockP := new(MockProvider)
		srv := NewServer(mockP, nil)

		expectedItems := []MusicSearchItem{
			{
				Title:           "Test Track",
				Artist:          "Test Artist",
				Provider:        "youtube",
				ProviderTrackID: "abc123",
				ThumbnailURL:    "http://example.com/thumb.jpg",
				DurationMs:      120000,
			},
		}

		mockP.On("SearchTracks", mock.Anything, "test query", 10).Return(expectedItems, nil)

		req, _ := http.NewRequest("GET", "/music/search?query=test%20query", nil)
		rr := httptest.NewRecorder()

		srv.HandleSearch(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var resp SearchResponse
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, expectedItems, resp.Items)
		mockP.AssertExpectations(t)
	})

	t.Run("missing query", func(t *testing.T) {
		srv := NewServer(new(MockProvider), nil)
		req, _ := http.NewRequest("GET", "/music/search", nil)
		rr := httptest.NewRecorder()

		srv.HandleSearch(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "query is required")
	})

	t.Run("query too long", func(t *testing.T) {
		srv := NewServer(new(MockProvider), nil)
		longQuery := "a"
		for i := 0; i < 201; i++ {
			longQuery += "a"
		}
		req, _ := http.NewRequest("GET", "/music/search?query="+longQuery, nil)
		rr := httptest.NewRecorder()

		srv.HandleSearch(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "too long")
	})

	t.Run("unsupported provider", func(t *testing.T) {
		srv := NewServer(new(MockProvider), nil)
		req, _ := http.NewRequest("GET", "/music/search?query=test&provider=spotify", nil)
		rr := httptest.NewRecorder()

		srv.HandleSearch(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "unsupported provider")
	})

	t.Run("provider error", func(t *testing.T) {
		mockP := new(MockProvider)
		srv := NewServer(mockP, nil)

		mockP.On("SearchTracks", mock.Anything, "test", 10).Return(nil, errors.New("provider down"))

		req, _ := http.NewRequest("GET", "/music/search?query=test", nil)
		rr := httptest.NewRecorder()

		srv.HandleSearch(rr, req)

		assert.Equal(t, http.StatusBadGateway, rr.Code)
		assert.Contains(t, rr.Body.String(), "failed to query provider")
		mockP.AssertExpectations(t)
	})

	t.Run("custom limit", func(t *testing.T) {
		mockP := new(MockProvider)
		srv := NewServer(mockP, nil)

		mockP.On("SearchTracks", mock.Anything, "test", 5).Return([]MusicSearchItem{}, nil)

		req, _ := http.NewRequest("GET", "/music/search?query=test&limit=5", nil)
		rr := httptest.NewRecorder()

		srv.HandleSearch(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		mockP.AssertExpectations(t)
	})
}

func TestHandleHealth(t *testing.T) {
	srv := NewServer(nil, nil)
	req, _ := http.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	srv.HandleHealth(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "ok")
	assert.Contains(t, rr.Body.String(), "music-provider-service")
}
