package vote

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func AutoMigrate(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
      CREATE TABLE IF NOT EXISTS events(
          id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
          name TEXT NOT NULL,
          visibility TEXT NOT NULL DEFAULT 'public',
          owner_id TEXT NOT NULL DEFAULT '',
          license_mode TEXT NOT NULL DEFAULT 'everyone',
          geo_lat DOUBLE PRECISION,
          geo_lng DOUBLE PRECISION,
          geo_radius_m INT,
          vote_start TIMESTAMPTZ,
          vote_end TIMESTAMPTZ,
          created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
          updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
      )
  `)
	if err != nil {
		log.Printf("migrate vote-service: %v", err)
	}
	_, _ = pool.Exec(ctx, `ALTER TABLE events ADD COLUMN IF NOT EXISTS owner_id TEXT NOT NULL DEFAULT ''`)
	_, _ = pool.Exec(ctx, `ALTER TABLE events ADD COLUMN IF NOT EXISTS license_mode TEXT NOT NULL DEFAULT 'everyone'`)
	_, _ = pool.Exec(ctx, `ALTER TABLE events ADD COLUMN IF NOT EXISTS geo_lat DOUBLE PRECISION`)
	_, _ = pool.Exec(ctx, `ALTER TABLE events ADD COLUMN IF NOT EXISTS geo_lng DOUBLE PRECISION`)
	_, _ = pool.Exec(ctx, `ALTER TABLE events ADD COLUMN IF NOT EXISTS geo_radius_m INT`)
	_, _ = pool.Exec(ctx, `ALTER TABLE events ADD COLUMN IF NOT EXISTS vote_start TIMESTAMPTZ`)
	_, _ = pool.Exec(ctx, `ALTER TABLE events ADD COLUMN IF NOT EXISTS vote_end TIMESTAMPTZ`)
	_, _ = pool.Exec(ctx, `ALTER TABLE events ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now()`)
	_, _ = pool.Exec(ctx, `ALTER TABLE events ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`)

	if _, err := pool.Exec(ctx, `
        CREATE TABLE IF NOT EXISTS votes(
            id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
            event_id uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
            track TEXT NOT NULL,
            voter_id TEXT NOT NULL,
            created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
            UNIQUE(event_id, voter_id, track)
        )
    `); err != nil {
		return err
	}
	// one vote per user per event (cannot change vote)
	_, _ = pool.Exec(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS idx_votes_event_voter ON votes(event_id, voter_id)`)

	if _, err := pool.Exec(ctx, `
        CREATE TABLE IF NOT EXISTS event_invites(
            event_id uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
            user_id TEXT NOT NULL,
            created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
            PRIMARY KEY(event_id, user_id)
        )
    `); err != nil {
		return err
	}

	return nil
}

func loadEvent(ctx context.Context, pool *pgxpool.Pool, id string) (*Event, error) {
	var ev Event
	var geoLat, geoLng *float64
	var geoRadius *int
	var voteStart, voteEnd *time.Time
	err := pool.QueryRow(ctx, `
        SELECT id, name, visibility, owner_id, license_mode,
               geo_lat, geo_lng, geo_radius_m, vote_start, vote_end,
               created_at, updated_at
        FROM events WHERE id=$1
    `, id).Scan(
		&ev.ID, &ev.Name, &ev.Visibility, &ev.OwnerID, &ev.LicenseMode,
		&geoLat, &geoLng, &geoRadius, &voteStart, &voteEnd,
		&ev.CreatedAt, &ev.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	ev.GeoLat = geoLat
	ev.GeoLng = geoLng
	ev.GeoRadiusM = geoRadius
	ev.VoteStart = voteStart
	ev.VoteEnd = voteEnd
	return &ev, nil
}

func isInvited(ctx context.Context, pool *pgxpool.Pool, eventID, userID string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `
        SELECT EXISTS(SELECT 1 FROM event_invites WHERE event_id=$1 AND user_id=$2)
    `, eventID, userID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}
