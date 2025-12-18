package playlist

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// MockDB implements DB interface for testing.
type MockDB struct {
	ExecFunc     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryFunc    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRowFunc func(ctx context.Context, sql string, args ...any) pgx.Row
	BeginTxFunc  func(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

func (m *MockDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if m.ExecFunc != nil {
		return m.ExecFunc(ctx, sql, args...)
	}
	return pgconn.CommandTag{}, nil
}

func (m *MockDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.QueryFunc != nil {
		return m.QueryFunc(ctx, sql, args...)
	}
	return nil, nil
}

func (m *MockDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.QueryRowFunc != nil {
		return m.QueryRowFunc(ctx, sql, args...)
	}
	return &MockRow{}
}

func (m *MockDB) BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error) {
	if m.BeginTxFunc != nil {
		return m.BeginTxFunc(ctx, txOptions)
	}
	return &MockTx{}, nil
}

// MockRow implements pgx.Row
type MockRow struct {
	ScanFunc func(dest ...any) error
}

func (m *MockRow) Scan(dest ...any) error {
	if m.ScanFunc != nil {
		return m.ScanFunc(dest...)
	}
	return nil
}

// MockTx implements pgx.Tx
type MockTx struct {
	pgx.Tx // Embed to satisfy interface; unchecked methods will panic if called

	CommitFunc   func(ctx context.Context) error
	RollbackFunc func(ctx context.Context) error
	ExecFunc     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRowFunc func(ctx context.Context, sql string, args ...any) pgx.Row
	QueryFunc    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (m *MockTx) Commit(ctx context.Context) error {
	if m.CommitFunc != nil {
		return m.CommitFunc(ctx)
	}
	return nil
}

func (m *MockTx) Rollback(ctx context.Context) error {
	if m.RollbackFunc != nil {
		return m.RollbackFunc(ctx)
	}
	return nil
}

func (m *MockTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if m.ExecFunc != nil {
		return m.ExecFunc(ctx, sql, args...)
	}
	return pgconn.CommandTag{}, nil
}

func (m *MockTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.QueryRowFunc != nil {
		return m.QueryRowFunc(ctx, sql, args...)
	}
	return &MockRow{}
}

func (m *MockTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.QueryFunc != nil {
		return m.QueryFunc(ctx, sql, args...)
	}
	return nil, nil // Return nil or empty rows
}

// MockRows Helper for list queries
type MockRows struct {
	pgx.Rows
	Data [][]any
	Idx  int
}

func (m *MockRows) Next() bool {
	m.Idx++
	return m.Idx < len(m.Data)
}

func (m *MockRows) Scan(dest ...any) error {
	row := m.Data[m.Idx]
	if len(dest) != len(row) {
		return errors.New("column count mismatch")
	}
	for i, v := range row {
		if dest[i] == nil {
			continue
		}
		switch d := dest[i].(type) {
		case *string:
			*d = v.(string)
		case *time.Time:
			*d = v.(time.Time)
		case *bool:
			*d = v.(bool)
		case *int:
			*d = v.(int)
		}
	}
	return nil
}

func (m *MockRows) Close()                                       {}
func (m *MockRows) Err() error                                   { return nil }
func (m *MockRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (m *MockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (m *MockRows) Values() ([]any, error)                       { return nil, nil }
func (m *MockRows) RawValues() [][]byte                          { return nil }
func (m *MockRows) Conn() *pgx.Conn                              { return nil }
