package pql

import "testing"

func TestDeleteBuild(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		build func() *DeleteStmt
		want  buildResult
	}{
		{name: "table only", build: func() *DeleteStmt { return Delete("t") }, want: buildResult{query: "DELETE FROM t"}},
		{
			name: "where and returning",
			build: func() *DeleteStmt {
				s := Delete("t")
				s.Where("c1=$", 1)
				return s.WhereNot("c2 IS NOT NULL").Returning("c1")
			},
			want: buildResult{
				query: "DELETE FROM t WHERE (c1=$1) AND (NOT (c2 IS NOT NULL)) RETURNING c1",
				args:  []any{1},
			},
		},
		{
			name: "applied and nested clauses",
			build: func() *DeleteStmt {
				w := Or(Where("c0<=now()"), Where("c0 IS NULL"), Where("c2=$", 2))
				s := Delete("t").Apply(w)
				s.WhereNot("c1=$", 1)
				return s
			},
			want: buildResult{
				query: "DELETE FROM t WHERE ((c0<=now()) OR (c0 IS NULL) OR (c2=$1)) AND (NOT (c1=$2))",
				args:  []any{2, 1},
			},
		},
		{
			name:  "zero applied clause",
			build: func() *DeleteStmt { return Delete("t").Apply(WhereExpr{}) },
			want:  buildResult{err: "pql: WHERE condition is required"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt := tt.build()
			assertBuild(t, stmt, tt.want)
			assertBuild(t, stmt.Clone(), tt.want)
		})
	}
}

func TestDeleteSetReturning(t *testing.T) {
	t.Parallel()
	base := Delete("t").Where("c=$", 1).Returning("old")
	cols := []string{"c", "c + 1 AS next"}
	stmt := base.Clone()
	if stmt.SetReturning(cols...) != stmt {
		t.Fatal("SetReturning must return its receiver")
	}
	cols[0] = "changed"
	stmt.Returning().Returning("c + 2")
	assertBuild(t, stmt, buildResult{
		query: "DELETE FROM t WHERE (c=$1) RETURNING c,c + 1 AS next,c + 2",
		args:  []any{1},
	})
	assertBuild(t, base, buildResult{query: "DELETE FROM t WHERE (c=$1) RETURNING old", args: []any{1}})
	assertBuild(t, stmt.SetReturning(), buildResult{query: "DELETE FROM t WHERE (c=$1)", args: []any{1}})
	assertBuild(t, stmt.Returning("c").SetReturning([]string(nil)...), buildResult{query: "DELETE FROM t WHERE (c=$1)", args: []any{1}})
	assertBuild(t, stmt.Returning("c").SetReturning([]string{}...), buildResult{query: "DELETE FROM t WHERE (c=$1)", args: []any{1}})
	for _, col := range []string{"", "  "} {
		assertBuild(t, stmt.SetReturning(col), buildResult{err: "pql: RETURNING expression is required"})
	}
}

func TestDeleteSetWhere(t *testing.T) {
	t.Parallel()
	base := Delete("t").Returning("id").Where("old=$", 99)
	stmt := base.Clone()
	exprs := []WhereExpr{Or(Where("a=$", 1), Where("b=$", 2)), Where("c=$", 3)}
	if stmt.SetWhere(exprs...) != stmt {
		t.Fatal("SetWhere must return its receiver")
	}
	exprs[0] = Where("changed")
	stmt.Apply(Where("d=$", 4)).Where("enabled").WhereNot("hidden")
	want := buildResult{
		query: "DELETE FROM t WHERE ((a=$1) OR (b=$2)) AND (c=$3) AND (d=$4) AND (enabled) AND (NOT (hidden)) RETURNING id",
		args:  []any{1, 2, 3, 4},
	}
	assertBuild(t, stmt, want)
	assertBuild(t, stmt.Clone(), want)
	assertBuild(t, base, buildResult{query: "DELETE FROM t WHERE (old=$1) RETURNING id", args: []any{99}})

	cleared := buildResult{query: "DELETE FROM t RETURNING id", args: nil}
	assertBuild(t, stmt.SetWhere(), cleared)
	assertBuild(t, stmt.Where("old").SetWhere([]WhereExpr(nil)...), cleared)
	assertBuild(t, stmt.Where("old").SetWhere([]WhereExpr{}...), cleared)
	assertBuild(t, stmt.SetWhere(Where("fresh=$", 5)), buildResult{query: "DELETE FROM t WHERE (fresh=$1) RETURNING id", args: []any{5}})
	for _, exprs := range [][]WhereExpr{{{}}, {Where("valid"), {}}} {
		assertBuild(t, stmt.SetWhere(exprs...), buildResult{err: "pql: WHERE condition is required"})
	}
	assertBuild(t, stmt.SetWhere(Where("")), buildResult{err: "pql: WHERE condition is required"})
	assertBuild(t, stmt.SetWhere(), cleared)
}
