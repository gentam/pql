package pql

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ Querier = (*pgxpool.Pool)(nil)
var _ Querier = (*pgxpool.Conn)(nil)
var _ Querier = (*pgx.Conn)(nil)
var _ Querier = (pgx.Tx)(nil)

func TestExecutionWithoutDefaultPool(t *testing.T) {
	// Keep serial because SetDefaultPool changes the package-level pool.
	SetDefaultPool(nil)
	for _, tt := range executionStatements() {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			if rows, err := tt.stmt.Query(ctx); rows != nil || err != errDefaultPoolNotSet {
				t.Fatalf("Query() = (%v, %v), want (nil, %v)", rows, err, errDefaultPoolNotSet)
			}
			if err := tt.stmt.QueryRow(ctx).Scan(); err != errDefaultPoolNotSet {
				t.Fatalf("Scan() = %v, want %v", err, errDefaultPoolNotSet)
			}
			if stmt, ok := tt.stmt.(execStatement); ok {
				if tag, err := stmt.Exec(ctx); tag != (pgconn.CommandTag{}) || err != errDefaultPoolNotSet {
					t.Fatalf("Exec() = (%v, %v), want (zero tag, %v)", tag, err, errDefaultPoolNotSet)
				}
			}
		})
	}
	if exists, err := Select("1").Exists(t.Context()); exists || err != errDefaultPoolNotSet {
		t.Fatalf("Exists() = (%v, %v), want (false, %v)", exists, err, errDefaultPoolNotSet)
	}
}

func TestStatementExecution(t *testing.T) {
	t.Parallel()
	for _, tt := range executionStatements() {
		for _, result := range []struct {
			name string
			err  error
		}{
			{name: "success"},
			{name: "database error", err: errors.New("database error")},
			{name: "canceled", err: context.Canceled},
		} {
			t.Run(tt.name+"/"+result.name, func(t *testing.T) {
				ctx := t.Context()
				calls := map[string]int{}
				check := func(method string, gotCtx context.Context, query string, args []any) {
					calls[method]++
					if gotCtx != ctx || query != tt.query || !reflect.DeepEqual(args, []any{7}) {
						t.Errorf("%s received context %v, query %q, args %v", method, gotCtx, query, args)
					}
				}
				wantRows := &stubRows{}
				wantTag := pgconn.NewCommandTag(tt.tag)
				scans := 0
				value := 0
				conn := stubQuerier{
					query: func(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
						check("Query", ctx, query, args)
						return wantRows, result.err
					},
					queryRow: func(ctx context.Context, query string, args ...any) pgx.Row {
						check("QueryRow", ctx, query, args)
						return scanRow(func(dest ...any) error {
							scans++
							if len(dest) != 1 || dest[0] != &value {
								t.Fatal("Scan did not receive the supplied destination")
							}
							if result.err == nil {
								value = 7
							}
							return result.err
						})
					},
					exec: func(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
						check("Exec", ctx, query, args)
						return wantTag, result.err
					},
				}
				if rows, err := tt.stmt.Query(ctx, conn); rows != wantRows || !errors.Is(err, result.err) {
					t.Fatalf("Query() = (%v, %v), want (%v, %v)", rows, err, wantRows, result.err)
				}
				row := tt.stmt.QueryRow(ctx, conn)
				if row == nil || calls["QueryRow"] != 1 || scans != 0 {
					t.Fatal("QueryRow must execute once and defer scanning to the caller")
				}
				if err := row.Scan(&value); !errors.Is(err, result.err) {
					t.Fatalf("Scan() = %v, want %v", err, result.err)
				}
				if scans != 1 || (result.err == nil && value != 7) {
					t.Fatalf("Scan called %d times, value = %d", scans, value)
				}
				if stmt, ok := tt.stmt.(execStatement); ok {
					if tag, err := stmt.Exec(ctx, conn); tag != wantTag || !errors.Is(err, result.err) {
						t.Fatalf("Exec() = (%v, %v), want (%v, %v)", tag, err, wantTag, result.err)
					}
					if calls["Exec"] != 1 {
						t.Fatalf("Exec called %d times", calls["Exec"])
					}
				}
				if calls["Query"] != 1 || calls["QueryRow"] != 1 {
					t.Fatalf("unexpected calls: %v", calls)
				}
			})
		}
	}
}

func TestStatementExecutionValidation(t *testing.T) {
	t.Parallel()
	var nilPool *pgxpool.Pool
	for _, tt := range executionStatements() {
		for _, connection := range []struct {
			name string
			conn []Querier
			err  string
		}{
			{"nil connection", []Querier{nil}, errNilQuerier.Error()},
			{"typed nil connection", []Querier{nilPool}, errNilQuerier.Error()},
			{"multiple connections", []Querier{stubQuerier{}, stubQuerier{}}, "pql: expected at most one connection, got 2"},
		} {
			t.Run(tt.name+"/"+connection.name, func(t *testing.T) {
				assertExecutionError(t, tt.stmt, connection.conn, connection.err)
			})
		}
	}
	for _, tt := range []struct {
		name string
		stmt queryStatement
		err  string
	}{
		{"select", Select().Limit(-1), "pql: LIMIT must not be negative"},
		{"insert", Insert(""), "pql: INSERT table is required"},
		{"update", Update("t"), "pql: UPDATE values are required"},
		{"delete", Delete(""), "pql: DELETE table is required"},
		{"select empty result", Select("id", ""), "pql: SELECT expression is required"},
		{"select blank from", Select("1").From(" \t "), "pql: FROM expression is required"},
		{"insert empty column", Insert("t").Set("", 1), "pql: INSERT column is required"},
		{"update empty column", Update("t").Set("", 1), "pql: UPDATE column is required"},
		{"delete empty returning", Delete("t").Returning(""), "pql: RETURNING expression is required"},
		{"select zero composition", Select().Apply(And(Where("true"), WhereExpr{})), "pql: WHERE condition is required"},
		{"update zero composition", Update("t").Set("x", 0).Apply(Not(WhereExpr{})), "pql: WHERE condition is required"},
		{"delete zero composition", Delete("t").Apply(Or(WhereExpr{})), "pql: WHERE condition is required"},
		{"select missing argument", Select().Apply(Where("id=$")), "pql: WHERE placeholder count 1 does not match argument count 0"},
		{"update numbered placeholder", Update("t").Set("c", 1).Apply(Where("id=$1")), "pql: numbered placeholders are not supported in WHERE expressions; use $"},
	} {
		t.Run(tt.name+"/build error", func(t *testing.T) {
			// Build errors take precedence over connection errors and never reach the database.
			assertExecutionError(t, tt.stmt, []Querier{nil}, tt.err)
			assertExecutionError(t, tt.stmt, []Querier{stubQuerier{}}, tt.err)
		})
	}
}

func assertExecutionError(t *testing.T, stmt queryStatement, conn []Querier, want string) {
	t.Helper()
	ctx := t.Context()
	if rows, err := stmt.Query(ctx, conn...); rows != nil || err == nil || err.Error() != want {
		t.Fatalf("Query() = (%v, %v), want (nil, %q)", rows, err, want)
	}
	row := stmt.QueryRow(ctx, conn...)
	if row == nil {
		t.Fatal("QueryRow returned nil instead of an error-bearing row")
	}
	value := 42
	if err := row.Scan(&value); err == nil || err.Error() != want || value != 42 {
		t.Fatalf("Scan() = %v, value = %d; want %q, 42", err, value, want)
	}
	if stmt, ok := stmt.(execStatement); ok {
		if tag, err := stmt.Exec(ctx, conn...); tag != (pgconn.CommandTag{}) || err == nil || err.Error() != want {
			t.Fatalf("Exec() = (%v, %v), want (zero tag, %q)", tag, err, want)
		}
	}
}

type queryStatement interface {
	Query(context.Context, ...Querier) (pgx.Rows, error)
	QueryRow(context.Context, ...Querier) pgx.Row
}

type execStatement interface {
	Exec(context.Context, ...Querier) (pgconn.CommandTag, error)
}

type executionCase struct {
	name  string
	stmt  queryStatement
	query string
	tag   string
}

func executionStatements() []executionCase {
	return []executionCase{
		{"select", Select("n").From("t").Apply(Where("n=$", 7)), "SELECT n FROM t WHERE (n=$1)", ""},
		{"insert", Insert("t").Set("n", 7).Returning("n"), "INSERT INTO t (n) VALUES ($1) RETURNING n", "INSERT 0 1"},
		{"update", Update("t").Set("n", 7).Returning("n"), "UPDATE t SET n=$1 RETURNING n", "UPDATE 2"},
		{"delete", Delete("t").Apply(Where("n=$", 7)).Returning("n"), "DELETE FROM t WHERE (n=$1) RETURNING n", "DELETE 2"},
	}
}

func TestResolveQuerier(t *testing.T) {
	// Keep this test and its subtests serial because SetDefaultPool changes the package-level pool.
	defaultPool := &pgxpool.Pool{}
	explicitPool := &pgxpool.Pool{}

	t.Cleanup(func() { SetDefaultPool(nil) })
	tests := []struct {
		name    string
		pool    *pgxpool.Pool
		conn    []Querier
		want    Querier
		wantErr string
	}{
		{
			name: "default pool",
			pool: defaultPool,
			want: defaultPool,
		},
		{
			name: "explicit connection without default pool",
			conn: []Querier{explicitPool},
			want: explicitPool,
		},
		{
			name: "explicit connection overrides default pool",
			pool: defaultPool,
			conn: []Querier{explicitPool},
			want: explicitPool,
		},
		{
			name:    "unset default pool",
			wantErr: "pql: default pool is not set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaultPool(tt.pool)
			got, err := resolveQuerier(tt.conn)
			if err != nil {
				if err.Error() != tt.wantErr {
					t.Fatalf("error = %q, want %q", err, tt.wantErr)
				}
				return
			}
			if tt.wantErr != "" {
				t.Fatalf("error = nil, want %q", tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("connection = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveQuerierValidation(t *testing.T) {
	t.Parallel()
	var nilPool *pgxpool.Pool
	for _, tt := range []struct {
		name string
		conn []Querier
		err  string
	}{
		{
			name: "nil connection",
			conn: []Querier{nil},
			err:  "pql: connection is nil",
		},
		{
			name: "typed nil connection",
			conn: []Querier{nilPool},
			err:  "pql: connection is nil",
		},
		{
			name: "multiple connections",
			conn: []Querier{&pgxpool.Pool{}, &pgxpool.Pool{}},
			err:  "pql: expected at most one connection, got 2",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveQuerier(tt.conn)
			if got != nil || err == nil || err.Error() != tt.err {
				t.Fatalf("resolveQuerier() = (%v, %v), want (nil, %q)", got, err, tt.err)
			}
		})
	}
}

func TestSelectExists(t *testing.T) {
	t.Parallel()
	queryErr := errors.New("query failed")
	tests := []struct {
		name   string
		stmt   *SelectStmt
		query  string
		args   []any
		row    bool
		rowErr error
	}{
		{
			name:  "matching row with bound values",
			stmt:  Select("id", "title").From("items").Apply(And(Where("status=$", "OPEN"), Where("id > $", 10))),
			query: "SELECT EXISTS (SELECT id,title FROM items WHERE ((status=$1) AND (id > $2)))",
			args:  []any{"OPEN", 10},
			row:   true,
		},
		{
			name:  "no matching rows",
			stmt:  Select().From("items"),
			query: "SELECT EXISTS (SELECT * FROM items)",
		},
		{
			name:  "ordering limit and offset",
			stmt:  Select("id").From("items").Desc("id").Limit(10).Offset(5),
			query: "SELECT EXISTS (SELECT id FROM items ORDER BY id DESC LIMIT 10 OFFSET 5)",
			row:   true,
		},
		{
			name:  "zero limit and offset",
			stmt:  Select("1").From("items").Limit(0).Offset(0),
			query: "SELECT EXISTS (SELECT 1 FROM items LIMIT 0 OFFSET 0)",
		},
		{
			name:  "aggregate projection",
			stmt:  Select("count(*)").From("items"),
			query: "SELECT EXISTS (SELECT count(*) FROM items)",
			row:   true,
		},
		{
			name:   "query error from scan",
			stmt:   Select("1"),
			query:  "SELECT EXISTS (SELECT 1)",
			row:    true,
			rowErr: queryErr,
		},
		{
			name:   "cancellation from scan",
			stmt:   Select("1"),
			query:  "SELECT EXISTS (SELECT 1)",
			rowErr: context.Canceled,
		},
		{
			name:   "unexpected missing result row",
			stmt:   Select("1"),
			query:  "SELECT EXISTS (SELECT 1)",
			rowErr: pgx.ErrNoRows,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			calls := 0
			conn := stubQuerier{queryRow: func(gotCtx context.Context, query string, args ...any) pgx.Row {
				calls++
				if gotCtx != ctx {
					t.Error("QueryRow did not receive the supplied context")
				}
				if query != tt.query || !reflect.DeepEqual(args, tt.args) {
					t.Errorf("QueryRow got (%q, %#v), want (%q, %#v)", query, args, tt.query, tt.args)
				}
				return scanRow(func(dest ...any) error {
					if len(dest) != 1 {
						t.Fatalf("Scan received %d destinations, want 1", len(dest))
					}
					exists, ok := dest[0].(*bool)
					if !ok {
						t.Fatalf("Scan destination = %T, want *bool", dest[0])
					}
					*exists = tt.row
					return tt.rowErr
				})
			}}
			query, args, err := tt.stmt.Build()
			if err != nil {
				t.Fatal(err)
			}
			exists, err := tt.stmt.Exists(ctx, conn)
			if want := tt.row && tt.rowErr == nil; exists != want || !errors.Is(err, tt.rowErr) {
				t.Errorf("Exists() = (%v, %v), want (%v, %v)", exists, err, want, tt.rowErr)
			}
			if calls != 1 {
				t.Errorf("QueryRow called %d times, want 1", calls)
			}
			assertBuild(t, tt.stmt, buildResult{query: query, args: args})
		})
	}
}

func TestSelectExistsOverridesDefaultPool(t *testing.T) {
	// Keep serial because SetDefaultPool changes the package-level pool.
	SetDefaultPool(&pgxpool.Pool{})
	t.Cleanup(func() { SetDefaultPool(nil) })
	conn := stubQuerier{queryRow: func(context.Context, string, ...any) pgx.Row {
		return scanRow(func(dest ...any) error {
			*dest[0].(*bool) = true
			return nil
		})
	}}
	if exists, err := Select("1").Exists(t.Context(), conn); err != nil || !exists {
		t.Fatalf("Exists() = (%v, %v), want (true, nil)", exists, err)
	}
}

func TestSelectExistsValidation(t *testing.T) {
	t.Parallel()
	var nilPool *pgxpool.Pool
	for _, tt := range []struct {
		name string
		stmt *SelectStmt
		conn []Querier
		err  string
	}{
		{
			name: "build error before connection resolution",
			stmt: Select().From("t").Apply(Where("")),
			conn: []Querier{nil},
			err:  "pql: WHERE condition is required",
		},
		{
			name: "negative limit",
			stmt: Select().From("t").Limit(-1),
			err:  "pql: LIMIT must not be negative",
		},
		{
			name: "negative offset",
			stmt: Select().From("t").Offset(-1),
			err:  "pql: OFFSET must not be negative",
		},
		{
			name: "nil connection",
			stmt: Select("1"),
			conn: []Querier{nil},
			err:  "pql: connection is nil",
		},
		{
			name: "typed nil connection",
			stmt: Select("1"),
			conn: []Querier{nilPool},
			err:  "pql: connection is nil",
		},
		{
			name: "multiple connections",
			stmt: Select("1"),
			conn: []Querier{nil, nil},
			err:  "pql: expected at most one connection, got 2",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			exists, err := tt.stmt.Exists(t.Context(), tt.conn...)
			if exists || err == nil || err.Error() != tt.err {
				t.Fatalf("Exists() = (%v, %v), want (false, %q)", exists, err, tt.err)
			}
		})
	}
}

type stubQuerier struct {
	exec     func(context.Context, string, ...any) (pgconn.CommandTag, error)
	query    func(context.Context, string, ...any) (pgx.Rows, error)
	queryRow func(context.Context, string, ...any) pgx.Row
}

func (q stubQuerier) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	return q.exec(ctx, query, args...)
}

func (q stubQuerier) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	return q.query(ctx, query, args...)
}

func (q stubQuerier) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	return q.queryRow(ctx, query, args...)
}

type stubRows struct{ pgx.Rows }

type scanRow func(...any) error

func (r scanRow) Scan(dest ...any) error { return r(dest...) }
