package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
)

func TestHandleSearchUsers(t *testing.T) {
	s, mock := setupMockServer(t)
	defer mock.Close()

	t.Run("Success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users?query=test", nil)
		w := httptest.NewRecorder()

		// match search query
		mock.ExpectQuery("SELECT.*FROM user_profiles WHERE LOWER").
			WithArgs("%test%").
			WillReturnRows(pgxmock.NewRows([]string{
				"id", "user_id", "display_name", "username",
				"avatar_url", "has_custom_avatar", "bio",
				"visibility", "preferences", "is_premium", "created_at", "updated_at",
			}).AddRow(
				"pid1", "id1", "Test User 1", "testuser1",
				"url1", false, "bio1",
				"public", []byte(`{}`), false, time.Now(), time.Now(),
			).AddRow(
				"pid2", "id2", "Test User 2", "testuser2",
				"url2", true, "bio2",
				"public", []byte(`{}`), false, time.Now(), time.Now(),
			))

		s.handleSearchUsers(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			Items []FriendItem `json:"items"`
		}
		json.Unmarshal(w.Body.Bytes(), &resp)

		assert.Len(t, resp.Items, 2)
		assert.Equal(t, "testuser1", resp.Items[0].Username)
		assert.Equal(t, "testuser2", resp.Items[1].Username)
	})

	t.Run("EmptyQuery", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users?query=", nil)
		w := httptest.NewRecorder()

		s.handleSearchUsers(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "query is required")
	})

	t.Run("QueryTooLong", func(t *testing.T) {
		longQuery := strings.Repeat("a", 51)
		req := httptest.NewRequest("GET", "/users?query="+longQuery, nil)
		w := httptest.NewRecorder()

		s.handleSearchUsers(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "query too long")
	})
}
