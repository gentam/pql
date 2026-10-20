package pql

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestWherePlaceholdersPostgres(t *testing.T) {
	t.Parallel()
	url := os.Getenv("PQL_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set PQL_TEST_DATABASE_URL to test placeholders against PostgreSQL")
	}
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["standard_conforming_strings"] = "on"
	db, err := pgxpool.NewWithConfig(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	for _, tt := range []struct {
		name string
		from string
		expr string
		arg  any
	}{
		{"ordinary string", "", `'$' = $`, "$"},
		{"ordinary backslash", "", `'\' = $`, `\`},
		{"escaped quote", "", `E'it\'s $' = $`, "it's $"},
		{"escaped backslash", "", `E'\\$' = $`, `\$`},
		{"escape continuation", "", "E'a'\n" + `'\'$' = $`, "a'$"},
		{"escape continuation after comment", "", "E'a' -- $\r\n" + `'\'$' = $`, "a'$"},
		{"unicode string", "", `U&'price\0024' = $`, "price$"},
		{"custom unicode escape", "", `U&'price$0024' UESCAPE '$' = $`, "price$"},
		{"dollar quote", "", `$$price $1$$ = $`, "price $1"},
		{"tagged dollar quote", "", `$tag$price $$ $tag$ = $`, "price $$ "},
		{"non-ASCII tag", "", `$引用$価格$引用$ = $`, "価格"},
		{"quoted identifier", `(VALUES (42)) AS t("price$")`, `"price$" = $`, 42},
		{"unquoted identifier", `(VALUES (42)) AS t(price$usd)`, `price$usd = $`, 42},
		{"unicode identifier", `(VALUES (42)) AS t("price$")`, `U&"price\0024" = $`, 42},
		{"non-ASCII identifier", `(VALUES (42)) AS t(価格$円)`, `価格$円 = $`, 42},
		{"comments", "", "/* $ /* $ */ $ */ $::boolean -- $", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stmt := Select("true").Apply(Where(tt.expr, tt.arg))
			if tt.from != "" {
				stmt.From(tt.from)
			}
			var matched bool
			if err := stmt.QueryRow(t.Context(), db).Scan(&matched); err != nil || !matched {
				t.Fatalf("Scan() = %v, matched = %v; want true", err, matched)
			}
		})
	}
}

func TestTrailingLineCommentsPostgres(t *testing.T) {
	t.Parallel()
	url := os.Getenv("PQL_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set PQL_TEST_DATABASE_URL to test trailing line comments against PostgreSQL")
	}
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `
			CREATE TEMP TABLE pql_comments (id integer PRIMARY KEY, n integer NOT NULL);
			INSERT INTO pql_comments VALUES (1, 10), (2, 10), (3, 20)`)
		return err
	}
	db, err := pgxpool.NewWithConfig(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	for _, tt := range []struct {
		name  string
		build func() queryStatement
		want  []int
	}{
		{
			name: "select",
			build: func() queryStatement {
				return Select("id -- result").From("pql_comments -- table").
					Apply(And(Where("n=$ -- filter", 10), Where("id>$ -- comparison", 0))).
					Desc("id -- order").Limit(1)
			},
			want: []int{2},
		},
		{
			name: "negation and raw conditions",
			build: func() queryStatement {
				s := Select("id").From("pql_comments").Asc("id")
				s.Apply(And(Not(Where("n=20 -- filter")), Or(Where("id=1 -- first"), Where("id=2 -- last"))))
				return s
			},
			want: []int{1, 2},
		},
		{
			name: "insert",
			build: func() queryStatement {
				return Insert("pql_comments -- table").Set("id -- column", 4).Set("n", 10).Returning("id -- result")
			},
			want: []int{4},
		},
		{
			name: "update",
			build: func() queryStatement {
				return Update("pql_comments -- table").Set("n -- column", 30).
					Apply(Where("id=$ -- filter", 2)).Returning("id -- result")
			},
			want: []int{2},
		},
		{
			name: "delete",
			build: func() queryStatement {
				return Delete("pql_comments -- table").Apply(Where("id=$ -- column", 3)).Returning("id -- result")
			},
			want: []int{3},
		},
		{
			name: "keyset",
			build: func() queryStatement {
				orders := []OrderTerm{Desc("n -- score"), Desc("id -- tie")}
				return Select("id").From("pql_comments").SetOrders(orders...).Apply(After(orders, 20, 3))
			},
			want: []int{2, 1},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tx, err := db.Begin(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			rows, err := tt.build().Query(t.Context(), tx)
			if err != nil {
				t.Fatal(err)
			}
			ids, err := pgx.CollectRows(rows, pgx.RowTo[int])
			if err != nil || !slices.Equal(ids, tt.want) {
				t.Fatalf("returned IDs = %v, error = %v; want %v", ids, err, tt.want)
			}
		})
	}
	t.Run("exists", func(t *testing.T) {
		t.Parallel()
		exists, err := Select("1 -- result").Apply(Where("true -- filter")).Exists(t.Context(), db)
		if err != nil || !exists {
			t.Fatalf("Exists() = (%v, %v), want (true, nil)", exists, err)
		}
	})
}

func TestStatementExecutionPostgres(t *testing.T) {
	t.Parallel()
	url := os.Getenv("PQL_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set PQL_TEST_DATABASE_URL to test statement execution against PostgreSQL")
	}
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `
			CREATE TEMP TABLE pql_execution (id integer PRIMARY KEY, n integer NOT NULL);
			INSERT INTO pql_execution VALUES (1, 0), (2, 0)`)
		return err
	}
	db, err := pgxpool.NewWithConfig(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	for _, tt := range []struct {
		name      string
		build     func() queryStatement
		ids       []int
		tag       string
		remaining int
		changed   int
	}{
		{
			name:      "select",
			build:     func() queryStatement { return Select("id").From("pql_execution").Asc("id") },
			ids:       []int{1, 2},
			remaining: 2,
		},
		{
			name:      "insert",
			build:     func() queryStatement { return Insert("pql_execution").Set("id", 3).Set("n", 7).Returning("id") },
			ids:       []int{3},
			tag:       "INSERT 0 1",
			remaining: 3,
			changed:   1,
		},
		{
			name:      "update",
			build:     func() queryStatement { return Update("pql_execution").Set("n", 7).Returning("id") },
			ids:       []int{1, 2},
			tag:       "UPDATE 2",
			remaining: 2,
			changed:   2,
		},
		{
			name:  "delete",
			build: func() queryStatement { return Delete("pql_execution").Returning("id") },
			ids:   []int{1, 2},
			tag:   "DELETE 2",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			conn, err := db.Acquire(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(conn.Release)
			// Keep subtests serial: they share this connection. Roll back each write.
			for _, method := range []string{"Query", "QueryRow", "Exec", "QueryRow error", "Scan error"} {
				if method == "Exec" && tt.name == "select" {
					continue
				}
				t.Run(method, func(t *testing.T) {
					ctx := t.Context()
					tx, err := conn.Begin(ctx)
					if err != nil {
						t.Fatal(err)
					}
					defer tx.Rollback(context.Background())
					stmt := tt.build()
					switch method {
					case "Query":
						rows, err := stmt.Query(ctx, tx)
						if err != nil {
							t.Fatal(err)
						}
						ids, err := pgx.CollectRows(rows, pgx.RowTo[int])
						if err != nil {
							t.Fatal(err)
						}
						slices.Sort(ids)
						if !slices.Equal(ids, tt.ids) {
							t.Fatalf("returned IDs = %v, want %v", ids, tt.ids)
						}
					case "QueryRow":
						var id int
						if err := stmt.QueryRow(ctx, tx).Scan(&id); err != nil || !slices.Contains(tt.ids, id) {
							t.Fatalf("Scan() = %v, id = %d; want an ID from %v", err, id, tt.ids)
						}
						if tt.name == "select" && id != tt.ids[0] {
							t.Fatalf("QueryRow returned ID %d, want first ID %d", id, tt.ids[0])
						}
					case "Exec":
						tag, err := stmt.(execStatement).Exec(ctx, tx)
						if err != nil || tag.String() != tt.tag || tag.RowsAffected() != int64(len(tt.ids)) {
							t.Fatalf("Exec() = (%v, %v), want (%s, nil)", tag, err, tt.tag)
						}
					case "QueryRow error":
						switch s := stmt.(type) {
						case *SelectStmt:
							s.Where("false")
						case *UpdateStmt:
							s.Where("false")
						case *DeleteStmt:
							s.Where("false")
						case *InsertStmt:
							s.Set("id", 1) // Duplicate primary key: the query error is reported by Scan.
						}
						var id int
						err := stmt.QueryRow(ctx, tx).Scan(&id)
						if tt.name == "insert" {
							var pgErr *pgconn.PgError
							if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
								t.Fatalf("Scan() = %v, want unique violation", err)
							}
						} else if !errors.Is(err, pgx.ErrNoRows) {
							t.Fatalf("Scan() = %v, want %v", err, pgx.ErrNoRows)
						}
						return
					case "Scan error":
						var a, b int
						if err := stmt.QueryRow(ctx, tx).Scan(&a, &b); err == nil {
							t.Fatal("Scan succeeded with too many destinations")
						}
					}
					// QueryRow reads one row but must still update/delete all matching rows.
					// QueryRow and CollectRows must also leave the connection ready for reuse.
					var remaining, changed int
					err = tx.QueryRow(ctx, "SELECT count(*), count(*) FILTER (WHERE n=7) FROM pql_execution").Scan(&remaining, &changed)
					if err != nil || remaining != tt.remaining || changed != tt.changed {
						t.Fatalf("stored rows = (%d, %d), error = %v; want (%d, %d)", remaining, changed, err, tt.remaining, tt.changed)
					}
				})
			}
		})
	}
}

func TestWhereCompositionPostgres(t *testing.T) {
	t.Parallel()
	url := os.Getenv("PQL_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set PQL_TEST_DATABASE_URL to test condition composition against PostgreSQL")
	}
	conn, err := pgx.Connect(t.Context(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close(context.Background()) })
	// Rows distinguish both grouping choices and SQL's three-valued logic.
	const from = `(VALUES
		(1, true, false, true),
		(2, false, true, true),
		(3, true, true, false),
		(4, NULL::boolean, false, true),
		(5, true, NULL::boolean, false),
		(6, false, false, false)
	) AS t(id,a,b,c)`
	a, b, c := Where("a"), Where("b"), Where("c")
	for _, tt := range []struct {
		name string
		w    WhereExpr
		want []int
	}{
		{"and with or", And(a, Or(b, c)), []int{1, 3}},
		{"or with and", Or(And(a, b), c), []int{1, 2, 3, 4}},
		{"not entire or", Not(Or(a, b)), []int{6}},
		{"not entire and", Not(And(a, b)), []int{1, 2, 4, 6}},
		{"not null remains unknown", Not(Where("$::boolean", nil)), nil},
		{"empty and", And(), []int{1, 2, 3, 4, 5, 6}},
		{"empty or", Or(), nil},
		{"negated empty or", Not(Or()), []int{1, 2, 3, 4, 5, 6}},
		{"bound values and comments", And(Where("id>$ -- lower", 1), Not(Where("id>$ -- upper", 3))), []int{2, 3}},
		{"keyset in or", Or(Before([]OrderTerm{Asc("id")}, 2), After([]OrderTerm{Asc("id")}, 5)), []int{1, 6}},
		{"negated keyset group", Not(Or(Before([]OrderTerm{Asc("id")}, 2), After([]OrderTerm{Asc("id")}, 5))), []int{2, 3, 4, 5}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rows, err := Select("id").From(from).Apply(tt.w).Asc("id").Query(t.Context(), conn)
			if err != nil {
				t.Fatal(err)
			}
			got, err := pgx.CollectRows(rows, pgx.RowTo[int])
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("matched %v, want %v", got, tt.want)
			}
		})
	}
}
