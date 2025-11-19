package main

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
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
}

var ErrUserNotFound = errors.New("user not found")

func autoMigrate(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS pgcrypto`)
	if err != nil {
		log.Printf("auth-service: extension: %v", err)
	}
	_, err = pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS auth_users(
        id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
        email TEXT UNIQUE NOT NULL,
        password TEXT,
        email_verified BOOLEAN NOT NULL DEFAULT FALSE,
        google_id TEXT UNIQUE,
        ft_id TEXT UNIQUE,
        verification_token TEXT,
        verification_sent_at TIMESTAMPTZ,
        reset_token TEXT,
        reset_sent_at TIMESTAMPTZ,
        reset_expires_at TIMESTAMPTZ,
        created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
        updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
    )`)
	if err != nil {
		return err
	}
	return nil
}

func scanAuthUser(row pgx.Row) (AuthUser, error) {
	var u AuthUser
	var googleID, ftID, verifToken, resetToken *string
	var verifSentAt, resetSentAt, resetExpiresAt *time.Time

	err := row.Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.EmailVerified,
		&googleID,
		&ftID,
		&verifToken,
		&verifSentAt,
		&resetToken,
		&resetSentAt,
		&resetExpiresAt,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AuthUser{}, ErrUserNotFound
		}
		return AuthUser{}, err
	}
	u.GoogleID = googleID
	u.FTID = ftID
	u.VerifToken = verifToken
	u.VerifSentAt = verifSentAt
	u.ResetToken = resetToken
	u.ResetSentAt = resetSentAt
	u.ResetExpiresAt = resetExpiresAt
	return u, nil
}

func (s *Server) findUserByEmail(ctx context.Context, email string) (AuthUser, error) {
	row := s.db.QueryRow(ctx, `SELECT
        id, email, password, email_verified,
        google_id, ft_id,
        verification_token, verification_sent_at,
        reset_token, reset_sent_at, reset_expires_at,
        created_at, updated_at
      FROM auth_users WHERE email = $1`, email)
	return scanAuthUser(row)
}

func (s *Server) findUserByID(ctx context.Context, id string) (AuthUser, error) {
	row := s.db.QueryRow(ctx, `SELECT
        id, email, password, email_verified,
        google_id, ft_id,
        verification_token, verification_sent_at,
        reset_token, reset_sent_at, reset_expires_at,
        created_at, updated_at
      FROM auth_users WHERE id = $1`, id)
	return scanAuthUser(row)
}

func (s *Server) findUserByGoogleID(ctx context.Context, googleID string) (AuthUser, error) {
	row := s.db.QueryRow(ctx, `SELECT
        id, email, password, email_verified,
        google_id, ft_id,
        verification_token, verification_sent_at,
        reset_token, reset_sent_at, reset_expires_at,
        created_at, updated_at
      FROM auth_users WHERE google_id = $1`, googleID)
	return scanAuthUser(row)
}

func (s *Server) findUserByFTID(ctx context.Context, ftID string) (AuthUser, error) {
	row := s.db.QueryRow(ctx, `SELECT
        id, email, password, email_verified,
        google_id, ft_id,
        verification_token, verification_sent_at,
        reset_token, reset_sent_at, reset_expires_at,
        created_at, updated_at
      FROM auth_users WHERE ft_id = $1`, ftID)
	return scanAuthUser(row)
}

func (s *Server) createUserWithPassword(ctx context.Context, email, passwordHash string) (AuthUser, error) {
	row := s.db.QueryRow(ctx, `INSERT INTO auth_users (email, password)
        VALUES ($1, $2)
        ON CONFLICT (email) DO NOTHING
        RETURNING id, email, password, email_verified,
                  google_id, ft_id,
                  verification_token, verification_sent_at,
                  reset_token, reset_sent_at, reset_expires_at,
                  created_at, updated_at`,
		email, passwordHash,
	)
	return scanAuthUser(row)
}

func (s *Server) upsertUserWithGoogle(ctx context.Context, email, googleID string) (AuthUser, error) {
	row := s.db.QueryRow(ctx, `INSERT INTO auth_users (email, google_id, email_verified)
        VALUES ($1, $2, TRUE)
        ON CONFLICT (google_id) DO UPDATE
            SET email = EXCLUDED.email,
                email_verified = TRUE,
                updated_at = now()
        RETURNING id, email, password, email_verified,
                  google_id, ft_id,
                  verification_token, verification_sent_at,
                  reset_token, reset_sent_at, reset_expires_at,
                  created_at, updated_at`,
		email, googleID,
	)
	return scanAuthUser(row)
}

func (s *Server) upsertUserWithFT(ctx context.Context, email, ftID string) (AuthUser, error) {
	row := s.db.QueryRow(ctx, `INSERT INTO auth_users (email, ft_id, email_verified)
        VALUES ($1, $2, TRUE)
        ON CONFLICT (ft_id) DO UPDATE
            SET email = EXCLUDED.email,
                email_verified = TRUE,
                updated_at = now()
        RETURNING id, email, password, email_verified,
                  google_id, ft_id,
                  verification_token, verification_sent_at,
                  reset_token, reset_sent_at, reset_expires_at,
                  created_at, updated_at`,
		email, ftID,
	)
	return scanAuthUser(row)
}

func (s *Server) setVerificationToken(ctx context.Context, userID, token string) error {
	_, err := s.db.Exec(ctx, `UPDATE auth_users
        SET verification_token = $1,
            verification_sent_at = now(),
            updated_at = now()
        WHERE id = $2`, token, userID)
	return err
}

func (s *Server) verifyEmailByToken(ctx context.Context, token string) (AuthUser, error) {
	row := s.db.QueryRow(ctx, `UPDATE auth_users
        SET email_verified = TRUE,
            verification_token = NULL,
            updated_at = now()
        WHERE verification_token = $1
        RETURNING id, email, password, email_verified,
                  google_id, ft_id,
                  verification_token, verification_sent_at,
                  reset_token, reset_sent_at, reset_expires_at,
                  created_at, updated_at`, token)
	return scanAuthUser(row)
}

func (s *Server) setResetToken(ctx context.Context, userID, token string, expiresAt time.Time) error {
	_, err := s.db.Exec(ctx, `UPDATE auth_users
        SET reset_token = $1,
            reset_sent_at = now(),
            reset_expires_at = $2,
            updated_at = now()
        WHERE id = $3`, token, expiresAt, userID)
	return err
}

func (s *Server) resetPasswordByToken(ctx context.Context, token, newHash string, now time.Time) (AuthUser, error) {
	row := s.db.QueryRow(ctx, `UPDATE auth_users
        SET password = $1,
            reset_token = NULL,
            reset_expires_at = NULL,
            updated_at = now()
        WHERE reset_token = $2
          AND (reset_expires_at IS NULL OR reset_expires_at > $3)
        RETURNING id, email, password, email_verified,
                  google_id, ft_id,
                  verification_token, verification_sent_at,
                  reset_token, reset_sent_at, reset_expires_at,
                  created_at, updated_at`,
		newHash, token, now,
	)
	return scanAuthUser(row)
}
