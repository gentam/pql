package pql

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Map associates column expressions with values.
type Map map[string]any

// Builder builds SQL and its positional arguments.
type Builder interface {
	// Build returns SQL, positional arguments, and a validation error.
	Build() (string, []any, error)
}

// Querier executes PostgreSQL statements and queries.
// It is satisfied by *[pgxpool.Pool], *[pgxpool.Conn], *[pgx.Conn],
// and [pgx.Tx].
type Querier interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

var pool *pgxpool.Pool

var errPoolNotInitialized = errors.New("pql: pool is not initialized")
var errNilQuerier = errors.New("pql: connection is nil")

// Init sets the pool used by statement execution methods.
func Init(p *pgxpool.Pool) {
	pool = p
}

// Query executes the SELECT statement and returns its rows.
// An optional conn overrides the initialized pool.
func (s *SelectStmt) Query(ctx context.Context, conn ...Querier) (pgx.Rows, error) {
	query, args, err := s.Build()
	if err != nil {
		return nil, err
	}
	db, err := resolveQuerier(conn)
	if err != nil {
		return nil, err
	}
	return db.Query(ctx, query, args...)
}

// QueryRow executes the SELECT statement and returns one row.
// An optional conn overrides the initialized pool.
func (s *SelectStmt) QueryRow(ctx context.Context, conn ...Querier) (pgx.Row, error) {
	query, args, err := s.Build()
	if err != nil {
		return nil, err
	}
	db, err := resolveQuerier(conn)
	if err != nil {
		return nil, err
	}
	return db.QueryRow(ctx, query, args...), nil
}

// Exec executes the UPDATE statement.
// An optional conn overrides the initialized pool.
func (s *UpdateStmt) Exec(ctx context.Context, conn ...Querier) error {
	query, args, err := s.Build()
	if err != nil {
		return err
	}
	db, err := resolveQuerier(conn)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, query, args...)
	return err
}

// ExecRet executes the UPDATE statement and returns one row.
// An optional conn overrides the initialized pool.
func (s *UpdateStmt) ExecRet(ctx context.Context, conn ...Querier) (pgx.Row, error) {
	query, args, err := s.Build()
	if err != nil {
		return nil, err
	}
	db, err := resolveQuerier(conn)
	if err != nil {
		return nil, err
	}
	return db.QueryRow(ctx, query, args...), nil
}

// Exec executes the DELETE statement.
// An optional conn overrides the initialized pool.
func (s *DeleteStmt) Exec(ctx context.Context, conn ...Querier) error {
	query, args, err := s.Build()
	if err != nil {
		return err
	}
	db, err := resolveQuerier(conn)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, query, args...)
	return err
}

// ExecRet executes the DELETE statement and returns one row.
// An optional conn overrides the initialized pool.
func (s *DeleteStmt) ExecRet(ctx context.Context, conn ...Querier) (pgx.Row, error) {
	query, args, err := s.Build()
	if err != nil {
		return nil, err
	}
	db, err := resolveQuerier(conn)
	if err != nil {
		return nil, err
	}
	return db.QueryRow(ctx, query, args...), nil
}

// Exec executes the INSERT statement.
// An optional conn overrides the initialized pool.
func (s *InsertStmt) Exec(ctx context.Context, conn ...Querier) error {
	query, args, err := s.Build()
	if err != nil {
		return err
	}
	db, err := resolveQuerier(conn)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, query, args...)
	return err
}

// ExecRet executes the INSERT statement and returns one row.
// An optional conn overrides the initialized pool.
func (s *InsertStmt) ExecRet(ctx context.Context, conn ...Querier) (pgx.Row, error) {
	query, args, err := s.Build()
	if err != nil {
		return nil, err
	}
	db, err := resolveQuerier(conn)
	if err != nil {
		return nil, err
	}
	return db.QueryRow(ctx, query, args...), nil
}

func initializedPool() (*pgxpool.Pool, error) {
	if pool == nil {
		return nil, errPoolNotInitialized
	}
	return pool, nil
}

func resolveQuerier(conn []Querier) (Querier, error) {
	switch len(conn) {
	case 0:
		return initializedPool()
	case 1:
		if isNilQuerier(conn[0]) {
			return nil, errNilQuerier
		}
		return conn[0], nil
	default:
		return nil, fmt.Errorf("pql: expected at most one connection, got %d", len(conn))
	}
}

func isNilQuerier(q Querier) bool {
	if q == nil {
		return true
	}
	v := reflect.ValueOf(q)
	return v.Kind() == reflect.Pointer && v.IsNil()
}

func buildReturning(b *strings.Builder, returning []string) error {
	b.WriteString(" RETURNING ")
	return writeSQLList(b, returning, "RETURNING expression")
}

func writeSQLList(b *strings.Builder, exprs []string, kind string) error {
	for i, expr := range exprs {
		if strings.TrimSpace(expr) == "" {
			return fmt.Errorf("pql: %s is required", kind)
		}
		if i != 0 {
			b.WriteByte(',')
		}
		b.WriteString(terminateLineComment(expr))
	}
	return nil
}
