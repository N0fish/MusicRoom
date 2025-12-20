package main

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	FindUserByEmail(ctx context.Context, email string) (AuthUser, error)
	FindUserByID(ctx context.Context, id string) (AuthUser, error)
	FindUserByGoogleID(ctx context.Context, googleID string) (AuthUser, error)
	FindUserByFTID(ctx context.Context, ftID string) (AuthUser, error)
	CreateUserWithPassword(ctx context.Context, email, passwordHash string) (AuthUser, error)
	UpsertUserWithGoogle(ctx context.Context, email, googleID string) (AuthUser, error)
	UpsertUserWithFT(ctx context.Context, email, ftID string) (AuthUser, error)
	SetVerificationToken(ctx context.Context, userID, token string) error
	VerifyEmailByToken(ctx context.Context, token string) (AuthUser, error)
	SetResetToken(ctx context.Context, userID, token string, expiresAt time.Time) error
	ResetPasswordByToken(ctx context.Context, token, newHash string, now time.Time) (AuthUser, error)
	DeleteUser(ctx context.Context, userID string) error
	UpdateGoogleID(ctx context.Context, userID string, googleID *string) (AuthUser, error)
	UpdateFTID(ctx context.Context, userID string, ftID *string) (AuthUser, error)
}

// DBOps defines the subset of pgxpool.Pool methods we use.
// This allows us to inject a mock for testing.
type DBOps interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type PostgresRepository struct {
	db DBOps
}

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) FindUserByEmail(ctx context.Context, email string) (AuthUser, error) {
	row := r.db.QueryRow(ctx, `SELECT
        id, email, password, email_verified,
        google_id, ft_id,
        verification_token, verification_sent_at,
        reset_token, reset_sent_at, reset_expires_at,
        token_version,
        created_at, updated_at
      FROM auth_users WHERE email = $1`, email)
	return scanAuthUser(row)
}

func (r *PostgresRepository) FindUserByID(ctx context.Context, id string) (AuthUser, error) {
	row := r.db.QueryRow(ctx, `SELECT
        id, email, password, email_verified,
        google_id, ft_id,
        verification_token, verification_sent_at,
        reset_token, reset_sent_at, reset_expires_at,
        token_version,
        created_at, updated_at
      FROM auth_users WHERE id = $1`, id)
	return scanAuthUser(row)
}

func (r *PostgresRepository) FindUserByGoogleID(ctx context.Context, googleID string) (AuthUser, error) {
	row := r.db.QueryRow(ctx, `SELECT
        id, email, password, email_verified,
        google_id, ft_id,
        verification_token, verification_sent_at,
        reset_token, reset_sent_at, reset_expires_at,
        token_version,
        created_at, updated_at
      FROM auth_users WHERE google_id = $1`, googleID)
	return scanAuthUser(row)
}

func (r *PostgresRepository) FindUserByFTID(ctx context.Context, ftID string) (AuthUser, error) {
	row := r.db.QueryRow(ctx, `SELECT
        id, email, password, email_verified,
        google_id, ft_id,
        verification_token, verification_sent_at,
        reset_token, reset_sent_at, reset_expires_at,
        token_version,
        created_at, updated_at
      FROM auth_users WHERE ft_id = $1`, ftID)
	return scanAuthUser(row)
}

func (r *PostgresRepository) CreateUserWithPassword(ctx context.Context, email, passwordHash string) (AuthUser, error) {
	row := r.db.QueryRow(ctx, `INSERT INTO auth_users (email, password)
        VALUES ($1, $2)
        ON CONFLICT (email) DO NOTHING
        RETURNING id, email, password, email_verified,
                  google_id, ft_id,
                  verification_token, verification_sent_at,
                  reset_token, reset_sent_at, reset_expires_at,
                  token_version,
                  created_at, updated_at`,
		email, passwordHash,
	)
	return scanAuthUser(row)
}

func (r *PostgresRepository) UpsertUserWithGoogle(ctx context.Context, email, googleID string) (AuthUser, error) {
	user, err := r.FindUserByGoogleID(ctx, googleID)
	if err != nil {
		if err != ErrUserNotFound {
			return AuthUser{}, err
		}
	} else {
		if user.Email != email {
			row := r.db.QueryRow(ctx, `UPDATE auth_users SET email = $1, updated_at = now() WHERE id = $2
						RETURNING id, email, password, email_verified,
											google_id, ft_id,
											verification_token, verification_sent_at,
											reset_token, reset_sent_at, reset_expires_at,
                                            token_version,
											created_at, updated_at`,
				email, user.ID,
			)
			return scanAuthUser(row)
		}
		return user, nil
	}

	user, err = r.FindUserByEmail(ctx, email)
	if err != nil {
		if err != ErrUserNotFound {
			return AuthUser{}, err
		}
	} else {
		row := r.db.QueryRow(ctx, `UPDATE auth_users SET google_id = $1, email_verified = TRUE, updated_at = now() WHERE id = $2
					RETURNING id, email, password, email_verified,
										google_id, ft_id,
										verification_token, verification_sent_at,
										reset_token, reset_sent_at, reset_expires_at,
                                        token_version,
										created_at, updated_at`,
			googleID, user.ID,
		)
		return scanAuthUser(row)
	}

	row := r.db.QueryRow(ctx, `INSERT INTO auth_users (email, google_id, email_verified)
        VALUES ($1, $2, TRUE)
        RETURNING id, email, password, email_verified,
                  google_id, ft_id,
                  verification_token, verification_sent_at,
                  reset_token, reset_sent_at, reset_expires_at,
                  token_version,
                  created_at, updated_at`,
		email, googleID,
	)
	return scanAuthUser(row)
}

func (r *PostgresRepository) UpsertUserWithFT(ctx context.Context, email, ftID string) (AuthUser, error) {
	row := r.db.QueryRow(ctx, `INSERT INTO auth_users (email, ft_id, email_verified)
        VALUES ($1, $2, TRUE)
        ON CONFLICT (ft_id) DO UPDATE
            SET email = EXCLUDED.email,
                email_verified = TRUE,
                updated_at = now()
        RETURNING id, email, password, email_verified,
                  google_id, ft_id,
                  verification_token, verification_sent_at,
                  reset_token, reset_sent_at, reset_expires_at,
                  token_version,
                  created_at, updated_at`,
		email, ftID,
	)
	return scanAuthUser(row)
}

func (r *PostgresRepository) SetVerificationToken(ctx context.Context, userID, token string) error {
	_, err := r.db.Exec(ctx, `UPDATE auth_users
        SET verification_token = $1,
        verification_sent_at = now(),
        updated_at = now()
        WHERE id = $2`, token, userID)
	return err
}

func (r *PostgresRepository) VerifyEmailByToken(ctx context.Context, token string) (AuthUser, error) {
	row := r.db.QueryRow(ctx, `UPDATE auth_users
        SET email_verified = TRUE,
            verification_token = NULL,
            updated_at = now()
        WHERE verification_token = $1
        RETURNING id, email, password, email_verified,
                  google_id, ft_id,
                  verification_token, verification_sent_at,
                  reset_token, reset_sent_at, reset_expires_at,
                  token_version,
                  created_at, updated_at`, token)
	return scanAuthUser(row)
}

func (r *PostgresRepository) SetResetToken(ctx context.Context, userID, token string, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx, `UPDATE auth_users
        SET reset_token = $1,
            reset_sent_at = now(),
            reset_expires_at = $2,
            updated_at = now()
        WHERE id = $3`, token, expiresAt, userID)
	return err
}

func (r *PostgresRepository) ResetPasswordByToken(ctx context.Context, token, newHash string, now time.Time) (AuthUser, error) {
	row := r.db.QueryRow(ctx, `UPDATE auth_users
        SET password = $1,
            reset_token = NULL,
            reset_expires_at = NULL,
            token_version = token_version + 1,
            updated_at = now()
        WHERE reset_token = $2
          AND (reset_expires_at IS NULL OR reset_expires_at > $3)
        RETURNING id, email, password, email_verified,
                  google_id, ft_id,
                  verification_token, verification_sent_at,
                  reset_token, reset_sent_at, reset_expires_at,
                  token_version,
                  created_at, updated_at`,
		newHash, token, now,
	)
	return scanAuthUser(row)
}

func (r *PostgresRepository) DeleteUser(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM auth_users WHERE id = $1`, userID)
	return err
}

func (r *PostgresRepository) UpdateGoogleID(ctx context.Context, userID string, googleID *string) (AuthUser, error) {
	row := r.db.QueryRow(ctx, `UPDATE auth_users
        SET google_id = $1, updated_at = now()
        WHERE id = $2
        RETURNING id, email, password, email_verified,
                  google_id, ft_id,
                  verification_token, verification_sent_at,
                  reset_token, reset_sent_at, reset_expires_at,
                  token_version,
                  created_at, updated_at`,
		googleID, userID,
	)
	return scanAuthUser(row)
}

func (r *PostgresRepository) UpdateFTID(ctx context.Context, userID string, ftID *string) (AuthUser, error) {
	row := r.db.QueryRow(ctx, `UPDATE auth_users
        SET ft_id = $1, updated_at = now()
        WHERE id = $2
        RETURNING id, email, password, email_verified,
                  google_id, ft_id,
                  verification_token, verification_sent_at,
                  reset_token, reset_sent_at, reset_expires_at,
                  token_version,
                  created_at, updated_at`,
		ftID, userID,
	)
	return scanAuthUser(row)
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
		&u.TokenVersion,
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
