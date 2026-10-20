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
//
// Statement execution methods accept at most one non-nil connection.
// When omitted, they use the pool configured with [SetDefaultPool] and report
// an error if no pool is set.
type Querier interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

var defaultPool *pgxpool.Pool

var errDefaultPoolNotSet = errors.New("pql: default pool is not set")
var errNilQuerier = errors.New("pql: connection is nil")

// SetDefaultPool sets the pool used when no connection is supplied to a statement
// execution method. Passing nil clears the default pool.
//
// Configure the default pool before executing statements concurrently.
func SetDefaultPool(p *pgxpool.Pool) {
	defaultPool = p
}

// Query executes the SELECT statement and returns its rows.
// See [Querier] for connection selection.
func (s *SelectStmt) Query(ctx context.Context, conn ...Querier) (pgx.Rows, error) {
	return query(ctx, s, conn)
}

// QueryRow executes the SELECT statement and returns its first row.
// Scan reports all errors, including build and connection errors, and returns
// [pgx.ErrNoRows] if the statement returns no rows.
// See [Querier] for connection selection.
func (s *SelectStmt) QueryRow(ctx context.Context, conn ...Querier) pgx.Row {
	return queryRow(ctx, s, conn)
}

// Exists reports whether the SELECT statement returns any rows.
// It executes SELECT EXISTS (...) without modifying the statement and preserves
// its LIMIT/OFFSET settings. See [Querier] for connection selection.
func (s *SelectStmt) Exists(ctx context.Context, conn ...Querier) (bool, error) {
	query, args, err := s.Build()
	if err != nil {
		return false, err
	}
	db, err := resolveQuerier(conn)
	if err != nil {
		return false, err
	}
	var exists bool
	if err := db.QueryRow(ctx, "SELECT EXISTS ("+query+")", args...).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

// Exec executes the UPDATE statement and returns its command tag.
// Any returned rows are discarded. See [Querier] for connection selection.
func (s *UpdateStmt) Exec(ctx context.Context, conn ...Querier) (pgconn.CommandTag, error) {
	return exec(ctx, s, conn)
}

// Query executes the UPDATE statement and returns its rows.
// Use [UpdateStmt.Returning] to specify result columns.
// See [Querier] for connection selection.
func (s *UpdateStmt) Query(ctx context.Context, conn ...Querier) (pgx.Rows, error) {
	return query(ctx, s, conn)
}

// QueryRow executes the UPDATE statement and returns its first row.
// Use [UpdateStmt.Returning] to specify result columns. Scan reports all errors,
// including build and connection errors, and returns [pgx.ErrNoRows] if there
// are no rows. See [Querier] for connection selection.
func (s *UpdateStmt) QueryRow(ctx context.Context, conn ...Querier) pgx.Row {
	return queryRow(ctx, s, conn)
}

// Exec executes the DELETE statement and returns its command tag.
// Any returned rows are discarded. See [Querier] for connection selection.
func (s *DeleteStmt) Exec(ctx context.Context, conn ...Querier) (pgconn.CommandTag, error) {
	return exec(ctx, s, conn)
}

// Query executes the DELETE statement and returns its rows.
// Use [DeleteStmt.Returning] to specify result columns.
// See [Querier] for connection selection.
func (s *DeleteStmt) Query(ctx context.Context, conn ...Querier) (pgx.Rows, error) {
	return query(ctx, s, conn)
}

// QueryRow executes the DELETE statement and returns its first row.
// Use [DeleteStmt.Returning] to specify result columns. Scan reports all errors,
// including build and connection errors, and returns [pgx.ErrNoRows] if there
// are no rows. See [Querier] for connection selection.
func (s *DeleteStmt) QueryRow(ctx context.Context, conn ...Querier) pgx.Row {
	return queryRow(ctx, s, conn)
}

// Exec executes the INSERT statement and returns its command tag.
// Any returned rows are discarded. See [Querier] for connection selection.
func (s *InsertStmt) Exec(ctx context.Context, conn ...Querier) (pgconn.CommandTag, error) {
	return exec(ctx, s, conn)
}

// Query executes the INSERT statement and returns its rows.
// Use [InsertStmt.Returning] to specify result columns.
// See [Querier] for connection selection.
func (s *InsertStmt) Query(ctx context.Context, conn ...Querier) (pgx.Rows, error) {
	return query(ctx, s, conn)
}

// QueryRow executes the INSERT statement and returns its first row.
// Use [InsertStmt.Returning] to specify result columns. Scan reports all errors,
// including build and connection errors, and returns [pgx.ErrNoRows] if there
// are no rows. See [Querier] for connection selection.
func (s *InsertStmt) QueryRow(ctx context.Context, conn ...Querier) pgx.Row {
	return queryRow(ctx, s, conn)
}

func query(ctx context.Context, stmt Builder, conn []Querier) (pgx.Rows, error) {
	query, args, err := stmt.Build()
	if err != nil {
		return nil, err
	}
	db, err := resolveQuerier(conn)
	if err != nil {
		return nil, err
	}
	return db.Query(ctx, query, args...)
}

func queryRow(ctx context.Context, stmt Builder, conn []Querier) pgx.Row {
	query, args, err := stmt.Build()
	if err != nil {
		return errorRow{err: err}
	}
	db, err := resolveQuerier(conn)
	if err != nil {
		return errorRow{err: err}
	}
	return db.QueryRow(ctx, query, args...)
}

func exec(ctx context.Context, stmt Builder, conn []Querier) (pgconn.CommandTag, error) {
	query, args, err := stmt.Build()
	if err != nil {
		return pgconn.CommandTag{}, err
	}
	db, err := resolveQuerier(conn)
	if err != nil {
		return pgconn.CommandTag{}, err
	}
	return db.Exec(ctx, query, args...)
}

type errorRow struct {
	err error
}

func (r errorRow) Scan(...any) error { return r.err }

func resolveQuerier(conn []Querier) (Querier, error) {
	switch len(conn) {
	case 0:
		if defaultPool == nil {
			return nil, errDefaultPoolNotSet
		}
		return defaultPool, nil
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
