package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Mocks ---

type MockDBOps struct {
	mock.Mock
}

func (m *MockDBOps) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	// Pass arguments as variadic to Called to make matching easier or pass slice?
	// mocking libraries usually handle variadic by passing them as slice if signature is variadic
	// But let's pass explicitly
	args := m.Called(ctx, sql, arguments)
	return args.Get(0).(pgconn.CommandTag), args.Error(1)
}

func (m *MockDBOps) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	callArgs := m.Called(ctx, sql, args)
	if val := callArgs.Get(0); val != nil {
		return val.(pgx.Rows), callArgs.Error(1)
	}
	return nil, callArgs.Error(1)
}

func (m *MockDBOps) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	callArgs := m.Called(ctx, sql, args)
	if val := callArgs.Get(0); val != nil {
		return val.(pgx.Row)
	}
	return &RepoMockRow{err: errors.New("unexpected call to QueryRow")}
}

// Renamed to RepoMockRow to avoid collision with mocks_test.go
type RepoMockRow struct {
	mock.Mock
	err    error
	values []interface{}
}

func (m *RepoMockRow) Scan(dest ...any) error {
	if m.err != nil {
		return m.err
	}
	args := m.Called(dest...)
	return args.Error(0)
}

type PredefinedRow struct {
	values []interface{}
	err    error
}

func (r *PredefinedRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return errors.New("scan values count mismatch")
	}
	for i, v := range r.values {
		if v == nil {
			continue
		}

		switch d := dest[i].(type) {
		case *string:
			if strVal, ok := v.(string); ok {
				*d = strVal
			} else if vPtr, ok := v.(*string); ok {
				if vPtr != nil {
					*d = *vPtr
				}
			}
		case **string:
			if strPtr, ok := v.(*string); ok {
				*d = strPtr
			}
		case *bool:
			if boolVal, ok := v.(bool); ok {
				*d = boolVal
			}
		case *time.Time:
			if timeVal, ok := v.(time.Time); ok {
				*d = timeVal
			}
		case **time.Time:
			if timePtr, ok := v.(*time.Time); ok {
				*d = timePtr
			}
		case *int:
			if intVal, ok := v.(int); ok {
				*d = intVal
			}
		case *interface{}:
			*d = v
		}
	}
	return nil
}

func validUserRow(id, email string) []interface{} {
	now := time.Now()
	var nullStr *string
	var nullTime *time.Time

	return []interface{}{
		id,
		email,    // email
		"hash",   // password
		true,     // verified
		nullStr,  // google_id
		nullStr,  // ft_id
		nullStr,  // verification_token
		nullTime, // verification_sent_at
		nullStr,  // reset_token
		nullTime, // reset_sent_at
		nullTime, // reset_expires_at
		1,        // token_version
		now,      // created_at
		now,      // updated_at
	}
}

// --- Tests ---

func TestFindUserByID(t *testing.T) {
	mockDB := new(MockDBOps)
	repo := &PostgresRepository{db: mockDB}
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		row := &PredefinedRow{values: validUserRow("u1", "test@example.com")}
		mockDB.On("QueryRow", ctx, mock.Anything, mock.Anything).Return(row)

		user, err := repo.FindUserByID(ctx, "u1")
		assert.NoError(t, err)
		assert.Equal(t, "u1", user.ID)
	})

	t.Run("Not Found", func(t *testing.T) {
		row := &PredefinedRow{err: pgx.ErrNoRows}
		// Reset expected call to match
		mockDB.ExpectedCalls = nil
		mockDB.On("QueryRow", ctx, mock.Anything, mock.Anything).Return(row)

		_, err := repo.FindUserByID(ctx, "missing")
		assert.ErrorIs(t, err, ErrUserNotFound)
	})
}

func TestSetVerificationToken(t *testing.T) {
	mockDB := new(MockDBOps)
	repo := &PostgresRepository{db: mockDB}
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		// Expect Exec with ANY args
		mockDB.On("Exec", mock.Anything, mock.Anything, mock.Anything).
			Return(pgconn.CommandTag{}, nil)

		err := repo.SetVerificationToken(ctx, "u1", "token")
		assert.NoError(t, err)
	})

	t.Run("Error", func(t *testing.T) {
		mockDB.ExpectedCalls = nil
		mockDB.On("Exec", mock.Anything, mock.Anything, mock.Anything).
			Return(pgconn.CommandTag{}, errors.New("db error"))

		err := repo.SetVerificationToken(ctx, "u1", "token")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})
}

func TestUpsertUserWithGoogle(t *testing.T) {
	mockDB := new(MockDBOps)
	repo := &PostgresRepository{db: mockDB}
	ctx := context.Background()

	t.Run("Link Existing User by Email", func(t *testing.T) {
		// Mock FindUserByGoogleID -> NotFound
		rowNotFound := &PredefinedRow{err: pgx.ErrNoRows}

		// Mock FindUserByEmail -> Success
		rowUser := &PredefinedRow{values: validUserRow("u1", "test@gmail.com")}

		// Mock Update -> Success
		rowUpdate := &PredefinedRow{values: validUserRow("u1", "test@gmail.com")}

		// Sequence of QueryRow calls:
		// 1. FindUserByGoogleID
		// 2. FindUserByEmail
		// 3. Update
		// Since we can't easily sequence same method calls with simple On,
		// we use mock.MatchedBy to differentiate SQL or just assume order if we return different mocks?
		// But QueryRow returns a Row which has Scan called immediately.

		// We can match SQL loosely
		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "WHERE google_id = $1")
		}), mock.Anything).Return(rowNotFound).Once()

		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "WHERE email = $1")
		}), mock.Anything).Return(rowUser).Once()

		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "UPDATE auth_users SET google_id")
		}), mock.Anything).Return(rowUpdate).Once()

		user, err := repo.UpsertUserWithGoogle(ctx, "test@gmail.com", "g1")
		assert.NoError(t, err)
		assert.Equal(t, "u1", user.ID)
	})

	t.Run("Returning User", func(t *testing.T) {
		mockDB.ExpectedCalls = nil
		rowUser := &PredefinedRow{values: validUserRow("u1", "test@gmail.com")}

		// 1. FindGoogle -> Found
		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "WHERE google_id = $1")
		}), mock.Anything).Return(rowUser).Once()

		// If email matches, it just returns. If email differs, it updates.
		// Let's test email match first (simplest path)
		user, err := repo.UpsertUserWithGoogle(ctx, "test@gmail.com", "g1")
		assert.NoError(t, err)
		assert.Equal(t, "u1", user.ID)
	})

	t.Run("Create New User", func(t *testing.T) {
		mockDB.ExpectedCalls = nil
		rowNotFound := &PredefinedRow{err: pgx.ErrNoRows}
		rowNew := &PredefinedRow{values: validUserRow("new-u", "new@gmail.com")}

		// 1. FindGoogle -> 404
		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "WHERE google_id = $1")
		}), mock.Anything).Return(rowNotFound).Once()

		// 2. FindEmail -> 404
		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "WHERE email = $1")
		}), mock.Anything).Return(rowNotFound).Once()

		// 3. Insert
		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "INSERT INTO auth_users")
		}), mock.Anything).Return(rowNew).Once()

		user, err := repo.UpsertUserWithGoogle(ctx, "new@gmail.com", "g2")
		assert.NoError(t, err)
		assert.Equal(t, "new-u", user.ID)
	})
}

func TestDeleteUser(t *testing.T) {
	mockDB := new(MockDBOps)
	repo := &PostgresRepository{db: mockDB}
	ctx := context.Background()

	mockDB.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return contains(sql, "DELETE FROM auth_users")
	}), mock.Anything).Return(pgconn.CommandTag{}, nil)

	err := repo.DeleteUser(ctx, "u1")
	assert.NoError(t, err)
}

func TestFindUserByFTID(t *testing.T) {
	mockDB := new(MockDBOps)
	repo := &PostgresRepository{db: mockDB}
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		row := &PredefinedRow{values: validUserRow("u1", "test@42.fr")}
		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "WHERE ft_id = $1")
		}), mock.Anything).Return(row)

		user, err := repo.FindUserByFTID(ctx, "ft:42")
		assert.NoError(t, err)
		assert.Equal(t, "u1", user.ID)
	})
}

func TestUpsertUserWithFT(t *testing.T) {
	mockDB := new(MockDBOps)
	repo := &PostgresRepository{db: mockDB}
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		row := &PredefinedRow{values: validUserRow("u1", "test@42.fr")}
		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "INSERT INTO auth_users") && contains(sql, "ft_id")
		}), mock.Anything).Return(row)

		user, err := repo.UpsertUserWithFT(ctx, "test@42.fr", "ft:42")
		assert.NoError(t, err)
		assert.Equal(t, "u1", user.ID)
	})
}

func TestVerifyEmailByToken(t *testing.T) {
	mockDB := new(MockDBOps)
	repo := &PostgresRepository{db: mockDB}
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		row := &PredefinedRow{values: validUserRow("u1", "test@example.com")}
		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "UPDATE auth_users") && contains(sql, "verification_token")
		}), mock.Anything).Return(row)

		user, err := repo.VerifyEmailByToken(ctx, "token")
		assert.NoError(t, err)
		assert.Equal(t, "u1", user.ID)
	})
}

func TestSetResetToken(t *testing.T) {
	mockDB := new(MockDBOps)
	repo := &PostgresRepository{db: mockDB}
	ctx := context.Background()

	mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
		return contains(sql, "set_reset_token") || contains(sql, "reset_token = $1")
	}), mock.Anything).Return(pgconn.CommandTag{}, nil)

	err := repo.SetResetToken(ctx, "u1", "token", time.Now())
	assert.NoError(t, err)
}

func TestResetPasswordByToken(t *testing.T) {
	mockDB := new(MockDBOps)
	repo := &PostgresRepository{db: mockDB}
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		row := &PredefinedRow{values: validUserRow("u1", "test@example.com")}
		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "UPDATE auth_users") && contains(sql, "reset_token")
		}), mock.Anything).Return(row)

		user, err := repo.ResetPasswordByToken(ctx, "token", "newhash", time.Now())
		assert.NoError(t, err)
		assert.Equal(t, "u1", user.ID)
	})
}

func TestUpdateGoogleID(t *testing.T) {
	mockDB := new(MockDBOps)
	repo := &PostgresRepository{db: mockDB}
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		row := &PredefinedRow{values: validUserRow("u1", "test@example.com")}
		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "UPDATE auth_users") && contains(sql, "google_id = $1")
		}), mock.Anything).Return(row)

		val := "new-google-id"
		user, err := repo.UpdateGoogleID(ctx, "u1", &val)
		assert.NoError(t, err)
		assert.Equal(t, "u1", user.ID)
	})
}

func TestUpdateFTID(t *testing.T) {
	mockDB := new(MockDBOps)
	repo := &PostgresRepository{db: mockDB}
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		row := &PredefinedRow{values: validUserRow("u1", "test@example.com")}
		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "UPDATE auth_users") && contains(sql, "ft_id = $1")
		}), mock.Anything).Return(row)

		val := "new-ft-id"
		user, err := repo.UpdateFTID(ctx, "u1", &val)
		assert.NoError(t, err)
		assert.Equal(t, "u1", user.ID)
	})
}

func contains(s, substr string) bool {
	// inefficient but enough for tests
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
