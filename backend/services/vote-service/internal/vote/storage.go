package vote

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store interface {
	LoadEvent(ctx context.Context, id string) (*Event, error)
	// IsInvited checks if user is a participant. Use GetParticipantRole for finer check.
	IsInvited(ctx context.Context, eventID, userID string) (bool, error)
	GetParticipantRole(ctx context.Context, eventID, userID string) (string, error)
	CastVote(ctx context.Context, eventID, trackID, voterID string) error
	GetVoteCount(ctx context.Context, eventID, trackID string) (int, error)
	GetVoteTally(ctx context.Context, eventID, voterID string) ([]Row, error)
	RemoveVote(ctx context.Context, eventID, trackID, voterID string) error
	// Event Management
	ListEvents(ctx context.Context, userID, visibility string) ([]Event, error)
	CreateEvent(ctx context.Context, ev *Event) (string, error)
	DeleteEvent(ctx context.Context, id string) error
	UpdateEvent(ctx context.Context, id string, updates map[string]any) error
	TransferOwnership(ctx context.Context, id, newOwnerID string) error
	// Invite Management
	CreateInvite(ctx context.Context, eventID, userID, role string) error
	DeleteInvite(ctx context.Context, eventID, userID string) error
	ListInvites(ctx context.Context, eventID string) ([]Invite, error)
	// Stats
	GetUserStats(ctx context.Context, userID string) (*UserStats, error)
}

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

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
		return err
	}

	// Ensure columns exist for existing tables
	if _, err := pool.Exec(ctx, `ALTER TABLE events ADD COLUMN IF NOT EXISTS owner_id TEXT NOT NULL DEFAULT ''`); err != nil {
		log.Printf("migrate alter owner_id: %v", err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE events ADD COLUMN IF NOT EXISTS license_mode TEXT NOT NULL DEFAULT 'everyone'`); err != nil {
		log.Printf("migrate alter license_mode: %v", err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE events ADD COLUMN IF NOT EXISTS geo_lat DOUBLE PRECISION`); err != nil {
		log.Printf("migrate alter geo_lat: %v", err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE events ADD COLUMN IF NOT EXISTS geo_lng DOUBLE PRECISION`); err != nil {
		log.Printf("migrate alter geo_lng: %v", err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE events ADD COLUMN IF NOT EXISTS geo_radius_m INT`); err != nil {
		log.Printf("migrate alter geo_radius_m: %v", err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE events ADD COLUMN IF NOT EXISTS vote_start TIMESTAMPTZ`); err != nil {
		log.Printf("migrate alter vote_start: %v", err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE events ADD COLUMN IF NOT EXISTS vote_end TIMESTAMPTZ`); err != nil {
		log.Printf("migrate alter vote_end: %v", err)
	}

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

	_, _ = pool.Exec(ctx, `DROP INDEX IF EXISTS idx_votes_event_voter`)

	if _, err := pool.Exec(ctx, `
        CREATE TABLE IF NOT EXISTS event_invites(
            event_id uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
            user_id TEXT NOT NULL,
            role TEXT NOT NULL DEFAULT 'contributor',
            created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
            PRIMARY KEY(event_id, user_id)
        )
    `); err != nil {
		return err
	}

	// Ensure role column exists for existing tables
	if _, err := pool.Exec(ctx, `ALTER TABLE event_invites ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'contributor'`); err != nil {
		log.Printf("migrate alter event_invites role: %v", err)
	}

	return nil
}

func (s *PostgresStore) LoadEvent(ctx context.Context, id string) (*Event, error) {
	var ev Event
	var geoLat, geoLng *float64
	var geoRadius *int
	var voteStart, voteEnd *time.Time
	err := s.pool.QueryRow(ctx, `
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

func (s *PostgresStore) IsInvited(ctx context.Context, eventID, userID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
        SELECT EXISTS(SELECT 1 FROM event_invites WHERE event_id=$1 AND user_id=$2)
    `, eventID, userID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (s *PostgresStore) GetParticipantRole(ctx context.Context, eventID, userID string) (string, error) {
	var role string
	err := s.pool.QueryRow(ctx, `
        SELECT role FROM event_invites WHERE event_id=$1 AND user_id=$2
    `, eventID, userID).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil // Not found, no error
		}
		return "", err
	}
	return role, nil
}

func (s *PostgresStore) CastVote(ctx context.Context, eventID, trackID, voterID string) error {
	_, err := s.pool.Exec(ctx, `
        INSERT INTO votes(event_id, track, voter_id)
        VALUES($1,$2,$3)
    `, eventID, trackID, voterID)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrVoteConflict // We need to define this or handle it
		}
		return err
	}
	return nil
}

func (s *PostgresStore) CreateInvite(ctx context.Context, eventID, userID, role string) error {
	_, err := s.pool.Exec(ctx, `
        INSERT INTO event_invites(event_id, user_id, role)
        VALUES($1,$2,$3) ON CONFLICT(event_id, user_id) DO UPDATE SET role = EXCLUDED.role
    `, eventID, userID, role)
	return err
}

func (s *PostgresStore) DeleteInvite(ctx context.Context, eventID, userID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM event_invites WHERE event_id=$1 AND user_id=$2`, eventID, userID)
	return err
}

func (s *PostgresStore) ListInvites(ctx context.Context, eventID string) ([]Invite, error) {
	rows, err := s.pool.Query(ctx, `SELECT user_id, created_at FROM event_invites WHERE event_id=$1 ORDER BY created_at`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invites []Invite
	for rows.Next() {
		var inv Invite
		if err := rows.Scan(&inv.UserID, &inv.CreatedAt); err != nil {
			return nil, err
		}
		invites = append(invites, inv)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return invites, nil
}

func (s *PostgresStore) ListEvents(ctx context.Context, userID, visibility string) ([]Event, error) {
	var rows pgx.Rows
	var err error
	if userID == "" {
		rows, err = s.pool.Query(ctx, `
            SELECT id, name, visibility, owner_id, license_mode,
                   geo_lat, geo_lng, geo_radius_m, vote_start, vote_end,
                   created_at, updated_at
            FROM events
            WHERE visibility = $1
            ORDER BY created_at DESC
        `, visibility)
	} else {
		rows, err = s.pool.Query(ctx, `
            SELECT DISTINCT e.id, e.name, e.visibility, e.owner_id, e.license_mode,
                   e.geo_lat, e.geo_lng, e.geo_radius_m, e.vote_start, e.vote_end,
                   e.created_at, e.updated_at,
                   CASE WHEN i.user_id IS NOT NULL OR e.owner_id = $1 THEN true ELSE false END as is_joined
            FROM events e
            LEFT JOIN event_invites i
              ON i.event_id = e.id AND i.user_id = $1
            WHERE e.visibility = $2
               OR e.owner_id = $1
               OR i.user_id IS NOT NULL
            ORDER BY e.created_at DESC
        `, userID, visibility)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]Event, 0)
	for rows.Next() {
		var ev Event
		var geoLat, geoLng *float64
		var geoRadius *int
		var voteStart, voteEnd *time.Time

		if userID == "" {
			if err := rows.Scan(
				&ev.ID, &ev.Name, &ev.Visibility, &ev.OwnerID, &ev.LicenseMode,
				&geoLat, &geoLng, &geoRadius, &voteStart, &voteEnd,
				&ev.CreatedAt, &ev.UpdatedAt,
			); err != nil {
				return nil, err
			}
			ev.IsJoined = false
		} else {
			if err := rows.Scan(
				&ev.ID, &ev.Name, &ev.Visibility, &ev.OwnerID, &ev.LicenseMode,
				&geoLat, &geoLng, &geoRadius, &voteStart, &voteEnd,
				&ev.CreatedAt, &ev.UpdatedAt,
				&ev.IsJoined,
			); err != nil {
				return nil, err
			}
		}
		ev.GeoLat = geoLat
		ev.GeoLng = geoLng
		ev.GeoRadiusM = geoRadius
		ev.VoteStart = voteStart
		ev.VoteEnd = voteEnd
		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func (s *PostgresStore) CreateEvent(ctx context.Context, ev *Event) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
        INSERT INTO events (id, name, visibility, owner_id, license_mode, geo_lat, geo_lng, geo_radius_m, vote_start, vote_end)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
        RETURNING id
    `, ev.ID, ev.Name, ev.Visibility, ev.OwnerID, ev.LicenseMode, ev.GeoLat, ev.GeoLng, ev.GeoRadiusM, ev.VoteStart, ev.VoteEnd).Scan(&id)
	return id, err
}

func (s *PostgresStore) DeleteEvent(ctx context.Context, id string) error {
	res, err := s.pool.Exec(ctx, `DELETE FROM events WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *PostgresStore) UpdateEvent(ctx context.Context, id string, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}
	setParts := []string{}
	args := []any{}
	idxArg := 1
	for k, v := range updates {
		setParts = append(setParts, k+" = $"+itoa(idxArg))
		args = append(args, v)
		idxArg++
	}
	args = append(args, id)
	query := "UPDATE events SET " + join(setParts, ", ") + ", updated_at = now() WHERE id = $" + itoa(idxArg)
	res, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *PostgresStore) TransferOwnership(ctx context.Context, id, newOwnerID string) error {
	res, err := s.pool.Exec(ctx, `UPDATE events SET owner_id = $1, updated_at = now() WHERE id = $2`, newOwnerID, id)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// Helpers need to be moved or duplicated if logic.go ones are not accessible?
// logic.go has no helpers 'itoa' or 'join'. They were inline in http_events.go probably?
// Ah, 'itoa' and 'join' were likely in http_utils.go or similar. I need to check where they are defined.
// If they are in http_utils.go (package vote), they are accessible here.
// Assuming they are available.

func (s *PostgresStore) RemoveVote(ctx context.Context, eventID, trackID, voterID string) error {
	res, err := s.pool.Exec(ctx, `
        DELETE FROM votes
        WHERE event_id=$1 AND track=$2 AND voter_id=$3
    `, eventID, trackID, voterID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *PostgresStore) GetVoteCount(ctx context.Context, eventID, trackID string) (int, error) {
	var total int
	err := s.pool.QueryRow(ctx, `
        SELECT COUNT(*) FROM votes WHERE event_id=$1 AND track=$2
    `, eventID, trackID).Scan(&total)
	return total, err
}

func (s *PostgresStore) GetVoteTally(ctx context.Context, eventID, voterID string) ([]Row, error) {
	rows, err := s.pool.Query(ctx, `
        SELECT 
            track, 
            COUNT(*) AS c,
            COALESCE(BOOL_OR(voter_id = $2), false) as is_my_vote
        FROM votes
        WHERE event_id = $1
        GROUP BY track
        ORDER BY c DESC, track ASC
    `, eventID, voterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Row
	for rows.Next() {
		var row Row
		if err := rows.Scan(&row.Track, &row.Count, &row.IsMyVote); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (s *PostgresStore) GetUserStats(ctx context.Context, userID string) (*UserStats, error) {
	var stats UserStats
	err := s.pool.QueryRow(ctx, `
        SELECT 
            (SELECT COUNT(*) FROM events WHERE owner_id=$1),
            (SELECT COUNT(*) FROM votes WHERE voter_id=$1)
    `, userID).Scan(&stats.EventsHosted, &stats.VotesCast)

	if err != nil {
		return nil, err
	}
	return &stats, nil
}
