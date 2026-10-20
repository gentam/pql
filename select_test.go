package pql

import (
	"testing"
)

func TestSelectBuild(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		stmt *SelectStmt
		want buildResult
	}{
		{name: "default columns", stmt: Select(), want: buildResult{query: "SELECT *"}},
		{name: "empty columns", stmt: Select([]string{}...), want: buildResult{query: "SELECT *"}},
		{name: "columns", stmt: Select("a", "1", "now()"), want: buildResult{query: "SELECT a,1,now()"}},
		{name: "whitespace in quoted result expressions", stmt: Select("''", " ' ' ", `" "`), want: buildResult{query: `SELECT '', ' ' ," "`}},
		{name: "empty appended columns", stmt: Select("a").Select().Select([]string{}...), want: buildResult{query: "SELECT a"}},
		{name: "from and chained select", stmt: Select("a").From("t").Select("b"), want: buildResult{query: "SELECT a,b FROM t"}},
		{name: "empty from", stmt: Select("1").From(""), want: buildResult{query: "SELECT 1"}},
		{name: "clear from", stmt: Select("1").From("t").From(""), want: buildResult{query: "SELECT 1"}},
		{name: "whitespace-only from", stmt: Select("1").From(" \t\n "), want: buildResult{err: "pql: FROM expression is required"}},
		{name: "unicode whitespace-only from", stmt: Select("1").From("\u3000"), want: buildResult{err: "pql: FROM expression is required"}},
		{name: "preserve from whitespace", stmt: Select("1").From(" users "), want: buildResult{query: "SELECT 1 FROM  users "}},
		{name: "ascending order", stmt: Select().From("t").Asc("c"), want: buildResult{query: "SELECT * FROM t ORDER BY c ASC"}},
		{name: "order expression", stmt: Select().From("t").Order(" COALESCE(a, b) ", true), want: buildResult{query: "SELECT * FROM t ORDER BY  COALESCE(a, b)  DESC"}},
		{
			name: "mixed order helpers",
			stmt: Select().From("t").Desc("a").Asc("b").Order("c", true).Order("d", false),
			want: buildResult{query: "SELECT * FROM t ORDER BY a DESC,b ASC,c DESC,d ASC"},
		},
		{
			name: "null ordering",
			stmt: Select().From("t").
				Asc("a", NullsFirst).
				Asc("b", NullsLast).
				Desc("c", NullsFirst).
				Desc("d", NullsLast),
			want: buildResult{query: "SELECT * FROM t ORDER BY a ASC NULLS FIRST,b ASC NULLS LAST,c DESC NULLS FIRST,d DESC NULLS LAST"},
		},
		{name: "limit and offset", stmt: Select().From("t").Limit(1).Offset(10), want: buildResult{query: "SELECT * FROM t LIMIT 1 OFFSET 10"}},
		{name: "explicit zero limit", stmt: Select().From("t").Limit(0), want: buildResult{query: "SELECT * FROM t LIMIT 0"}},
		{name: "explicit zero offset", stmt: Select().From("t").Offset(0), want: buildResult{query: "SELECT * FROM t OFFSET 0"}},
		{name: "zero limit and offset", stmt: Select().From("t").Limit(0).Offset(0), want: buildResult{query: "SELECT * FROM t LIMIT 0 OFFSET 0"}},
		{name: "negative limit", stmt: Select().From("t").Limit(-1), want: buildResult{err: "pql: LIMIT must not be negative"}},
		{name: "negative offset", stmt: Select().From("t").Offset(-1), want: buildResult{err: "pql: OFFSET must not be negative"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertBuild(t, tt.stmt, tt.want)
		})
	}
}

func TestSelectOffsetValue(t *testing.T) {
	const want = 10
	if got := Select().Offset(want).OffsetValue(); got != want {
		t.Fatalf("OffsetValue() = %d, want %d", got, want)
	}
}

func TestSelectWhere(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		build func() *SelectStmt
		want  buildResult
	}{
		{
			name: "raw condition",
			build: func() *SelectStmt {
				s := Select().From("t")
				s.Where("true")
				return s
			},
			want: buildResult{query: "SELECT * FROM t WHERE (true)"},
		},
		{
			name: "negated raw condition",
			build: func() *SelectStmt {
				s := Select().From("t")
				s.WhereNot("true")
				return s
			},
			want: buildResult{query: "SELECT * FROM t WHERE (NOT (true))"},
		},
		{
			name: "multiple raw placeholders",
			build: func() *SelectStmt {
				s := Select().From("t1")
				s.WhereNot("c1 in ($,$,$)", 1, 2, 3).
					Where("c2 = any (select id from t2 where c <> $)", "4")
				return s
			},
			want: buildResult{
				query: "SELECT * FROM t1 WHERE (NOT (c1 in ($1,$2,$3))) AND (c2 = any (select id from t2 where c <> $4))",
				args:  []any{1, 2, 3, "4"},
			},
		},
		{
			name: "applied condition",
			build: func() *SelectStmt {
				return Select().From("t").Apply(Or(Where("c1=$", 1), Where("c2=$", 2)))
			},
			want: buildResult{query: "SELECT * FROM t WHERE ((c1=$1) OR (c2=$2))", args: []any{1, 2}},
		},
		{
			name:  "zero applied clause",
			build: func() *SelectStmt { return Select().From("t").Apply(WhereExpr{}) },
			want:  buildResult{err: "pql: WHERE condition is required"},
		},
		{
			name: "postfix operator on expression with placeholders",
			build: func() *SelectStmt {
				s := Select().From("t")
				s.Where("(select 1 from t2 where c1>$ and c2<$) IS NULL", 1, 2)
				return s
			},
			want: buildResult{
				query: "SELECT * FROM t WHERE ((select 1 from t2 where c1>$1 and c2<$2) IS NULL)",
				args:  []any{1, 2},
			},
		},
		{
			name: "binary operator on expression with placeholders",
			build: func() *SelectStmt {
				s := Select().From("t")
				s.Where("coalesce($,$)=$", 1, 2, 3)
				return s
			},
			want: buildResult{
				query: "SELECT * FROM t WHERE (coalesce($1,$2)=$3)",
				args:  []any{1, 2, 3},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt := tt.build()
			assertBuild(t, stmt, tt.want)
		})
	}
}

func TestSelectSetWhere(t *testing.T) {
	t.Parallel()
	stmt := Select("id").From("t").Desc("id").Limit(5).Where("old=$", 99)
	exprs := []WhereExpr{Or(Where("a=$", 1), Where("b=$", 2)), Where("c=$", 3)}
	if stmt.SetWhere(exprs...) != stmt {
		t.Fatal("SetWhere must return its receiver")
	}
	exprs[0] = Where("changed")
	stmt.Apply(Where("d=$", 4)).Where("enabled").WhereNot("hidden")
	want := buildResult{
		query: "SELECT id FROM t WHERE ((a=$1) OR (b=$2)) AND (c=$3) AND (d=$4) AND (enabled) AND (NOT (hidden)) ORDER BY id DESC LIMIT 5",
		args:  []any{1, 2, 3, 4},
	}
	assertBuild(t, stmt, want)

	cleared := buildResult{query: "SELECT id FROM t ORDER BY id DESC LIMIT 5", args: nil}
	assertBuild(t, stmt.SetWhere(), cleared)
	assertBuild(t, stmt.Where("old").SetWhere([]WhereExpr(nil)...), cleared)
	assertBuild(t, stmt.Where("old").SetWhere([]WhereExpr{}...), cleared)
	assertBuild(t, stmt.SetWhere(Where("fresh=$", 5)), buildResult{query: "SELECT id FROM t WHERE (fresh=$1) ORDER BY id DESC LIMIT 5", args: []any{5}})
	for _, exprs := range [][]WhereExpr{{{}}, {Where("valid"), {}}} {
		assertBuild(t, stmt.SetWhere(exprs...), buildResult{err: "pql: WHERE condition is required"})
	}
	assertBuild(t, stmt.SetWhere(Where("")), buildResult{err: "pql: WHERE condition is required"})
	assertBuild(t, stmt.SetWhere(), cleared)
}
