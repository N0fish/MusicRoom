package main

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureUsername(t *testing.T) {
	s, mock := setupMockServer(t)
	defer mock.Close()

	ctx := context.Background()

	t.Run("AlreadyHasUsername", func(t *testing.T) {
		prof := UserProfile{UserID: "uid-1", Username: "valid"}
		final, err := s.ensureUsername(ctx, prof)
		require.NoError(t, err)
		assert.Equal(t, "valid", final.Username)
	})

	t.Run("GenerateNew_Success", func(t *testing.T) {
		// Empty username triggers generation
		prof := UserProfile{UserID: "uid-2", Username: ""}

		// 1. generateRandomUsername is called internally
		// 2. Checks existence in DB. Mock returning false (not taken)
		mock.ExpectQuery("SELECT EXISTS.*user_profiles").
			WithArgs(pgxmock.AnyArg()). // random name
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))

		// 3. Updates DB
		mock.ExpectExec("UPDATE user_profiles").
			WithArgs(pgxmock.AnyArg(), prof.UserID).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		final, err := s.ensureUsername(ctx, prof)
		require.NoError(t, err)
		assert.NotEmpty(t, final.Username)
		assert.NotEqual(t, "", final.Username)
	})

	t.Run("GenerateNew_RetryLoop", func(t *testing.T) {
		prof := UserProfile{UserID: "uid-3", Username: ""}

		// 1. First attempt: Taken
		mock.ExpectQuery("SELECT EXISTS.*user_profiles").
			WithArgs(pgxmock.AnyArg()).
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))

		// 2. Second attempt: Not taken
		mock.ExpectQuery("SELECT EXISTS.*user_profiles").
			WithArgs(pgxmock.AnyArg()).
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))

		// 3. Update
		mock.ExpectExec("UPDATE user_profiles").
			WithArgs(pgxmock.AnyArg(), prof.UserID).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		final, err := s.ensureUsername(ctx, prof)
		require.NoError(t, err)
		assert.NotEmpty(t, final.Username)
	})
}

func TestGenerateRandomUsername(t *testing.T) {
	name := generateRandomUsername()
	assert.NotEmpty(t, name)
	assert.Greater(t, len(name), 5)
	assert.Contains(t, name, "-")
}

func TestGetOrCreateProfile(t *testing.T) {
	s, mock := setupMockServer(t)
	defer mock.Close()

	ctx := context.Background()
	userID := "new-user-id"

	// 1. findProfileByUserID fails with ErrProfileNotFound (via pgx.ErrNoRows)
	mock.ExpectQuery("SELECT id, user_id").
		WithArgs(userID).
		WillReturnError(pgx.ErrNoRows)

	// 2. INSERT ... RETURNING ...
	rows := pgxmock.NewRows([]string{
		"id", "user_id", "display_name", "username", "avatar_url",
		"has_custom_avatar", "bio", "visibility", "preferences", "is_premium",
		"created_at", "updated_at",
	}).AddRow(
		"p-id", userID, "", "", "avatar.jpg",
		false, "", "public", []byte("{}"),
		false, time.Now(), time.Now(),
	)
	mock.ExpectQuery("INSERT INTO user_profiles").
		WithArgs(userID).
		WillReturnRows(rows)

	// 3. ensureUsername called internally
	// a) Generates name
	// b) Checks exists -> false (not taken)
	mock.ExpectQuery("SELECT EXISTS.*user_profiles").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))

	// c) Updates username
	mock.ExpectExec("UPDATE user_profiles").
		WithArgs(pgxmock.AnyArg(), userID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	prof, err := s.getOrCreateProfile(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, userID, prof.UserID)
	assert.NotEmpty(t, prof.Username)
}
