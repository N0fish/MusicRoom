package playlist

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

func AutoMigrate(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
      CREATE TABLE IF NOT EXISTS playlists (
          id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
          owner_id    TEXT NOT NULL,
          name        TEXT NOT NULL,
          description TEXT NOT NULL DEFAULT '',
          is_public   BOOLEAN NOT NULL DEFAULT TRUE,
          edit_mode   TEXT NOT NULL DEFAULT 'everyone',
          created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
      )
    `)
	if err != nil {
		log.Printf("migrate playlists-service: %v", err)
		return err
	}

	if _, err := pool.Exec(ctx, `
      CREATE TABLE IF NOT EXISTS tracks (
          id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
          playlist_id uuid NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
          title       TEXT NOT NULL,
          artist      TEXT NOT NULL,
          position    INT NOT NULL,
          provider    TEXT NOT NULL DEFAULT '',
          provider_track_id TEXT NOT NULL DEFAULT '',
          thumbnail_url TEXT NOT NULL DEFAULT '',
          created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
      )
    `); err != nil {
		return err
	}

	// --- Migrations for Playback Logic ---

	// 1. Add columns to tracks
	if _, err := pool.Exec(ctx, `
		ALTER TABLE tracks ADD COLUMN IF NOT EXISTS duration_ms INT NOT NULL DEFAULT 0;
		ALTER TABLE tracks ADD COLUMN IF NOT EXISTS vote_count INT NOT NULL DEFAULT 0;
		ALTER TABLE tracks ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'queued';
		ALTER TABLE tracks ADD COLUMN IF NOT EXISTS provider TEXT NOT NULL DEFAULT '';
		ALTER TABLE tracks ADD COLUMN IF NOT EXISTS provider_track_id TEXT NOT NULL DEFAULT '';
		ALTER TABLE tracks ADD COLUMN IF NOT EXISTS thumbnail_url TEXT NOT NULL DEFAULT '';
	`); err != nil {
		return err
	}

	// 2. Add columns to playlists (circular reference handled by ALTER)
	if _, err := pool.Exec(ctx, `
		ALTER TABLE playlists ADD COLUMN IF NOT EXISTS current_track_id uuid REFERENCES tracks(id) ON DELETE SET NULL;
		ALTER TABLE playlists ADD COLUMN IF NOT EXISTS playing_started_at TIMESTAMPTZ;
	`); err != nil {
		return err
	}

	if _, err := pool.Exec(ctx, `
      CREATE UNIQUE INDEX IF NOT EXISTS idx_tracks_playlist_position
      ON tracks(playlist_id, position)
    `); err != nil {
		return err
	}

	if _, err := pool.Exec(ctx, `
      CREATE TABLE IF NOT EXISTS playlist_members (
          playlist_id uuid NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
          user_id     TEXT NOT NULL,
          created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
          PRIMARY KEY (playlist_id, user_id)
      )
    `); err != nil {
		return err
	}

	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS track_votes (
			track_id uuid NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
			user_id  TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (track_id, user_id)
		)
	`); err != nil {
		return err
	}

	return nil
}
