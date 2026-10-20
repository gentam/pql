package pql

import (
	"reflect"
	"strconv"
	"testing"
)

type buildResult struct {
	query string
	args  []any
	err   string
}

func assertBuild(t *testing.T, b Builder, want buildResult) {
	t.Helper()

	gotQuery, gotArgs, gotErr := b.Build()
	if gotQuery != want.query {
		t.Errorf("query mismatch\n got: %q\nwant: %q", gotQuery, want.query)
	}
	if !reflect.DeepEqual(gotArgs, want.args) {
		t.Errorf("args mismatch\n got: %#v\nwant: %#v", gotArgs, want.args)
	}
	if gotErr == nil {
		if want.err != "" {
			t.Errorf("error mismatch\n got: nil\nwant: %q", want.err)
		}
	} else if gotErr.Error() != want.err {
		t.Errorf("error mismatch\n got: %q\nwant: %q", gotErr, want.err)
	}
}

func TestBuildTrailingLineComments(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		stmt Builder
		want buildResult
	}{
		{
			name: "select",
			stmt: Select("a -- column", "b").From("t -- table").Asc("a -- order", NullsFirst).Desc("b -- order").Limit(1),
			want: buildResult{query: "SELECT a -- column\n,b FROM t -- table\n ORDER BY a -- order\n ASC NULLS FIRST,b -- order\n DESC LIMIT 1"},
		},
		{
			name: "insert",
			stmt: Insert("t -- table").Set("a -- column", 1).Set("b", 2).Returning("a -- result", "b -- result"),
			want: buildResult{
				query: "INSERT INTO t -- table\n (a -- column\n,b) VALUES ($1,$2) RETURNING a -- result\n,b -- result\n",
				args:  []any{1, 2},
			},
		},
		{
			name: "update",
			stmt: Update("t -- table").Set("a -- column", 1).Set("b", 2).Apply(Where("id=$ -- filter", 3)).Returning("a -- result", "b"),
			want: buildResult{
				query: "UPDATE t -- table\n SET a -- column\n=$1,b=$2 WHERE (id=$3 -- filter\n) RETURNING a -- result\n,b",
				args:  []any{1, 2, 3},
			},
		},
		{
			name: "delete",
			stmt: Delete("t -- table").Apply(Where("id=$ -- filter", 1)).Returning("id -- result", "a"),
			want: buildResult{
				query: "DELETE FROM t -- table\n WHERE (id=$1 -- filter\n) RETURNING id -- result\n,a",
				args:  []any{1},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertBuild(t, tt.stmt, tt.want)
		})
	}
}

func TestBuildError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		build func() Builder
		want  string
	}{
		{
			name:  "insert without table",
			build: func() Builder { return Insert("").Set("c", 1) },
			want:  "pql: INSERT table is required",
		},
		{
			name:  "insert without values",
			build: func() Builder { return Insert("t") },
			want:  "pql: INSERT values are required",
		},
		{
			name:  "update without table",
			build: func() Builder { return Update("").Set("c", 1) },
			want:  "pql: UPDATE table is required",
		},
		{
			name:  "update without values",
			build: func() Builder { return Update("t") },
			want:  "pql: UPDATE values are required",
		},
		{
			name:  "delete without table",
			build: func() Builder { return Delete("") },
			want:  "pql: DELETE table is required",
		},
		{
			name: "empty condition",
			build: func() Builder {
				s := Select().From("t")
				s.Where("")
				return s
			},
			want: "pql: WHERE condition is required",
		},
		{
			name: "empty negated select condition",
			build: func() Builder {
				s := Select().From("t")
				s.WhereNot("")
				return s
			},
			want: "pql: WHERE condition is required",
		},
		{
			name: "empty negated update condition",
			build: func() Builder {
				s := Update("t").Set("c", 1)
				s.WhereNot(" ")
				return s
			},
			want: "pql: WHERE condition is required",
		},
		{
			name: "empty negated delete condition",
			build: func() Builder {
				s := Delete("t")
				s.WhereNot("\t")
				return s
			},
			want: "pql: WHERE condition is required",
		},
		{
			name: "numbered placeholder collides with generated argument",
			build: func() Builder {
				s := Select("id").From("items")
				s.Where("tenant_id=$1")
				s.Where("id=$", 42)
				return s
			},
			want: "pql: numbered placeholders are not supported in WHERE expressions; use $",
		},
		{
			name: "numbered placeholder collides with update value",
			build: func() Builder {
				return Update("t").Set("c", 1).Apply(Where("id=$1"))
			},
			want: "pql: numbered placeholders are not supported in WHERE expressions; use $",
		},
		{
			name: "missing argument in composed condition",
			build: func() Builder {
				return Delete("t").Apply(And(Where("active=$", true), Where("id=$")))
			},
			want: "pql: WHERE placeholder count 1 does not match argument count 0",
		},
		{
			name: "missing argument in negated condition",
			build: func() Builder {
				s := Select().From("t")
				s.WhereNot("id=$")
				return s
			},
			want: "pql: WHERE placeholder count 1 does not match argument count 0",
		},
		{
			name: "missing argument inside SQL expression",
			build: func() Builder {
				return Select().From("t").Apply(Where("coalesce($, id) = 42"))
			},
			want: "pql: WHERE placeholder count 1 does not match argument count 0",
		},
		{
			name: "too few placeholders",
			build: func() Builder {
				s := Select().From("t")
				s.Where("a=$", 1, 2)
				return s
			},
			want: "pql: WHERE placeholder count 1 does not match argument count 2",
		},
		{
			name: "too many placeholders",
			build: func() Builder {
				s := Select().From("t")
				s.Where("a=$ AND b=$", 1)
				return s
			},
			want: "pql: WHERE placeholder count 2 does not match argument count 1",
		},
		{
			name: "question mark is not a placeholder",
			build: func() Builder {
				s := Select().From("t")
				s.Where("a=?", 1)
				return s
			},
			want: "pql: WHERE placeholder count 0 does not match argument count 1",
		},
		{
			name: "extra argument in raw condition",
			build: func() Builder {
				s := Select().From("t")
				s.Where("a", 1)
				return s
			},
			want: "pql: WHERE placeholder count 0 does not match argument count 1",
		},
		{
			name: "postfix expression argument without placeholder",
			build: func() Builder {
				s := Select().From("t")
				s.Where("a IS NULL", 1)
				return s
			},
			want: "pql: WHERE placeholder count 0 does not match argument count 1",
		},
		{
			name:  "empty order expression",
			build: func() Builder { return Select().From("t").Asc("") },
			want:  "pql: ORDER BY expression is required",
		},
		{
			name: "blank order expression after valid order",
			build: func() Builder {
				return Select().From("t").Apply(Where("active=$", true)).Asc("a").Desc(" \t\n")
			},
			want: "pql: ORDER BY expression is required",
		},
		{
			name:  "zero order term",
			build: func() Builder { return Select().From("t").SetOrders(Order{}) },
			want:  "pql: ORDER BY expression is required",
		},
		{
			name: "invalid null ordering",
			build: func() Builder {
				return Select().From("t").Asc("a", NullsOrder(0))
			},
			want: "pql: invalid NULLS order 0",
		},
		{
			name: "multiple null ordering options",
			build: func() Builder {
				return Select().From("t").Asc("a", NullsFirst, NullsLast)
			},
			want: "pql: ORDER BY accepts at most 1 NULLS option, got 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertBuild(t, tt.build(), buildResult{err: tt.want})
		})
	}
}

func TestBuildEmptyColumnExpressions(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		build func(string) Builder
		err   string
	}{
		{"select", func(col string) Builder { return Select(col) }, "pql: SELECT expression is required"},
		{"select append", func(col string) Builder { return Select("id").Select(col) }, "pql: SELECT expression is required"},
		{"insert set", func(col string) Builder { return Insert("t").Set("a", 1).Set(col, 2) }, "pql: INSERT column is required"},
		{"insert values", func(col string) Builder { return Insert("t").Values(Map{"a": 1, col: 2}) }, "pql: INSERT column is required"},
		{"update set", func(col string) Builder { return Update("t").Set("a", 1).Set(col, 2) }, "pql: UPDATE column is required"},
		{"update values", func(col string) Builder { return Update("t").Values(Map{"a": 1, col: 2}) }, "pql: UPDATE column is required"},
		{"insert returning", func(col string) Builder { return Insert("t").Set("a", 1).Returning("a", col) }, "pql: RETURNING expression is required"},
		{"update returning", func(col string) Builder { return Update("t").Set("a", 1).Returning("a").Returning(col) }, "pql: RETURNING expression is required"},
		{"delete returning", func(col string) Builder { return Delete("t").Apply(Where("a=$", 1)).Returning("a", col) }, "pql: RETURNING expression is required"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for _, col := range []string{"", " \t\r\n", "\u2003"} {
				t.Run(strconv.Quote(col), func(t *testing.T) {
					t.Parallel()
					assertBuild(t, tt.build(col), buildResult{err: tt.err})
				})
			}
		})
	}
}

func TestBuildEmptyReturningLists(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		stmt Builder
		want buildResult
	}{
		{"insert", Insert("t").Set("a", 1).Returning().Returning([]string{}...), buildResult{query: "INSERT INTO t (a) VALUES ($1)", args: []any{1}}},
		{"update", Update("t").Set("a", 1).Returning().Returning([]string{}...), buildResult{query: "UPDATE t SET a=$1", args: []any{1}}},
		{"delete", Delete("t").Returning().Returning([]string{}...), buildResult{query: "DELETE FROM t"}},
		{"insert retains returning", Insert("t").Set("a", 1).Returning("a").Returning(), buildResult{query: "INSERT INTO t (a) VALUES ($1) RETURNING a", args: []any{1}}},
		{"update retains returning", Update("t").Set("a", 1).Returning("a").Returning(), buildResult{query: "UPDATE t SET a=$1 RETURNING a", args: []any{1}}},
		{"delete retains returning", Delete("t").Returning("a").Returning(), buildResult{query: "DELETE FROM t RETURNING a"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertBuild(t, tt.stmt, tt.want)
		})
	}
}
