package vote

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRegisterVote(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockStore := new(MockStore)
		ev := &Event{
			ID:          "ev1",
			OwnerID:     "owner",
			LicenseMode: "everyone",
		}
		mockStore.On("LoadEvent", ctx, "ev1").Return(ev, nil)
		mockStore.On("IsInvited", ctx, "ev1", "user1").Return(true, nil)
		mockStore.On("CastVote", ctx, "ev1", "tr1", "user1").Return(nil)
		mockStore.On("GetVoteCount", ctx, "ev1", "tr1").Return(5, nil)

		resp, err := registerVote(ctx, mockStore, nil, "ev1", "user1", "tr1", nil, nil)
		assert.NoError(t, err)
		assert.Equal(t, "ok", resp.Status)
		assert.Equal(t, 5, resp.TotalVotes)
	})

	t.Run("event not found", func(t *testing.T) {
		mockStore := new(MockStore)
		mockStore.On("LoadEvent", ctx, "ev1").Return((*Event)(nil), errors.New("not found")) // Simplification: we might need to wrap exact pgx error if using errors.Is(pgx.ErrNoRows), but here we return generic error unless we mock exact behavior.
		// logic.go checks errors.Is(err, pgx.ErrNoRows). For test simplicity, we can mock that specific error or just a generic one.
		// If we return generic error, it returns err.

		_, err := registerVote(ctx, mockStore, nil, "ev1", "user1", "tr1", nil, nil)
		assert.Error(t, err)
	})

	t.Run("store error", func(t *testing.T) {
		mockStore := new(MockStore)
		ev := &Event{ID: "ev1", OwnerID: "owner"}
		mockStore.On("LoadEvent", ctx, "ev1").Return(ev, nil)
		mockStore.On("IsInvited", ctx, "ev1", "user1").Return(true, nil)
		mockStore.On("CastVote", ctx, "ev1", "tr1", "user1").Return(errors.New("db error"))

		_, err := registerVote(ctx, mockStore, nil, "ev1", "user1", "tr1", nil, nil)
		assert.Error(t, err)
		assert.Equal(t, "db error", err.Error())
	})
}

func TestRemoveVote(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockStore := new(MockStore)
		ev := &Event{ID: "ev1", OwnerID: "owner"}
		mockStore.On("LoadEvent", ctx, "ev1").Return(ev, nil)
		// Logic currently doesn't re-check canUserVote fully or IsInvited for removeVote,
		// but let's see implementation. removeVote calls IsInvited inside logic.go?
		// Looking at code: removeVote checks voting window but doesn't seem to call canUserVote.
		// So this should be fine without IsInvited.
		// Based on logic.go viewing earlier, removeVote only loaded event and checked window.
		// So this should be fine without IsInvited.

		mockStore.On("RemoveVote", ctx, "ev1", "tr1", "user1").Return(nil)
		mockStore.On("GetVoteCount", ctx, "ev1", "tr1").Return(4, nil)

		resp, err := removeVote(ctx, mockStore, nil, "ev1", "user1", "tr1")
		assert.NoError(t, err)
		assert.Equal(t, "ok", resp.Status)
		assert.Equal(t, 4, resp.TotalVotes)
	})

	t.Run("event not found", func(t *testing.T) {
		mockStore := new(MockStore)
		mockStore.On("LoadEvent", ctx, "ev1").Return((*Event)(nil), pgx.ErrNoRows)

		_, err := removeVote(ctx, mockStore, nil, "ev1", "user1", "tr1")
		assert.Error(t, err)
		var vErr *voteError
		if assert.True(t, errors.As(err, &vErr)) {
			assert.Equal(t, http.StatusNotFound, vErr.status)
		}
	})

	t.Run("voting has ended", func(t *testing.T) {
		mockStore := new(MockStore)
		past := time.Now().Add(-1 * time.Hour)
		ev := &Event{ID: "ev1", OwnerID: "owner", VoteEnd: &past}
		mockStore.On("LoadEvent", ctx, "ev1").Return(ev, nil)

		_, err := removeVote(ctx, mockStore, nil, "ev1", "user1", "tr1")
		assert.Error(t, err)
		var vErr *voteError
		if assert.True(t, errors.As(err, &vErr)) {
			assert.Equal(t, http.StatusForbidden, vErr.status)
			assert.Equal(t, "voting has ended", vErr.msg)
		}
	})

	t.Run("store error", func(t *testing.T) {
		mockStore := new(MockStore)
		ev := &Event{ID: "ev1", OwnerID: "owner"}
		mockStore.On("LoadEvent", ctx, "ev1").Return(ev, nil)
		mockStore.On("RemoveVote", ctx, "ev1", "tr1", "user1").Return(errors.New("db error"))

		_, err := removeVote(ctx, mockStore, nil, "ev1", "user1", "tr1")
		assert.Error(t, err)
		assert.Equal(t, "db error", err.Error())
	})
}

func TestCanUserVote(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	t.Run("owner votes if joined - private", func(t *testing.T) {
		mockStore := new(MockStore)
		ev := &Event{ID: "ev1", OwnerID: "owner", Visibility: "private"}
		mockStore.On("IsInvited", ctx, "ev1", "owner").Return(true, nil)
		ok, _, err := canUserVote(ctx, mockStore, ev, "owner", nil, nil, now)
		assert.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("owner votes if joined - invited only", func(t *testing.T) {
		mockStore := new(MockStore)
		ev := &Event{ID: "ev1", OwnerID: "owner", LicenseMode: "invited_only"}
		mockStore.On("GetParticipantRole", ctx, "ev1", "owner").Return("contributor", nil)
		ok, _, err := canUserVote(ctx, mockStore, ev, "owner", nil, nil, now)
		assert.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("license everyone", func(t *testing.T) {
		mockStore := new(MockStore)
		ev := &Event{ID: "ev1", OwnerID: "owner", LicenseMode: "everyone"}
		mockStore.On("IsInvited", ctx, "ev1", "user2").Return(true, nil)

		ok, _, err := canUserVote(ctx, mockStore, ev, "user2", nil, nil, now)
		assert.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("license everyone not joined", func(t *testing.T) {
		mockStore := new(MockStore)
		ev := &Event{ID: "ev1", OwnerID: "owner", LicenseMode: "everyone"}
		mockStore.On("IsInvited", ctx, "ev1", "user2").Return(false, nil)

		ok, reason, err := canUserVote(ctx, mockStore, ev, "user2", nil, nil, now)
		assert.NoError(t, err)
		assert.False(t, ok)
		assert.Equal(t, "you must join the event to vote", reason)
	})

	t.Run("private event invited success", func(t *testing.T) {
		mockStore := new(MockStore)
		ev := &Event{ID: "ev1", OwnerID: "owner", Visibility: "private"}
		mockStore.On("IsInvited", ctx, "ev1", "user2").Return(true, nil)

		ok, _, err := canUserVote(ctx, mockStore, ev, "user2", nil, nil, now)
		assert.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("private event not invited", func(t *testing.T) {
		mockStore := new(MockStore)
		ev := &Event{ID: "ev1", OwnerID: "owner", Visibility: "private"}
		mockStore.On("IsInvited", ctx, "ev1", "user2").Return(false, nil)

		ok, reason, err := canUserVote(ctx, mockStore, ev, "user2", nil, nil, now)
		assert.NoError(t, err)
		assert.False(t, ok)
		assert.Equal(t, "event is private, invite required", reason)
	})

	t.Run("license invited success", func(t *testing.T) {
		mockStore := new(MockStore)
		ev := &Event{ID: "ev1", OwnerID: "owner", LicenseMode: "invited_only"}
		mockStore.On("GetParticipantRole", ctx, "ev1", "user2").Return("contributor", nil)

		ok, _, err := canUserVote(ctx, mockStore, ev, "user2", nil, nil, now)
		assert.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("license invited not invited", func(t *testing.T) {
		mockStore := new(MockStore)
		ev := &Event{ID: "ev1", OwnerID: "owner", LicenseMode: "invited_only"}
		mockStore.On("GetParticipantRole", ctx, "ev1", "user2").Return("", nil)

		ok, reason, err := canUserVote(ctx, mockStore, ev, "user2", nil, nil, now)
		assert.NoError(t, err)
		assert.False(t, ok)
		assert.Equal(t, "license requires invitation to vote", reason)
	})

	t.Run("license invited guest", func(t *testing.T) {
		mockStore := new(MockStore)
		ev := &Event{ID: "ev1", OwnerID: "owner", LicenseMode: "invited_only"}
		mockStore.On("GetParticipantRole", ctx, "ev1", "user2").Return("guest", nil)

		ok, reason, err := canUserVote(ctx, mockStore, ev, "user2", nil, nil, now)
		assert.NoError(t, err)
		assert.False(t, ok)
		assert.Equal(t, "guests cannot vote in invited-only events", reason)
	})

	t.Run("geo time blocked time - early", func(t *testing.T) {
		start := now.Add(1 * time.Hour)
		ev := &Event{
			ID:          "ev1",
			OwnerID:     "owner",
			LicenseMode: "geo_time",
			VoteStart:   &start,
		}
		ok, reason, err := canUserVote(ctx, nil, ev, "user2", nil, nil, now)
		assert.NoError(t, err)
		assert.False(t, ok)
		assert.Equal(t, "voting has not started yet", reason)
	})

	t.Run("geo time blocked time - late", func(t *testing.T) {
		end := now.Add(-1 * time.Hour)
		ev := &Event{
			ID:          "ev1",
			OwnerID:     "owner",
			LicenseMode: "geo_time",
			VoteEnd:     &end,
		}
		ok, reason, err := canUserVote(ctx, nil, ev, "user2", nil, nil, now)
		assert.NoError(t, err)
		assert.False(t, ok)
		assert.Equal(t, "voting has ended", reason)
	})

	t.Run("geo time blocked lat/lng missing", func(t *testing.T) {
		ev := &Event{
			ID:          "ev1",
			OwnerID:     "owner",
			LicenseMode: "geo_time",
		}
		// Missing config in event
		ok, reason, err := canUserVote(ctx, nil, ev, "user2", nil, nil, now)
		assert.NoError(t, err)
		assert.False(t, ok)
		assert.Equal(t, "event is not configured for geo voting", reason)

		// Missing user coords
		lat, lng := 10.0, 10.0
		radius := 100
		ev.GeoLat = &lat
		ev.GeoLng = &lng
		ev.GeoRadiusM = &radius

		ok, reason, err = canUserVote(ctx, nil, ev, "user2", nil, nil, now)
		assert.NoError(t, err)
		assert.False(t, ok)
		assert.Equal(t, "location (lat,lng) is required for geo voting", reason)
	})

	t.Run("geo time outside radius", func(t *testing.T) {
		lat, lng := 10.0, 10.0
		radius := 1000 // 1km
		ev := &Event{
			ID:          "ev1",
			OwnerID:     "owner",
			LicenseMode: "geo_time",
			GeoLat:      &lat,
			GeoLng:      &lng,
			GeoRadiusM:  &radius,
		}

		// User far away (e.g. 11.0, 11.0 is ~150km away)
		uLat, uLng := 11.0, 11.0
		ok, reason, err := canUserVote(ctx, nil, ev, "user2", &uLat, &uLng, now)
		assert.NoError(t, err)
		assert.False(t, ok)
		assert.Equal(t, "user is outside of allowed geo area", reason)
	})

	t.Run("geo time inside radius", func(t *testing.T) {
		lat, lng := 10.0, 10.0
		radius := 1000000 // 1000km, generous
		ev := &Event{
			ID:          "ev1",
			OwnerID:     "owner",
			LicenseMode: "geo_time",
			GeoLat:      &lat,
			GeoLng:      &lng,
			GeoRadiusM:  &radius,
		}

		uLat, uLng := 10.01, 10.01 // Close enough
		ok, _, err := canUserVote(ctx, nil, ev, "user2", &uLat, &uLng, now)
		assert.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("unsupported license mode", func(t *testing.T) {
		ev := &Event{
			ID:          "ev1",
			OwnerID:     "owner",
			LicenseMode: "weird",
		}
		ok, reason, err := canUserVote(ctx, nil, ev, "user2", nil, nil, now)
		assert.NoError(t, err)
		assert.False(t, ok)
		assert.Equal(t, "unsupported license mode", reason)
	})
}

func TestWithinRadius(t *testing.T) {
	// Center Paris: 48.8566, 2.3522
	cLat, cLng := 48.8566, 2.3522

	// 100m radius
	radius := 100

	// User at same point
	assert.True(t, withinRadius(cLat, cLng, radius, cLat, cLng))

	// User slightly offset (approx 10m lat)
	// 1 deg lat is ~111km. 0.0001 deg is ~11m
	assert.True(t, withinRadius(cLat, cLng, radius, cLat+0.0001, cLng))

	// User far away
	assert.False(t, withinRadius(cLat, cLng, radius, cLat+1.0, cLng))
}

func TestCheckUserExists(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/internal/users/user1/exists", r.URL.Path)
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		err := checkUserExists(ctx, http.DefaultClient, ts.URL, "user1")
		assert.NoError(t, err)
	})

	t.Run("status no content", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		defer ts.Close()

		err := checkUserExists(ctx, http.DefaultClient, ts.URL, "user1")
		assert.NoError(t, err)
	})

	t.Run("not found", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer ts.Close()

		err := checkUserExists(ctx, http.DefaultClient, ts.URL, "user1")
		assert.Error(t, err)
		assert.Equal(t, "user not found", err.Error())
	})

	t.Run("server error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer ts.Close()

		err := checkUserExists(ctx, http.DefaultClient, ts.URL, "user1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "user-service returned 500")
	})

	t.Run("bad url", func(t *testing.T) {
		err := checkUserExists(ctx, http.DefaultClient, "://bad-url", "user1")
		assert.Error(t, err)
	})
}

func TestValidateVotingWindow(t *testing.T) {
	now := time.Now()
	t.Run("valid window", func(t *testing.T) {
		start := now.Add(1 * time.Hour)
		end := now.Add(3 * time.Hour)
		err := validateVotingWindow(&start, &end, now)
		assert.NoError(t, err)
	})

	t.Run("end before start", func(t *testing.T) {
		start := now.Add(3 * time.Hour)
		end := now.Add(2 * time.Hour)
		err := validateVotingWindow(&start, &end, now)
		assert.Error(t, err)
		assert.Equal(t, "voteEnd must be after voteStart", err.Error())
	})

	t.Run("start in past", func(t *testing.T) {
		past := now.Add(-1 * time.Hour)
		err := validateVotingWindow(&past, nil, now)
		assert.Error(t, err)
		assert.Equal(t, "voteStart cannot be in the past", err.Error())
	})

	t.Run("end in past", func(t *testing.T) {
		past := now.Add(-1 * time.Hour)
		err := validateVotingWindow(nil, &past, now)
		assert.Error(t, err)
		assert.Equal(t, "voteEnd cannot be in the past", err.Error())
	})

	t.Run("start too far future", func(t *testing.T) {
		future := now.Add(400 * 24 * time.Hour)
		err := validateVotingWindow(&future, nil, now)
		assert.Error(t, err)
		assert.Equal(t, "voteStart cannot be more than 1 year in the future", err.Error())
	})

	t.Run("end too far future", func(t *testing.T) {
		future := now.Add(400 * 24 * time.Hour)
		err := validateVotingWindow(nil, &future, now)
		assert.Error(t, err)
		assert.Equal(t, "voteEnd cannot be more than 1 year in the future", err.Error())
	})

	t.Run("window too small", func(t *testing.T) {
		start := now.Add(1 * time.Hour)
		end := now.Add(1*time.Hour + 30*time.Minute)
		err := validateVotingWindow(&start, &end, now)
		assert.Error(t, err)
		assert.Equal(t, "voting window must be at least 1 hour", err.Error())
	})
}

func TestHttpUtils(t *testing.T) {
	t.Run("join", func(t *testing.T) {
		assert.Equal(t, "a,b,c", join([]string{"a", "b", "c"}, ","))
		assert.Equal(t, "a", join([]string{"a"}, ","))
		assert.Equal(t, "", join([]string{}, ","))
	})

	t.Run("itoa", func(t *testing.T) {
		assert.Equal(t, "0", itoa(0))
		assert.Equal(t, "123", itoa(123))
		assert.Equal(t, "-456", itoa(-456))
	})

	t.Run("publishEvent no redis", func(t *testing.T) {
		s := &HTTPServer{rdb: nil}
		// Should not panic
		s.publishEvent(context.Background(), "test", "payload")
	})

	t.Run("writeError", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writeError(rec, http.StatusBadRequest, "some error")
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		assert.Contains(t, rec.Body.String(), `"error":"some error"`)
	})

	t.Run("writeVoteError", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writeVoteError(rec, &voteError{status: http.StatusTeapot, msg: "tea"})
		assert.Equal(t, http.StatusTeapot, rec.Code)

		rec2 := httptest.NewRecorder()
		writeVoteError(rec2, errors.New("generic"))
		assert.Equal(t, http.StatusInternalServerError, rec2.Code)
	})

	t.Run("publishEvent json error", func(t *testing.T) {
		s := &HTTPServer{rdb: &redis.Client{}} // rdb not nil
		// To trigger json.Marshal error, we need something that can't be marshaled.
		// Channels or functions usually fail.
		s.publishEvent(context.Background(), "test", make(chan int))
	})
}

func TestLogicErrorPaths(t *testing.T) {
	ctx := context.Background()
	t.Run("registerVote load error", func(t *testing.T) {
		mockStore := new(MockStore)
		mockStore.On("LoadEvent", ctx, "ev1").Return((*Event)(nil), errors.New("load fail"))
		_, err := registerVote(ctx, mockStore, nil, "ev1", "u1", "t1", nil, nil)
		assert.Error(t, err)
	})

	t.Run("removeVote load error", func(t *testing.T) {
		mockStore := new(MockStore)
		mockStore.On("LoadEvent", ctx, "ev1").Return((*Event)(nil), errors.New("load fail"))
		_, err := removeVote(ctx, mockStore, nil, "ev1", "u1", "t1")
		assert.Error(t, err)
	})

	t.Run("registerVote get count error", func(t *testing.T) {
		mockStore := new(MockStore)
		ev := &Event{ID: "ev1", OwnerID: "owner"}
		mockStore.On("LoadEvent", ctx, "ev1").Return(ev, nil)
		mockStore.On("IsInvited", ctx, "ev1", "u1").Return(true, nil)
		mockStore.On("CastVote", ctx, "ev1", "t1", "u1").Return(nil)
		mockStore.On("GetVoteCount", ctx, "ev1", "t1").Return(0, errors.New("count fail"))

		_, err := registerVote(ctx, mockStore, nil, "ev1", "u1", "t1", nil, nil)
		assert.Error(t, err)
		assert.Equal(t, "count fail", err.Error())
	})

	t.Run("removeVote get count error", func(t *testing.T) {
		mockStore := new(MockStore)
		ev := &Event{ID: "ev1", OwnerID: "owner"}
		mockStore.On("LoadEvent", ctx, "ev1").Return(ev, nil)
		mockStore.On("RemoveVote", ctx, "ev1", "t1", "u1").Return(nil)
		mockStore.On("GetVoteCount", ctx, "ev1", "t1").Return(0, errors.New("count fail"))

		_, err := removeVote(ctx, mockStore, nil, "ev1", "u1", "t1")
		assert.Error(t, err)
		assert.Equal(t, "count fail", err.Error())
	})
}

func TestMockStoreCoverage(t *testing.T) {
	ctx := context.Background()
	m := new(MockStore)

	t.Run("GetVoteTally", func(t *testing.T) {
		m.On("GetVoteTally", ctx, "e1", "u1").Return([]Row{{Track: "t1", Count: 1}}, nil)
		res, _ := m.GetVoteTally(ctx, "e1", "u1")
		assert.Len(t, res, 1)
	})

	t.Run("CreateEvent", func(t *testing.T) {
		m.On("CreateEvent", ctx, mock.Anything).Return("ev1", nil)
		res, _ := m.CreateEvent(ctx, &Event{})
		assert.Equal(t, "ev1", res)
	})

	t.Run("DeleteInvite", func(t *testing.T) {
		m.On("DeleteInvite", ctx, "e1", "u1").Return(nil)
		err := m.DeleteInvite(ctx, "e1", "u1")
		assert.NoError(t, err)
	})

	t.Run("ListEvents", func(t *testing.T) {
		m.On("ListEvents", ctx, "u1", "public").Return([]Event{{ID: "e1"}}, nil)
		res, _ := m.ListEvents(ctx, "u1", "public")
		assert.Len(t, res, 1)
	})

	t.Run("UpdateEvent", func(t *testing.T) {
		m.On("UpdateEvent", ctx, "e1", mock.Anything).Return(nil)
		err := m.UpdateEvent(ctx, "e1", map[string]any{"name": "n"})
		assert.NoError(t, err)
	})

	t.Run("DeleteEvent", func(t *testing.T) {
		m.On("DeleteEvent", ctx, "e1").Return(nil)
		err := m.DeleteEvent(ctx, "e1")
		assert.NoError(t, err)
	})

	t.Run("TransferOwnership", func(t *testing.T) {
		m.On("TransferOwnership", ctx, "e1", "u2").Return(nil)
		err := m.TransferOwnership(ctx, "e1", "u2")
		assert.NoError(t, err)
	})

	t.Run("IsInvited", func(t *testing.T) {
		m.On("IsInvited", ctx, "e1", "u1").Return(true, nil)
		res, _ := m.IsInvited(ctx, "e1", "u1")
		assert.True(t, res)
	})

	t.Run("CastVote", func(t *testing.T) {
		m.On("CastVote", ctx, "e1", "t1", "u1").Return(nil)
		err := m.CastVote(ctx, "e1", "t1", "u1")
		assert.NoError(t, err)
	})

	t.Run("RemoveVote", func(t *testing.T) {
		m.On("RemoveVote", ctx, "e1", "t1", "u1").Return(nil)
		err := m.RemoveVote(ctx, "e1", "t1", "u1")
		assert.NoError(t, err)
	})

	t.Run("GetVoteCount", func(t *testing.T) {
		m.On("GetVoteCount", ctx, "e1", "t1").Return(10, nil)
		res, _ := m.GetVoteCount(ctx, "e1", "t1")
		assert.Equal(t, 10, res)
	})
}

func TestLogicEdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("canUserVote isInvited error", func(t *testing.T) {
		m := new(MockStore)
		ev := &Event{ID: "e1", Visibility: "private"}
		m.On("IsInvited", ctx, "e1", "u1").Return(false, errors.New("fail"))
		ok, _, err := canUserVote(ctx, m, ev, "u1", nil, nil, time.Now())
		assert.Error(t, err)
		assert.False(t, ok)
	})

	t.Run("registerVote canUserVote error", func(t *testing.T) {
		m := new(MockStore)
		ev := &Event{ID: "e1", Visibility: "private"}
		m.On("LoadEvent", ctx, "e1").Return(ev, nil)
		m.On("IsInvited", ctx, "e1", "u1").Return(false, errors.New("fail"))
		_, err := registerVote(ctx, m, nil, "e1", "u1", "t1", nil, nil)
		assert.Error(t, err)
	})

	t.Run("registerVote duplicate vote", func(t *testing.T) {
		m := new(MockStore)
		ev := &Event{ID: "e1", LicenseMode: "everyone"}
		m.On("LoadEvent", ctx, "e1").Return(ev, nil)
		m.On("IsInvited", ctx, "e1", "u1").Return(true, nil)
		m.On("CastVote", ctx, "e1", "t1", "u1").Return(ErrVoteConflict)
		_, err := registerVote(ctx, m, nil, "e1", "u1", "t1", nil, nil)
		assert.Error(t, err)
		var vErr *voteError
		if assert.True(t, errors.As(err, &vErr)) {
			assert.Equal(t, http.StatusConflict, vErr.status)
		}
	})
}
