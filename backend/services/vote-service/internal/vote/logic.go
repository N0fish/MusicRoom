package vote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

func registerVote(ctx context.Context, store Store, rdb *redis.Client, eventID, voterID, trackID string, lat, lng *float64) (*VoteResponse, error) {
	ev, err := store.LoadEvent(ctx, eventID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &voteError{status: http.StatusNotFound, msg: "event not found"}
		}
		return nil, err
	}

	if ok, reason, err := canUserVote(ctx, store, ev, voterID, lat, lng, time.Now()); err != nil {
		return nil, err
	} else if !ok {
		return nil, &voteError{status: http.StatusForbidden, msg: reason}
	}

	if err := store.CastVote(ctx, eventID, trackID, voterID); err != nil {
		if errors.Is(err, ErrVoteConflict) {
			return nil, &voteError{status: http.StatusConflict, msg: "duplicate vote"}
		}
		return nil, err
	}

	total, err := store.GetVoteCount(ctx, eventID, trackID)
	if err != nil {
		return nil, err
	}

	evt := map[string]any{
		"type": "vote.cast",
		"payload": map[string]any{
			"eventId":    eventID,
			"trackId":    trackID,
			"voterId":    voterID,
			"totalVotes": total,
		},
	}
	if b, err := json.Marshal(evt); err == nil && rdb != nil {
		_ = rdb.Publish(ctx, "broadcast", string(b)).Err()
	}

	return &VoteResponse{
		Status:     "ok",
		TrackID:    trackID,
		TotalVotes: total,
	}, nil
}

func removeVote(ctx context.Context, store Store, rdb *redis.Client, eventID, voterID, trackID string) (*VoteResponse, error) {
	ev, err := store.LoadEvent(ctx, eventID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &voteError{status: http.StatusNotFound, msg: "event not found"}
		}
		return nil, err
	}

	// Verify permission to vote (implies permission to remove vote?)
	// Usually if you can vote, you can unvote.
	// We might want to check ownership or voting window?
	// For simplicity, we re-use canUserVote or just check window.
	// Let's at least check window.
	// "Voting has ended" -> cannot remove vote?
	if ev.VoteEnd != nil && time.Now().After(*ev.VoteEnd) {
		return nil, &voteError{status: http.StatusForbidden, msg: "voting has ended"}
	}

	if err := store.RemoveVote(ctx, eventID, trackID, voterID); err != nil {
		return nil, err
	}

	total, err := store.GetVoteCount(ctx, eventID, trackID)
	if err != nil {
		return nil, err
	}

	evt := map[string]any{
		"type": "vote.removed",
		"payload": map[string]any{
			"eventId":    eventID,
			"trackId":    trackID,
			"voterId":    voterID,
			"totalVotes": total,
		},
	}
	if b, err := json.Marshal(evt); err == nil && rdb != nil {
		_ = rdb.Publish(ctx, "broadcast", string(b)).Err()
	}

	return &VoteResponse{
		Status:     "ok",
		TrackID:    trackID,
		TotalVotes: total,
	}, nil
}

func checkUserExists(ctx context.Context, client *http.Client, baseURL, userID string) error {
	u, err := url.Parse(baseURL)
	if err != nil {
		return err
	}
	u.Path = "/internal/users/" + url.PathEscape(userID)
	u.Path += "/exists"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent, http.StatusOK:
		return nil
	case http.StatusNotFound:
		return &inviteError{status: http.StatusNotFound, msg: "user not found"}
	default:
		return fmt.Errorf("user-service returned %d", resp.StatusCode)
	}
}

// validateVotingWindow enforces:
// - voteStart and voteEnd (if set) cannot be in the past
// - voteStart / voteEnd cannot be more than 1 year in the future
// - if both set, window must be at least 1 hour and end after start
func validateVotingWindow(voteStart, voteEnd *time.Time, now time.Time) error {
	const maxFuture = 365 * 24 * time.Hour
	const minWindow = time.Hour

	if voteStart != nil {
		if voteStart.Before(now) {
			return &validationError{"voteStart cannot be in the past"}
		}
		if voteStart.After(now.Add(maxFuture)) {
			return &validationError{"voteStart cannot be more than 1 year in the future"}
		}
	}
	if voteEnd != nil {
		if voteEnd.Before(now) {
			return &validationError{"voteEnd cannot be in the past"}
		}
		if voteEnd.After(now.Add(maxFuture)) {
			return &validationError{"voteEnd cannot be more than 1 year in the future"}
		}
	}
	if voteStart != nil && voteEnd != nil {
		if voteEnd.Before(*voteStart) {
			return &validationError{"voteEnd must be after voteStart"}
		}
		if voteEnd.Sub(*voteStart) < minWindow {
			return &validationError{"voting window must be at least 1 hour"}
		}
	}
	return nil
}

func canUserVote(ctx context.Context, store Store, ev *Event, userID string, lat, lng *float64, now time.Time) (bool, string, error) {
	if ev.Visibility == visibilityPrivate {
		invited, err := store.IsInvited(ctx, ev.ID, userID)
		if err != nil {
			return false, "", err
		}
		if !invited && ev.OwnerID != userID {
			return false, "event is private, invite required", nil
		}
	}

	// owner can always vote regardless of license mode or geo/time
	if ev.OwnerID == userID {
		return true, "", nil
	}

	switch ev.LicenseMode {
	case "", licenseEveryone:
		invited, err := store.IsInvited(ctx, ev.ID, userID)
		if err != nil {
			return false, "", err
		}
		if !invited {
			return false, "you must join the event to vote", nil
		}
		return true, "", nil

	case licenseInvited:
		role, err := store.GetParticipantRole(ctx, ev.ID, userID)
		if err != nil {
			return false, "", err
		}
		if role == "" {
			return false, "license requires invitation to vote", nil
		}
		if role != RoleContributor {
			return false, "guests cannot vote in invited-only events", nil
		}
		return true, "", nil

	case licenseGeoTime:
		if ev.VoteStart != nil && now.Before(*ev.VoteStart) {
			return false, "voting has not started yet", nil
		}
		if ev.VoteEnd != nil && now.After(*ev.VoteEnd) {
			return false, "voting has ended", nil
		}
		if ev.GeoLat == nil || ev.GeoLng == nil || ev.GeoRadiusM == nil {
			return false, "event is not configured for geo voting", nil
		}
		if lat == nil || lng == nil {
			return false, "location (lat,lng) is required for geo voting", nil
		}
		if !withinRadius(*ev.GeoLat, *ev.GeoLng, *ev.GeoRadiusM, *lat, *lng) {
			return false, "user is outside of allowed geo area", nil
		}
		return true, "", nil

	default:
		return false, "unsupported license mode", nil
	}
}

func withinRadius(centerLat, centerLng float64, radiusM int, userLat, userLng float64) bool {
	const earthRadiusM = 6371000.0
	rad := func(d float64) float64 { return d * math.Pi / 180 }

	dLat := rad(userLat - centerLat)
	dLng := rad(userLng - centerLng)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(rad(centerLat))*math.Cos(rad(userLat))*math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	dist := earthRadiusM * c
	return dist <= float64(radiusM)
}
