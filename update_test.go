package pql

import "testing"

func TestUpdateBuild(t *testing.T) {
	tests := []struct {
		name  string
		build func() *UpdateStmt
		want  buildResult
	}{
		{
			name:  "set",
			build: func() *UpdateStmt { return Update("t").Set("c1", 1).Set("c2", 2) },
			want:  buildResult{query: "UPDATE t SET c1=$1,c2=$2", args: []any{1, 2}},
		},
		{
			name:  "values",
			build: func() *UpdateStmt { return Update("t").Values(Map{"c1": 1, "c2": 2}) },
			want:  buildResult{query: "UPDATE t SET c1=$1,c2=$2", args: []any{1, 2}},
		},
		{
			name: "where and returning",
			build: func() *UpdateStmt {
				s := Update("t").Set("c1", 1)
				s.Where("c2").Eq(2)
				s.WhereNot("c3").Neq(3)
				return s.Returning("c1")
			},
			want: buildResult{
				query: "UPDATE t SET c1=$1 WHERE (c2=$2) AND (NOT c3<>$3) RETURNING c1",
				args:  []any{1, 2, 3},
			},
		},
		{
			name: "applied clauses",
			build: func() *UpdateStmt {
				return Update("t").Set("c0", 0).
					Apply(Where("c1").Eq(1)).
					Apply(Where("c2").Neq(2).Or("c3").IsNull())
			},
			want: buildResult{
				query: "UPDATE t SET c0=$1 WHERE (c1=$2) AND (c2<>$3 OR (c3 IS NULL))",
				args:  []any{0, 1, 2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertBuild(t, tt.build(), tt.want)
		})
	}
}
