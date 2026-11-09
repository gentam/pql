package pql

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// opaqueCursor is a cursor value whose nullness pql cannot determine.
type opaqueCursor struct{ v int }

type failingValuer struct{}

func (failingValuer) Value() (driver.Value, error) { return nil, errors.New("fail") }

func TestCursorNullness(t *testing.T) {
	t.Parallel()
	n, s := 1, "x"
	var nilInt *int
	var nilDate *pgtype.Date
	var nilBytes []byte
	var nilMap map[string]any
	for _, tt := range []struct {
		name  string
		value any
		want  nullness
	}{
		{"nil", nil, nullYes},
		{"nil pointer", nilInt, nullYes},
		{"nil pointer to valuer", nilDate, nullYes},
		{"nil slice", nilBytes, nullYes},
		{"nil map", nilMap, nullYes},
		{"null valuer", pgtype.Int4{}, nullYes},
		{"null sql valuer", sql.NullString{}, nullYes},
		{"pointer to null valuer", &pgtype.Date{}, nullYes},
		{"int", 1, nullNo},
		{"named string", ItemStatus("open"), nullNo},
		{"bool", false, nullNo},
		{"float", 1.5, nullNo},
		{"pointer", &n, nullNo},
		{"pointer to pointer", &[]*string{&s}[0], nullNo},
		{"time", time.Time{}, nullNo},
		{"uuid array", [16]byte{}, nullNo},
		{"bytes", []byte{}, nullNo},
		{"valid valuer", pgtype.Int4{Int32: 1, Valid: true}, nullNo},
		{"struct", opaqueCursor{1}, nullUnknown},
		{"failing valuer", failingValuer{}, nullUnknown},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := cursorNullness(tt.value); got != tt.want {
				t.Fatalf("cursorNullness(%#v) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}

// ItemStatus is a named string type, as applications often define.
type ItemStatus string

func TestKeysetBuild(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		w    WhereExpr
		want buildResult
	}{
		{
			name: "after ascending",
			w:    After([]OrderTerm{Asc("id")}, 10),
			want: buildResult{query: "SELECT * FROM t WHERE (((id) >= $1 OR (id) IS NULL) AND (id) IS DISTINCT FROM $1)", args: []any{10}},
		},
		{
			name: "before descending",
			w:    Before([]OrderTerm{Desc("id")}, 10),
			want: buildResult{query: "SELECT * FROM t WHERE (((id) >= $1 OR (id) IS NULL) AND (id) IS DISTINCT FROM $1)", args: []any{10}},
		},
		{
			name: "after descending",
			w:    After([]OrderTerm{Desc("id")}, 10),
			want: buildResult{query: "SELECT * FROM t WHERE (((id) <= $1 AND (id) IS NOT NULL) AND (id) IS DISTINCT FROM $1)", args: []any{10}},
		},
		{
			name: "before ascending",
			w:    Before([]OrderTerm{Asc("id")}, 10),
			want: buildResult{query: "SELECT * FROM t WHERE (((id) <= $1 AND (id) IS NOT NULL) AND (id) IS DISTINCT FROM $1)", args: []any{10}},
		},
		{
			name: "inclusive nulls first",
			w:    AtOrAfter([]OrderTerm{Asc("score", NullsFirst)}, nil),
			want: buildResult{query: "SELECT * FROM t WHERE ((TRUE))"},
		},
		{
			name: "inclusive before nulls last",
			w:    AtOrBefore([]OrderTerm{Desc("score", NullsLast)}, nil),
			want: buildResult{query: "SELECT * FROM t WHERE ((TRUE))"},
		},
		{
			name: "mixed directions and null placement",
			w:    After([]OrderTerm{Desc("score", NullsLast), Asc("id")}, nil, 10),
			want: buildResult{
				query: "SELECT * FROM t WHERE (((score) IS NULL) AND ((score) IS NOT NULL OR (((id) >= $1 OR (id) IS NULL) AND (id) IS DISTINCT FROM $1)))",
				args:  []any{10},
			},
		},
		{
			name: "descending composite boundary",
			w:    After([]OrderTerm{Desc("score"), Desc("id")}, 5, 10),
			want: buildResult{
				query: "SELECT * FROM t WHERE ((ROW((score),(id)) <= ROW($1,$2)) AND ((score) <= $1 AND (score) IS NOT NULL) AND ((score) IS DISTINCT FROM $1 OR (((id) <= $2 AND (id) IS NOT NULL) AND (id) IS DISTINCT FROM $2)))",
				args:  []any{5, 10},
			},
		},
		{
			name: "inclusive ascending composite boundary with null cursor",
			w:    AtOrBefore([]OrderTerm{Asc("score"), Asc("id")}, nil, 10),
			want: buildResult{
				query: "SELECT * FROM t WHERE ((TRUE) AND ((score) IS NOT NULL OR (((id) <= $1 AND (id) IS NOT NULL))))",
				args:  []any{10},
			},
		},
		{
			name: "expressions and literal question marks",
			w:    After([]OrderTerm{Asc("payload ? 'key'")}, false),
			want: buildResult{query: "SELECT * FROM t WHERE (((payload ? 'key') >= $1 OR (payload ? 'key') IS NULL) AND (payload ? 'key') IS DISTINCT FROM $1)", args: []any{false}},
		},
		{
			name: "unknown nullness keeps runtime checks",
			w:    After([]OrderTerm{Asc("id")}, opaqueCursor{10}),
			want: buildResult{query: "SELECT * FROM t WHERE (((id) >= $1 AND $1 IS NOT NULL OR (id) IS NULL) AND (id) IS DISTINCT FROM $1)", args: []any{opaqueCursor{10}}},
		},
		{
			name: "unknown nullness in composite boundary",
			w:    After([]OrderTerm{Desc("score"), Desc("id")}, opaqueCursor{5}, 10),
			want: buildResult{
				query: "SELECT * FROM t WHERE ((ROW((score),(id)) <= ROW($1,$2) OR $1 IS NULL) AND ((score) <= $1 AND (score) IS NOT NULL OR $1 IS NULL) AND ((score) IS DISTINCT FROM $1 OR (((id) <= $2 AND (id) IS NOT NULL) AND (id) IS DISTINCT FROM $2)))",
				args:  []any{opaqueCursor{5}, 10},
			},
		},
		{
			name: "null cursor truncates composite boundary",
			w:    After([]OrderTerm{Desc("a"), Desc("b"), Desc("c")}, 1, 2, nil),
			want: buildResult{
				query: "SELECT * FROM t WHERE ((ROW((a),(b)) <= ROW($1,$2)) AND ((a) <= $1 AND (a) IS NOT NULL) AND ((a) IS DISTINCT FROM $1 OR (((b) <= $2 AND (b) IS NOT NULL) AND ((b) IS DISTINCT FROM $2 OR ((TRUE) AND (c) IS NOT NULL)))))",
				args:  []any{1, 2},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assertBuild(t, Select().From("t").Apply(tt.w), tt.want)
			assertBuild(t, Select().From("t").Apply(tt.w).Clone(), tt.want)
		})
	}
}

func TestKeysetValidation(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name   string
		orders []OrderTerm
		values []any
		err    string
	}{
		{name: "no orders", err: "pql: keyset requires at least one order"},
		{name: "no values", orders: []OrderTerm{Asc("a")}, err: "pql: keyset order count 1 does not match value count 0"},
		{name: "too few values", orders: []OrderTerm{Asc("a"), Asc("b")}, values: []any{1}, err: "pql: keyset order count 2 does not match value count 1"},
		{name: "too many values", orders: []OrderTerm{Asc("a")}, values: []any{1, 2}, err: "pql: keyset order count 1 does not match value count 2"},
		{name: "empty expression", orders: []OrderTerm{{}}, values: []any{1}, err: "pql: ORDER BY expression is required"},
		{name: "blank expression", orders: []OrderTerm{Asc("a"), Desc("\t ")}, values: []any{1, 2}, err: "pql: ORDER BY expression is required"},
		{name: "invalid null option", orders: []OrderTerm{Asc("a", NullsOrder(99))}, values: []any{nil}, err: "pql: invalid NULLS order 99"},
		{name: "invalid final null option", orders: []OrderTerm{Desc("a", NullsFirst, NullsOrder(99))}, values: []any{nil}, err: "pql: invalid NULLS order 99"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, makeCondition := range []func([]OrderTerm, ...any) WhereExpr{After, Before, AtOrAfter, AtOrBefore} {
				w := makeCondition(tt.orders, tt.values...)
				assertBuild(t, Select().From("t").Apply(w), buildResult{err: tt.err})
				assertBuild(t, Select().From("t").Apply(w).Clone(), buildResult{err: tt.err})
			}
		})
	}
}

func TestKeysetComposition(t *testing.T) {
	t.Parallel()
	w := And(After([]OrderTerm{Asc("id")}, 10), Where("active=$", true))
	for _, tt := range []struct {
		stmt Builder
		want buildResult
	}{
		{
			stmt: Select().From("t").Apply(w),
			want: buildResult{query: "SELECT * FROM t WHERE ((((id) >= $1 OR (id) IS NULL) AND (id) IS DISTINCT FROM $1) AND (active=$2))", args: []any{10, true}},
		},
		{
			stmt: Update("t").Set("status", "OPEN").Apply(w),
			want: buildResult{query: "UPDATE t SET status=$1 WHERE ((((id) >= $2 OR (id) IS NULL) AND (id) IS DISTINCT FROM $2) AND (active=$3))", args: []any{"OPEN", 10, true}},
		},
		{
			stmt: Delete("t").Apply(w),
			want: buildResult{query: "DELETE FROM t WHERE ((((id) >= $1 OR (id) IS NULL) AND (id) IS DISTINCT FROM $1) AND (active=$2))", args: []any{10, true}},
		},
	} {
		assertBuild(t, tt.stmt, tt.want)
	}
	assertBuild(t, Select().Apply(Not(AtOrAfter([]OrderTerm{Asc("id")}, 10))), buildResult{
		query: "SELECT * WHERE (NOT (((id) >= $1 OR (id) IS NULL)))",
		args:  []any{10},
	})
	assertBuild(t, Update("t").Set("active", true).Apply(After([]OrderTerm{Desc("score"), Desc("id")}, 5, 10)), buildResult{
		query: "UPDATE t SET active=$1 WHERE ((ROW((score),(id)) <= ROW($2,$3)) AND ((score) <= $2 AND (score) IS NOT NULL) AND ((score) IS DISTINCT FROM $2 OR (((id) <= $3 AND (id) IS NOT NULL) AND (id) IS DISTINCT FROM $3)))",
		args:  []any{true, 5, 10},
	})
}

func TestKeysetSnapshotAndClone(t *testing.T) {
	t.Parallel()
	orders := []OrderTerm{Desc("score", NullsLast), Asc("id")}
	values := []any{nil, 10}
	w := After(orders, values...)
	source := Select().From("t").Apply(w)
	cloned := source.Clone()
	query, args, err := source.Build()
	if err != nil {
		t.Fatal(err)
	}
	want := buildResult{query: query, args: args}
	orders[0] = Asc("other")
	values[0] = 99
	assertBuild(t, source, want)
	source.Apply(Where("active=$", true))
	assertBuild(t, cloned, want)
	assertBuild(t, Select().From("t").Apply(w), want)
	for range 8 {
		t.Run("concurrent clone", func(t *testing.T) {
			t.Parallel()
			assertBuild(t, cloned.Clone(), want)
		})
	}
}
