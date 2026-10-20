package pql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type keysetRecord struct {
	ID int
	A  *int
	B  *int
}

// This optional test uses only temporary tables on pooled connections.
func TestKeysetPostgres(t *testing.T) {
	t.Parallel()
	url := os.Getenv("PQL_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set PQL_TEST_DATABASE_URL to test keysets against PostgreSQL")
	}
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	// Each connection owns an isolated fixture. Page tests roll back cursor deletions.
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `
			CREATE TEMP TABLE pql_keyset (id integer PRIMARY KEY, a integer, b integer);
			INSERT INTO pql_keyset
			SELECT row_number() OVER () * 2, a, b
			FROM unnest(ARRAY[NULL, -1, 1]) AS a
			CROSS JOIN unnest(ARRAY[NULL, -1, 1]) AS b
			CROSS JOIN generate_series(1, 2)`)
		return err
	}
	db, err := pgxpool.NewWithConfig(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	boundaries := []struct {
		name string
		make func([]OrderTerm, ...any) WhereExpr
		want func([]int, int) []int
	}{
		{"after", After, func(ids []int, i int) []int { return ids[i+1:] }},
		{"before", Before, func(ids []int, i int) []int { return ids[:i] }},
		{"at or after", AtOrAfter, func(ids []int, i int) []int { return ids[i:] }},
		{"at or before", AtOrBefore, func(ids []int, i int) []int { return ids[:i+1] }},
	}
	options := []struct {
		name string
		desc bool
		null []NullsOrder
	}{
		{"asc default", false, nil}, {"desc default", true, nil},
		{"asc first", false, []NullsOrder{NullsFirst}}, {"asc last", false, []NullsOrder{NullsLast}},
		{"desc first", true, []NullsOrder{NullsFirst}}, {"desc last", true, []NullsOrder{NullsLast}},
	}
	// PostgreSQL's actual ORDER BY results are the oracle, independent of the
	// predicate implementation. Duplicate (a, b) pairs exercise the tie-breaker.
	for _, a := range options {
		for _, b := range options {
			for _, descID := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/id desc %t", a.name, b.name, descID), func(t *testing.T) {
					t.Parallel()
					ctx := t.Context()
					conn := acquireKeysetConn(t, db)
					orders := []OrderTerm{Order("a", a.desc, a.null...), Order("b", b.desc, b.null...), Order("id", descID)}
					base := Select("id", "a", "b").From("pql_keyset").SetOrders(orders...)
					rows, err := base.Query(ctx, conn)
					if err != nil {
						t.Fatal(err)
					}
					records, err := pgx.CollectRows(rows, pgx.RowToStructByName[keysetRecord])
					if err != nil {
						t.Fatal(err)
					}
					ids := make([]int, len(records))
					for i, r := range records {
						ids[i] = r.ID
					}
					batch := &pgx.Batch{}
					for i, r := range records {
						for _, boundary := range boundaries {
							for _, complement := range []bool{false, true} {
								w := boundary.make(orders, r.A, r.B, r.ID)
								want := boundary.want(ids, i)
								if complement {
									w = Not(w)
									matched := want
									want = nil
									for _, id := range ids {
										if !slices.Contains(matched, id) {
											want = append(want, id)
										}
									}
								}
								query, args, err := Select("id").From("pql_keyset").SetOrders(orders...).Apply(w).Build()
								if err != nil {
									t.Fatal(err)
								}
								batch.Queue(query, args...).Query(func(rows pgx.Rows) error {
									got, err := pgx.CollectRows(rows, pgx.RowTo[int])
									if err == nil && !slices.Equal(got, want) {
										t.Errorf("%s cursor id %d complement %t: got %v, want %v", boundary.name, r.ID, complement, got, want)
									}
									return err
								})
							}
						}
					}
					if err := conn.SendBatch(ctx, batch).Close(); err != nil {
						t.Fatal(err)
					}
					// Keep page subtests serial: they share this connection and transaction state.
					for _, backwards := range []bool{false, true} {
						for _, size := range []int{1, 4, 7, 18, 20} {
							t.Run(fmt.Sprintf("pages/backwards %t/size %d", backwards, size), func(t *testing.T) {
								testKeysetPages(t, conn, orders, records, size, backwards, false)
							})
						}
						t.Run(fmt.Sprintf("pages/backwards %t/deleted cursors", backwards), func(t *testing.T) {
							testKeysetPages(t, conn, orders, records, 4, backwards, true)
						})
					}
				})
			}
		}
	}

	t.Run("equal and null boundary groups", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		conn := acquireKeysetConn(t, db)
		for _, option := range options {
			order := Order("a", option.desc, option.null...)
			rows, err := Select("a", "array_agg(id ORDER BY id) AS ids").
				From("pql_keyset GROUP BY a").SetOrders(order).Query(ctx, conn)
			if err != nil {
				t.Fatal(err)
			}
			type group struct {
				A   *int
				IDs []int
			}
			groups, err := pgx.CollectRows(rows, pgx.RowToStructByName[group])
			if err != nil {
				t.Fatal(err)
			}
			for i, g := range groups {
				for _, boundary := range boundaries {
					for _, complement := range []bool{false, true} {
						w := boundary.make([]OrderTerm{order}, g.A)
						if complement {
							w = Not(w)
						}
						rows, err := Select("id").From("pql_keyset").SetOrders(order, Asc("id")).Apply(w).Query(ctx, conn)
						if err != nil {
							t.Fatal(err)
						}
						got, err := pgx.CollectRows(rows, pgx.RowTo[int])
						if err != nil {
							t.Fatal(err)
						}
						matched := boundary.want([]int{0, 1, 2}, i)
						var want []int
						for j, group := range groups {
							if slices.Contains(matched, j) != complement {
								want = append(want, group.IDs...)
							}
						}
						if !slices.Equal(got, want) {
							t.Errorf("%s/%s group %d complement %t: got %v, want %v", option.name, boundary.name, i, complement, got, want)
						}
					}
				}
			}
		}
	})

	t.Run("null encodings and absent boundaries", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		conn := acquireKeysetConn(t, db)
		orders := []OrderTerm{Asc("a", NullsFirst), Asc("id", NullsFirst)}
		var nilInt *int
		for _, null := range []any{nil, nilInt, pgtype.Int4{}, sql.NullInt64{}} {
			stmt := Select("count(*)").From("pql_keyset").Apply(After(orders, null, 0))
			var got int
			if err := stmt.QueryRow(ctx, conn).Scan(&got); err != nil || got != 18 {
				t.Fatalf("null %T: count %d, error %v; want 18, nil", null, got, err)
			}
		}
		for _, value := range []int{0, 100} {
			for _, boundary := range boundaries {
				stmt := Select("id").From("pql_keyset").Asc("id").Apply(boundary.make([]OrderTerm{Asc("id")}, value))
				rows, err := stmt.Query(ctx, conn)
				if err != nil {
					t.Fatal(err)
				}
				got, err := pgx.CollectRows(rows, pgx.RowTo[int])
				if err != nil {
					t.Fatal(err)
				}
				want := 0
				if (value == 0) == (boundary.name == "after" || boundary.name == "at or after") {
					want = 18
				}
				if len(got) != want {
					t.Errorf("%s absent cursor %d: got %d rows, want %d", boundary.name, value, len(got), want)
				}
			}
		}
	})

	t.Run("two bounds and existing arguments", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		conn := acquireKeysetConn(t, db)
		orders := []OrderTerm{Asc("id")}
		stmt := Select("count(*)").From("pql_keyset").
			Apply(Where("id % $ = $", 2, 0)).
			Apply(After(orders, 10)).Apply(Before(orders, 20))
		var count int
		if err := stmt.QueryRow(ctx, conn).Scan(&count); err != nil || count != 4 {
			t.Fatalf("count %d, error %v; want 4, nil", count, err)
		}
	})

	t.Run("uuid timestamp and expressions", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		conn := acquireKeysetConn(t, db)
		from := `(VALUES
			('00000000-0000-0000-0000-000000000001'::uuid, '2026-01-01Z'::timestamptz, '{"key": true}'::jsonb),
			('00000000-0000-0000-0000-000000000002'::uuid, '2026-01-01Z'::timestamptz, '{}'::jsonb)
		) AS t(id, created_at, payload)`
		orders := []OrderTerm{Desc("created_at"), Desc("id")}
		stmt := Select("id::text").From(from).SetOrders(orders...).Apply(After(orders,
			time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), "00000000-0000-0000-0000-000000000002"))
		var id string
		if err := stmt.QueryRow(ctx, conn).Scan(&id); err != nil || id != "00000000-0000-0000-0000-000000000001" {
			t.Fatalf("id %q, error %v", id, err)
		}
		exists, err := Select("1").From(from).Apply(After([]OrderTerm{Asc("payload ? 'key'")}, false)).Exists(ctx, conn)
		if err != nil || !exists {
			t.Fatalf("JSON expression Exists() = (%t, %v), want (true, nil)", exists, err)
		}
		orders = []OrderTerm{Asc("COALESCE(a, b) + id"), Desc("id")}
		exists, err = Select("1").From("pql_keyset").Apply(AtOrAfter(orders, nil, 100)).Exists(ctx, conn)
		if err != nil || !exists {
			t.Fatalf("arithmetic expression Exists() = (%t, %v), want (true, nil)", exists, err)
		}
	})

	t.Run("repeated leading key uses composite index boundary", func(t *testing.T) {
		t.Parallel()
		testKeysetIndexBoundary(t, acquireKeysetConn(t, db))
	})
}

func acquireKeysetConn(t *testing.T, db *pgxpool.Pool) *pgx.Conn {
	t.Helper()
	conn, err := db.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(conn.Release)
	return conn.Conn()
}

func testKeysetPages(t *testing.T, conn *pgx.Conn, orders []OrderTerm, records []keysetRecord, size int, backwards, deleteCursors bool) {
	t.Helper()
	ctx := t.Context()
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	fetchOrders := orders
	boundary := After
	if backwards {
		boundary = Before
		fetchOrders = ReversedOrders(orders)
	}
	base := Select("id", "a", "b").From("pql_keyset").SetOrders(fetchOrders...).Limit(size + 1)
	want := make([]int, len(records))
	for i, r := range records {
		want[i] = r.ID
	}
	var got []int
	var cursor *keysetRecord
	// One extra request verifies that traversal terminates at an empty page,
	// including when the last page was full or its cursor row was deleted.
	for request := 0; request <= len(records); request++ {
		stmt := base.Clone()
		if cursor != nil {
			stmt.Apply(boundary(orders, cursor.A, cursor.B, cursor.ID))
		}
		rows, err := stmt.Query(ctx, tx)
		if err != nil {
			t.Fatal(err)
		}
		page, err := pgx.CollectRows(rows, pgx.RowToStructByName[keysetRecord])
		if err != nil {
			t.Fatal(err)
		}
		more := len(page) > size
		if more {
			page = page[:size]
		}
		if backwards {
			slices.Reverse(page)
		}
		remaining := len(want) - len(got)
		if len(page) != min(size, remaining) || more != (remaining > size) {
			t.Fatalf("request %d: %d rows, more %t; remaining %d, size %d", request, len(page), more, remaining, size)
		}
		if len(page) == 0 {
			if !slices.Equal(got, want) {
				t.Fatalf("traversal got %v, want %v", got, want)
			}
			return
		}
		ids := make([]int, len(page))
		for i, r := range page {
			ids[i] = r.ID
		}
		start := len(got)
		if backwards {
			start = remaining - len(page)
			cursor = &page[0]
			got = append(ids, got...)
		} else {
			cursor = &page[len(page)-1]
			got = append(got, ids...)
		}
		if !slices.Equal(ids, want[start:start+len(page)]) {
			t.Fatalf("request %d: got %v, want %v", request, ids, want[start:start+len(page)])
		}
		if deleteCursors {
			if _, err := tx.Exec(ctx, "DELETE FROM pql_keyset WHERE id=$1", cursor.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Fatal("page traversal did not terminate")
}

func testKeysetIndexBoundary(t *testing.T, conn *pgx.Conn) {
	t.Helper()
	ctx := t.Context()
	_, err := conn.Exec(ctx, `
		CREATE TEMP TABLE pql_keyset_index (a integer, id integer PRIMARY KEY, b integer);
		INSERT INTO pql_keyset_index SELECT 1, i, 0 FROM generate_series(1, 100000) AS i;
		CREATE INDEX ON pql_keyset_index (a DESC, id DESC, b ASC);
		CREATE INDEX ON pql_keyset_index (a ASC NULLS FIRST, id ASC NULLS FIRST);
		ANALYZE pql_keyset_index`)
	if err != nil {
		t.Fatal(err)
	}
	// Keep subtests serial: they share this connection and its temporary table.
	for _, tt := range []struct {
		name   string
		orders []OrderTerm
		before bool
		cursor int
	}{
		{"descending after", []OrderTerm{Desc("a"), Desc("id")}, false, 1000},
		{"ascending before", []OrderTerm{Asc("a"), Asc("id")}, true, 1000},
		{"ascending after nulls first", []OrderTerm{Asc("a", NullsFirst), Asc("id", NullsFirst)}, false, 99000},
		{"descending before nulls last", []OrderTerm{Desc("a", NullsLast), Desc("id", NullsLast)}, true, 99000},
		{"same direction prefix", []OrderTerm{Desc("a"), Desc("id"), Asc("b")}, false, 1000},
	} {
		for _, inclusive := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/inclusive %t", tt.name, inclusive), func(t *testing.T) {
				boundary := After
				if tt.before {
					boundary = Before
				}
				if inclusive {
					boundary = AtOrAfter
					if tt.before {
						boundary = AtOrBefore
					}
				}
				values := []any{1, tt.cursor}
				if len(tt.orders) == 3 {
					values = append(values, 0)
				}
				fetchOrders := tt.orders
				if tt.before {
					fetchOrders = ReversedOrders(tt.orders)
				}
				stmt := Select("id").From("pql_keyset_index").SetOrders(fetchOrders...).
					Apply(boundary(tt.orders, values...)).Limit(20)
				rows, err := stmt.Query(ctx, conn)
				if err != nil {
					t.Fatal(err)
				}
				got, err := pgx.CollectRows(rows, pgx.RowTo[int])
				if err != nil {
					t.Fatal(err)
				}
				want := make([]int, 20)
				step := 1
				if fetchOrders[1].Descending() {
					step = -1
				}
				for i := range want {
					want[i] = tt.cursor + step*i
					if !inclusive {
						want[i] += step
					}
				}
				if !slices.Equal(got, want) {
					t.Fatalf("got %v, want %v", got, want)
				}
				query, args, err := stmt.Build()
				if err != nil {
					t.Fatal(err)
				}
				var raw []byte
				if err := conn.QueryRow(ctx, "EXPLAIN (ANALYZE, FORMAT JSON, TIMING OFF) "+query, args...).Scan(&raw); err != nil {
					t.Fatal(err)
				}
				type plan struct {
					NodeType string  `json:"Node Type"`
					Rows     float64 `json:"Actual Rows"`
					Loops    float64 `json:"Actual Loops"`
					Removed  float64 `json:"Rows Removed by Filter"`
					Plans    []plan
				}
				var explained []struct{ Plan plan }
				if err := json.Unmarshal(raw, &explained); err != nil || len(explained) != 1 {
					t.Fatalf("invalid plan %s: %v", raw, err)
				}
				// Bound the work, not elapsed time or a particular plan's spelling.
				// The old leading-key-only bound discarded roughly 99,000 rows.
				var check func(plan)
				check = func(p plan) {
					if (p.Rows+p.Removed)*p.Loops > 100 {
						t.Fatalf("%s processed too many rows: %s", p.NodeType, raw)
					}
					for _, child := range p.Plans {
						check(child)
					}
				}
				check(explained[0].Plan)
			})
		}
	}
}
