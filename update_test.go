package pql

import "testing"

func TestUpdateBuild(t *testing.T) {
	t.Parallel()
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
			name: "values merge with existing values",
			build: func() *UpdateStmt {
				return Update("t").Set("c1", 1).Set("c2", 2).
					Values(Map{"c2": 20, "c3": 3})
			},
			want: buildResult{
				query: "UPDATE t SET c1=$1,c2=$2,c3=$3",
				args:  []any{1, 20, 3},
			},
		},
		{
			name:  "set after nil values",
			build: func() *UpdateStmt { return Update("t").Values(nil).Set("c", 1) },
			want:  buildResult{query: "UPDATE t SET c=$1", args: []any{1}},
		},
		{
			name:  "zero applied clause",
			build: func() *UpdateStmt { return Update("t").Set("c", 1).Apply(WhereExpr{}) },
			want:  buildResult{err: "pql: WHERE condition is required"},
		},
		{
			name: "where and returning",
			build: func() *UpdateStmt {
				s := Update("t").Set("c1", 1)
				s.Where("c2=$", 2)
				return s.WhereNot("c3<>$", 3).Returning("c1")
			},
			want: buildResult{
				query: "UPDATE t SET c1=$1 WHERE (c2=$2) AND (NOT (c3<>$3)) RETURNING c1",
				args:  []any{1, 2, 3},
			},
		},
		{
			name: "applied clauses",
			build: func() *UpdateStmt {
				return Update("t").Set("c0", 0).
					Apply(Where("c1=$", 1)).
					Apply(Or(Where("c2<>$", 2), Where("c3 IS NULL")))
			},
			want: buildResult{
				query: "UPDATE t SET c0=$1 WHERE (c1=$2) AND ((c2<>$3) OR (c3 IS NULL))",
				args:  []any{0, 1, 2},
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

func TestUpdateValuesClone(t *testing.T) {
	t.Parallel()
	values := Map{"c": 1}
	stmt := Update("t").Values(values)

	values["c"] = 2
	stmt.Set("d", 3)

	assertBuild(t, stmt, buildResult{
		query: "UPDATE t SET c=$1,d=$2",
		args:  []any{1, 3},
	})
	if _, ok := values["d"]; ok {
		t.Error("Set modified the map passed to Values")
	}
}

func TestUpdateSetWhere(t *testing.T) {
	t.Parallel()
	stmt := Update("t").Set("active", true).Returning("id").Where("old=$", 99)
	exprs := []WhereExpr{Or(Where("a=$", 1), Where("b=$", 2)), Where("c=$", 3)}
	if stmt.SetWhere(exprs...) != stmt {
		t.Fatal("SetWhere must return its receiver")
	}
	exprs[0] = Where("changed")
	stmt.Apply(Where("d=$", 4)).Where("enabled").WhereNot("hidden")
	want := buildResult{
		query: "UPDATE t SET active=$1 WHERE ((a=$2) OR (b=$3)) AND (c=$4) AND (d=$5) AND (enabled) AND (NOT (hidden)) RETURNING id",
		args:  []any{true, 1, 2, 3, 4},
	}
	assertBuild(t, stmt, want)

	cleared := buildResult{query: "UPDATE t SET active=$1 RETURNING id", args: []any{true}}
	assertBuild(t, stmt.SetWhere(), cleared)
	assertBuild(t, stmt.Where("old").SetWhere([]WhereExpr(nil)...), cleared)
	assertBuild(t, stmt.Where("old").SetWhere([]WhereExpr{}...), cleared)
	assertBuild(t, stmt.SetWhere(Where("fresh=$", 5)), buildResult{query: "UPDATE t SET active=$1 WHERE (fresh=$2) RETURNING id", args: []any{true, 5}})
	for _, exprs := range [][]WhereExpr{{{}}, {Where("valid"), {}}} {
		assertBuild(t, stmt.SetWhere(exprs...), buildResult{err: "pql: WHERE condition is required"})
	}
	assertBuild(t, stmt.SetWhere(Where("")), buildResult{err: "pql: WHERE condition is required"})
	assertBuild(t, stmt.SetWhere(), cleared)
}
