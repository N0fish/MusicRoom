package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserProfile struct {
	ID          string
	UserID      string
	DisplayName string
	AvatarURL   string
	PublicBio   string
	FriendsBio  string
	PrivateBio  string
	Visibility  string
	Preferences Preferences
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Preferences struct {
	Genres  []string `json:"genres,omitempty"`
	Artists []string `json:"artists,omitempty"`
	Moods   []string `json:"moods,omitempty"`
}

var ErrProfileNotFound = errors.New("profile not found")

func autoMigrate(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS pgcrypto`)
	if err != nil {
		log.Printf("user-service: extension: %v", err)
	}

	_, err = pool.Exec(ctx, `
      CREATE TABLE IF NOT EXISTS user_profiles (
          id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
          user_id uuid UNIQUE NOT NULL,
          display_name TEXT NOT NULL DEFAULT '',
          avatar_url TEXT NOT NULL DEFAULT '',
          public_bio TEXT NOT NULL DEFAULT '',
          friends_bio TEXT NOT NULL DEFAULT '',
          private_bio TEXT NOT NULL DEFAULT '',
          visibility TEXT NOT NULL DEFAULT 'public',
          preferences JSONB NOT NULL DEFAULT '{}'::jsonb,
          created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
          updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
      )
  `)
	if err != nil {
		log.Printf("migrate user_profiles: %v", err)
	}
	return nil
}

func (s *Server) findProfileByUserID(ctx context.Context, userID string) (UserProfile, error) {
	row := s.db.QueryRow(ctx, `
      SELECT id, user_id, display_name, avatar_url,
              public_bio, friends_bio, private_bio,
              visibility, preferences,
              created_at, updated_at
      FROM user_profiles
      WHERE user_id = $1
  `, userID)
	return scanUserProfile(row)
}

func (s *Server) getOrCreateProfile(ctx context.Context, userID string) (UserProfile, error) {
	prof, err := s.findProfileByUserID(ctx, userID)
	if err == nil {
		return prof, nil
	}
	if !errors.Is(err, ErrProfileNotFound) {
		return UserProfile{}, err
	}

	// Профиль отсутствует — создаём по умолчанию.
	row := s.db.QueryRow(ctx, `
      INSERT INTO user_profiles (user_id)
      VALUES ($1)
      ON CONFLICT (user_id) DO NOTHING
      RETURNING id, user_id, display_name, avatar_url,
                public_bio, friends_bio, private_bio,
                visibility, preferences,
                created_at, updated_at
  `, userID)

	prof, err = scanUserProfile(row)
	if err == nil {
		return prof, nil
	}
	if errors.Is(err, ErrProfileNotFound) {
		// Если : ON CONFLICT DO NOTHING и RETURNING не сработал.
		return s.findProfileByUserID(ctx, userID)
	}
	return UserProfile{}, err
}

func (s *Server) saveProfile(ctx context.Context, prof UserProfile) error {
	prefJSON, err := json.Marshal(prof.Preferences)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(ctx, `
        UPDATE user_profiles
        SET display_name = $1,
            avatar_url = $2,
            public_bio = $3,
            friends_bio = $4,
            private_bio = $5,
            visibility = $6,
            preferences = $7,
            updated_at = $8
        WHERE user_id = $9
    `,
		prof.DisplayName,
		prof.AvatarURL,
		prof.PublicBio,
		prof.FriendsBio,
		prof.PrivateBio,
		prof.Visibility,
		prefJSON,
		prof.UpdatedAt,
		prof.UserID,
	)
	return err
}

func scanUserProfile(row pgx.Row) (UserProfile, error) {
	var (
		p         UserProfile
		prefBytes []byte
	)

	err := row.Scan(
		&p.ID,
		&p.UserID,
		&p.DisplayName,
		&p.AvatarURL,
		&p.PublicBio,
		&p.FriendsBio,
		&p.PrivateBio,
		&p.Visibility,
		&prefBytes,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserProfile{}, ErrProfileNotFound
		}
		return UserProfile{}, err
	}

	if len(prefBytes) == 0 || string(prefBytes) == "null" {
		p.Preferences = Preferences{}
	} else {
		if err := json.Unmarshal(prefBytes, &p.Preferences); err != nil {
			// В случае битых данных не падаем, а логируем и используем пустое значение.
			log.Printf("user-service: invalid preferences JSON for user %s: %v", p.UserID, err)
			p.Preferences = Preferences{}
		}
	}

	return p, nil
}

// Helpers for nullable scan if needed
// func scanNullString(ns sql.NullString) string {
// 	if ns.Valid {
// 		return ns.String
// 	}
// 	return ""
// }
