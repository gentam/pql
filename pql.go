package pql

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Map associates columns with bound values for INSERT and UPDATE statements.
// It is a shorthand for map literals; [InsertStmt.Values] and [UpdateStmt.Values]
// also accept map[string]any and other types defined as map[string]any.
type Map map[string]any

// SQLExpr is an SQL expression assigned to a column by [InsertStmt.Set],
// [UpdateStmt.Set], or their Values methods, in place of a bound value.
// Create one with [SQL]. The zero value is invalid.
type SQLExpr struct {
	expr string
	args []any
}

// SQL creates an expression from trusted SQL and bound arguments, for use as
// a value assigned by INSERT or UPDATE:
//
//	Update("items").Set("updated_at", SQL("now()"))
//	Update("items").Set("views", SQL("views + $", n))
//	Insert("items").Set("due_date", SQL("$::date", "2026-01-31"))
//	Insert("items").Set("id", SQL("DEFAULT"))
//
// The expression is written verbatim, without parentheses. Placeholders follow
// the rules of [Where]: write unnumbered $ placeholders, one per argument.
// PostgreSQL infers a parameter's type from its context; cast parameters whose
// type it cannot infer, such as in SQL("$::integer * $", a, b).
// Empty expressions and placeholder errors are reported when the statement is
// built. The argument list is copied; bound values are not deep-copied.
func SQL(expr string, args ...any) SQLExpr {
	return SQLExpr{expr: expr, args: slices.Clone(args)}
}

// writeValue writes the value assigned to a column: an [SQLExpr], or a
// placeholder for a bound value. clause names the clause in error messages.
func writeValue(b *strings.Builder, value any, clause string, args []any) ([]any, error) {
	e, ok := value.(SQLExpr)
	if !ok {
		args = append(args, value)
		b.WriteByte('$')
		b.WriteString(strconv.Itoa(len(args)))
		return args, nil
	}
	if strings.TrimSpace(e.expr) == "" {
		return nil, fmt.Errorf("pql: %s expression is required", clause)
	}
	sql, err := numberPlaceholders(e.expr, clause, len(args), len(e.args))
	if err != nil {
		return nil, err
	}
	b.WriteString(sql)
	return append(args, e.args...), nil
}

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

var defaultPool atomic.Pointer[pgxpool.Pool]

var errDefaultPoolNotSet = errors.New("pql: default pool is not set")
var errNilQuerier = errors.New("pql: connection is nil")

// SetDefaultPool sets the pool used when no connection is supplied to a statement
// execution method. Passing nil clears the default pool.
//
// It is safe to call concurrently with statement execution. Each execution
// uses the pool configured when it starts.
func SetDefaultPool(p *pgxpool.Pool) {
	defaultPool.Store(p)
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
// It executes SELECT EXISTS (...) without modifying the statement.
// PostgreSQL does not evaluate the result expressions of an EXISTS subquery,
// so there is no need to replace them; an aggregate expression, however, still
// makes the statement return one row. LIMIT and OFFSET are preserved; ORDER BY
// is omitted unless one of them is set, because it cannot change the result.
// See [Querier] for connection selection.
func (s *SelectStmt) Exists(ctx context.Context, conn ...Querier) (bool, error) {
	var exists bool
	if err := s.queryScalar(ctx, conn, "SELECT EXISTS (", ")", &exists); err != nil {
		return false, err
	}
	return exists, nil
}

// Count reports the number of rows the SELECT statement returns.
// It executes SELECT count(*) FROM (...) without modifying the statement, so
// DISTINCT, aggregate, and set-returning result expressions are respected.
// LIMIT and OFFSET are preserved; ORDER BY is omitted unless one of them is set,
// because it cannot change the count. Use it to count the rows of a filtered
// statement before adding pagination. See [Querier] for connection selection.
func (s *SelectStmt) Count(ctx context.Context, conn ...Querier) (int64, error) {
	var count int64
	if err := s.queryScalar(ctx, conn, "SELECT count(*) FROM (", ") AS pql_count", &count); err != nil {
		return 0, err
	}
	return count, nil
}

// queryScalar wraps the statement in prefix and suffix and scans one value.
func (s *SelectStmt) queryScalar(ctx context.Context, conn []Querier, prefix, suffix string, dest any) error {
	stmt := s
	if len(s.order) != 0 && !s.limitSet && !s.offsetSet {
		// Omitted terms are still validated, as Build would.
		for _, order := range s.order {
			if _, err := order.sql(); err != nil {
				return err
			}
		}
		unordered := *s
		unordered.order = nil
		stmt = &unordered
	}
	query, args, err := stmt.Build()
	if err != nil {
		return err
	}
	db, err := resolveQuerier(conn)
	if err != nil {
		return err
	}
	return db.QueryRow(ctx, prefix+query+suffix, args...).Scan(dest)
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
		pool := defaultPool.Load()
		if pool == nil {
			return nil, errDefaultPoolNotSet
		}
		return pool, nil
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

// startArgs returns an argument slice whose length reserves the first offset
// parameter numbers, so that placeholders are numbered from offset+1.
func startArgs(offset int) ([]any, error) {
	if offset < 0 {
		return nil, errors.New("pql: parameter offset must not be negative")
	}
	if offset == 0 {
		return nil, nil
	}
	return make([]any, offset), nil
}

// finishArgs removes the reserved prefix added by startArgs.
func finishArgs(args []any, offset int) []any {
	if len(args) == offset {
		return nil
	}
	return args[offset:]
}

func buildReturning(b *strings.Builder, returning []string) error {
	b.WriteString(" RETURNING ")
	return writeSQLList(b, returning, "RETURNING expression")
}

func writeSQLList(b *strings.Builder, exprs []string, kind string) error {
	for i, expr := range exprs {
		if i != 0 {
			b.WriteByte(',')
		}
		if err := writeRawSQL(b, expr, kind); err != nil {
			return err
		}
	}
	return nil
}

// writeRawSQL writes a required trusted SQL fragment; see [rawSQL].
func writeRawSQL(b *strings.Builder, expr, kind string) error {
	if strings.TrimSpace(expr) == "" {
		return fmt.Errorf("pql: %s is required", kind)
	}
	sql, err := rawSQL(expr, kind)
	if err != nil {
		return err
	}
	b.WriteString(sql)
	return nil
}
