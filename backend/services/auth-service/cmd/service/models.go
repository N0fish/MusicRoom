package main

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthUser struct {
	ID             string
	Email          string
	PasswordHash   string
	EmailVerified  bool
	GoogleID       *string
	FTID           *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	VerifToken     *string
	VerifSentAt    *time.Time
	ResetToken     *string
	ResetSentAt    *time.Time
	ResetExpiresAt *time.Time
	TokenVersion   int
}

var ErrUserNotFound = errors.New("user not found")

func autoMigrate(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
      CREATE TABLE IF NOT EXISTS auth_users(
          id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
          email TEXT UNIQUE NOT NULL,
          password TEXT NOT NULL DEFAULT '',
          email_verified BOOLEAN NOT NULL DEFAULT FALSE,
          google_id TEXT UNIQUE,
          ft_id TEXT UNIQUE,
          verification_token TEXT,
          verification_sent_at TIMESTAMPTZ,
          reset_token TEXT,
          reset_sent_at TIMESTAMPTZ,
          reset_expires_at TIMESTAMPTZ,
          token_version INT NOT NULL DEFAULT 1,
          created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
          updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
      );
      
      -- Ensure column exists if table already existed without it
      DO $$
      BEGIN
          IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
                         WHERE table_name='auth_users' AND column_name='token_version') THEN
              ALTER TABLE auth_users ADD COLUMN token_version INT NOT NULL DEFAULT 1;
          END IF;
      END
      $$;
  `)
	if err != nil {
		log.Printf("migrate auth-service: %v", err)
		return err
	}
	return nil
}
