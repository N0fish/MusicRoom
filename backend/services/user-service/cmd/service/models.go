package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserProfile struct {
	ID              string
	UserID          string
	DisplayName     string
	Username        string
	AvatarURL       string
	HasCustomAvatar bool
	PublicBio       string
	FriendsBio      string
	PrivateBio      string
	Visibility      string
	Preferences     Preferences
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Preferences struct {
	Genres  []string `json:"genres,omitempty"`
	Artists []string `json:"artists,omitempty"`
	Moods   []string `json:"moods,omitempty"`
}

var ErrProfileNotFound = errors.New("profile not found")

func autoMigrate(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
      CREATE TABLE IF NOT EXISTS user_profiles (
          id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
          user_id uuid UNIQUE NOT NULL,
          display_name TEXT NOT NULL DEFAULT '',
          username TEXT NOT NULL DEFAULT '',
          avatar_url TEXT NOT NULL DEFAULT '',
          has_custom_avatar BOOLEAN NOT NULL DEFAULT FALSE,
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
		return err
	}

	_, _ = pool.Exec(ctx, `
      CREATE UNIQUE INDEX IF NOT EXISTS idx_user_profiles_username
      ON user_profiles (LOWER(username))
      WHERE username <> ''
  `)
	_, _ = pool.Exec(ctx, `
      CREATE INDEX IF NOT EXISTS idx_user_profiles_display_name
      ON user_profiles (LOWER(display_name))
  `)

	_, err = pool.Exec(ctx, `
      CREATE TABLE IF NOT EXISTS friend_requests (
          id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
          from_user_id uuid NOT NULL,
          to_user_id uuid NOT NULL,
          status TEXT NOT NULL DEFAULT 'pending',
          created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
          updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
      )
  `)
	if err != nil {
		log.Printf("migrate friend_requests: %v", err)
	}

	_, _ = pool.Exec(ctx, `
      CREATE UNIQUE INDEX IF NOT EXISTS idx_friend_requests_unique
      ON friend_requests (from_user_id, to_user_id)
      WHERE status = 'pending'
  `)

	_, err = pool.Exec(ctx, `
      CREATE TABLE IF NOT EXISTS user_friends (
          user1_id uuid NOT NULL,
          user2_id uuid NOT NULL,
          created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
          PRIMARY KEY (user1_id, user2_id)
      )
  `)
	if err != nil {
		log.Printf("migrate user_friends: %v", err)
	}
	return nil
}

func (s *Server) findProfileByUserID(ctx context.Context, userID string) (UserProfile, error) {
	row := s.db.QueryRow(ctx, `
      SELECT id, user_id, display_name, username,
             avatar_url, has_custom_avatar,
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
		return s.ensureUsername(ctx, prof)
	}
	if !errors.Is(err, ErrProfileNotFound) {
		return UserProfile{}, err
	}

	row := s.db.QueryRow(ctx, `
      INSERT INTO user_profiles (user_id)
      VALUES ($1)
      ON CONFLICT (user_id) DO NOTHING
      RETURNING id, user_id, display_name, username,
                avatar_url, has_custom_avatar,
                public_bio, friends_bio, private_bio,
                visibility, preferences,
                created_at, updated_at
  `, userID)

	prof, err = scanUserProfile(row)
	if err == nil {
		return s.ensureUsername(ctx, prof)
	}
	if errors.Is(err, ErrProfileNotFound) {
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
            username = $2,
            avatar_url = $3,
            has_custom_avatar = $4,
            public_bio = $5,
            friends_bio = $6,
            private_bio = $7,
            visibility = $8,
            preferences = $9,
            updated_at = $10
        WHERE user_id = $11
  `,
		prof.DisplayName,
		prof.Username,
		prof.AvatarURL,
		prof.HasCustomAvatar,
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
		&p.Username,
		&p.AvatarURL,
		&p.HasCustomAvatar,
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
			log.Printf("user-service: invalid preferences JSON for user %s: %v", p.UserID, err)
			p.Preferences = Preferences{
				Genres:  []string{},
				Artists: []string{},
				Moods:   []string{},
			}
		}
	}

	return p, nil
}

func (s *Server) userExists(ctx context.Context, userID string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(ctx, `
        SELECT EXISTS(
            SELECT 1 FROM user_profiles WHERE user_id = $1
        )
    `, userID).Scan(&exists)
	return exists, err
}

func (s *Server) areFriends(ctx context.Context, userID1, userID2 string) (bool, error) {
	if userID1 == "" || userID2 == "" || userID1 == userID2 {
		return false, nil
	}
	var a, b string
	if userID1 < userID2 {
		a, b = userID1, userID2
	} else {
		a, b = userID2, userID1
	}

	var exists bool
	err := s.db.QueryRow(ctx, `
      SELECT EXISTS(
        SELECT 1 FROM user_friends
        WHERE user1_id = $1 AND user2_id = $2
      )
  `, a, b).Scan(&exists)
	return exists, err
}

func (s *Server) addFriends(ctx context.Context, userID1, userID2 string) error {
	if userID1 == "" || userID2 == "" || userID1 == userID2 {
		return errors.New("invalid users")
	}
	var a, b string
	if userID1 < userID2 {
		a, b = userID1, userID2
	} else {
		a, b = userID2, userID1
	}
	_, err := s.db.Exec(ctx, `
      INSERT INTO user_friends (user1_id, user2_id)
      VALUES ($1, $2)
      ON CONFLICT DO NOTHING
  `, a, b)
	return err
}

func (s *Server) removeFriends(ctx context.Context, userID1, userID2 string) error {
	if userID1 == "" || userID2 == "" || userID1 == userID2 {
		return nil
	}
	var a, b string
	if userID1 < userID2 {
		a, b = userID1, userID2
	} else {
		a, b = userID2, userID1
	}
	_, err := s.db.Exec(ctx, `
      DELETE FROM user_friends
      WHERE user1_id = $1 AND user2_id = $2
  `, a, b)
	return err
}

var adjectives = []string{
	"sleepy", "awkward", "giant", "angry", "happy", "noisy",
	"quiet", "brave", "clever", "shiny", "cosmic", "wild",
}

var nouns = []string{
	"strawberry", "pineapple", "goose", "coffee", "otter", "falcon",
	"piano", "galaxy", "nebula", "comet", "dragon", "lemur",
}

func (s *Server) ensureUsername(ctx context.Context, p UserProfile) (UserProfile, error) {
	if strings.TrimSpace(p.Username) != "" {
		return p, nil
	}

	const maxAttempts = 20
	for i := 0; i < maxAttempts; i++ {
		u := generateRandomUsername()

		var exists bool
		err := s.db.QueryRow(ctx, `
        SELECT EXISTS(SELECT 1 FROM user_profiles WHERE LOWER(username) = LOWER($1))
      `, u).Scan(&exists)
		if err != nil {
			return p, err
		}
		if !exists {
			_, err := s.db.Exec(ctx, `
          UPDATE user_profiles
          SET username = $1, updated_at = now()
          WHERE user_id = $2
        `, u, p.UserID)
			if err != nil {
				return p, err
			}
			p.Username = u
			p.UpdatedAt = time.Now().UTC()
			return p, nil
		}
	}

	return p, errors.New("unable to generate unique username")
}

func generateRandomUsername() string {
	rand.Seed(time.Now().UnixNano())
	a := adjectives[rand.Intn(len(adjectives))]
	n := nouns[rand.Intn(len(nouns))]
	return a + "-" + n
}
