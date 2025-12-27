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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func registerVote(ctx context.Context, pool *pgxpool.Pool, rdb *redis.Client, eventID, voterID, trackID string, lat, lng *float64) (*VoteResponse, error) {
	ev, err := loadEvent(ctx, pool, eventID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &voteError{status: http.StatusNotFound, msg: "event not found"}
		}
		return nil, err
	}

	if ok, reason, err := canUserVote(ctx, pool, ev, voterID, lat, lng, time.Now()); err != nil {
		return nil, err
	} else if !ok {
		return nil, &voteError{status: http.StatusForbidden, msg: reason}
	}

	_, err = pool.Exec(ctx, `
        INSERT INTO votes(event_id, track, voter_id)
        VALUES($1,$2,$3)
        ON CONFLICT (event_id, voter_id) 
        DO UPDATE SET track = EXCLUDED.track, created_at = now()
    `, eventID, trackID, voterID)
	if err != nil {
		return nil, err
	}

	var total int
	if err := pool.QueryRow(ctx, `
        SELECT COUNT(*) FROM votes WHERE event_id=$1 AND track=$2
    `, eventID, trackID).Scan(&total); err != nil {
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
	if b, err := json.Marshal(evt); err == nil {
		_ = rdb.Publish(ctx, "broadcast", string(b)).Err()
	}

	return &VoteResponse{
		Status:     "ok",
		TrackID:    trackID,
		TotalVotes: total,
	}, nil
}

func checkUserExists(ctx context.Context, baseURL, userID string) error {
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
	resp, err := http.DefaultClient.Do(req)
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

func canUserVote(ctx context.Context, pool *pgxpool.Pool, ev *Event, userID string, lat, lng *float64, now time.Time) (bool, string, error) {
	if ev.Visibility == visibilityPrivate {
		invited, err := isInvited(ctx, pool, ev.ID, userID)
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
		return true, "", nil

	case licenseInvited:
		invited, err := isInvited(ctx, pool, ev.ID, userID)
		if err != nil {
			return false, "", err
		}
		if !invited {
			return false, "license requires invitation to vote", nil
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
