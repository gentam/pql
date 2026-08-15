package pql

import "testing"

func TestDeleteBuild(t *testing.T) {
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
				s.Where("c1").Eq(1)
				s.WhereNot("c2").IsNotNull()
				return s.Returning("c1")
			},
			want: buildResult{
				query: "DELETE FROM t WHERE (c1=$1) AND (NOT c2 IS NOT NULL) RETURNING c1",
				args:  []any{1},
			},
		},
		{
			name: "applied and nested clauses",
			build: func() *DeleteStmt {
				w := Where("c0<=now()").Or("c0").IsNull()
				s := Delete("t").Apply(w)
				s.WhereNot("c1").Eq(1)
				w.Or("c2").Eq(2)
				return s
			},
			want: buildResult{
				query: "DELETE FROM t WHERE (c0<=now() OR (c0 IS NULL OR (c2=$1))) AND (NOT c1=$2)",
				args:  []any{2, 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertBuild(t, tt.build(), tt.want)
		})
	}
}
