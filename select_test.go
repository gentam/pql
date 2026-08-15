package pql

import "testing"

func TestSelectBuild(t *testing.T) {
	tests := []struct {
		name string
		stmt *SelectStmt
		want buildResult
	}{
		{name: "default columns", stmt: Select(), want: buildResult{query: "SELECT *"}},
		{name: "empty columns", stmt: Select([]string{}...), want: buildResult{query: "SELECT *"}},
		{name: "columns", stmt: Select("a", "1", "now()"), want: buildResult{query: "SELECT a,1,now()"}},
		{name: "from and chained select", stmt: Select("a").From("t").Select("b"), want: buildResult{query: "SELECT a,b FROM t"}},
		{name: "ascending order", stmt: Select().From("t").Asc("c"), want: buildResult{query: "SELECT * FROM t ORDER BY c ASC"}},
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
			want: buildResult{query: "SELECT * FROM t WHERE (NOT true)"},
		},
		{
			name: "conditional clauses",
			build: func() *SelectStmt {
				s := Select().From("t")
				s.WhereCond(true, "c1=?", 1)
				s.WhereCond(false, "c2")
				return s
			},
			want: buildResult{query: "SELECT * FROM t WHERE (c1=$1) AND (NOT c2)", args: []any{1}},
		},
		{
			name: "multiple raw placeholders",
			build: func() *SelectStmt {
				s := Select().From("t1")
				s.WhereNot("c1 in (?,?,?)", 1, 2, 3)
				s.Where("c2 = any (select id from t2 where c <> ?)", "4")
				return s
			},
			want: buildResult{
				query: "SELECT * FROM t1 WHERE (NOT c1 in ($1,$2,$3)) AND (c2 = any (select id from t2 where c <> $4))",
				args:  []any{1, 2, 3, "4"},
			},
		},
		{
			name: "applied detached clause",
			build: func() *SelectStmt {
				return Select().From("t").Apply(Where("c1").Eq(1).Or("c2").Eq(2))
			},
			want: buildResult{query: "SELECT * FROM t WHERE (c1=$1 OR (c2=$2))", args: []any{1, 2}},
		},
		{
			name:  "nil applied clause",
			build: func() *SelectStmt { return Select().From("t").Apply(nil) },
			want:  buildResult{query: "SELECT * FROM t"},
		},
		{
			name: "postfix operator on expression with placeholders",
			build: func() *SelectStmt {
				s := Select().From("t")
				s.Where("(select 1 from t2 where c1>? and c2<?)", 1, 2).IsNull()
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
				s.Where("coalesce(?,?)", 1, 2).Eq(3)
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
			assertBuild(t, tt.build(), tt.want)
		})
	}
}
