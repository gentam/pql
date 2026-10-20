package pql

import "testing"

func TestUpdateBuild(t *testing.T) {
	t.Parallel()
	type fields map[string]any
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
			name:  "plain map",
			build: func() *UpdateStmt { return Update("t").Values(map[string]any{"c1": 1, "c2": 2}) },
			want:  buildResult{query: "UPDATE t SET c1=$1,c2=$2", args: []any{1, 2}},
		},
		{
			name:  "defined map",
			build: func() *UpdateStmt { return Update("t").Values(fields{"c1": 1, "c2": 2}) },
			want:  buildResult{query: "UPDATE t SET c1=$1,c2=$2", args: []any{1, 2}},
		},
		{
			name:  "set values plain map",
			build: func() *UpdateStmt { return Update("t").SetValues(map[string]any{"c1": 1, "c2": 2}) },
			want:  buildResult{query: "UPDATE t SET c1=$1,c2=$2", args: []any{1, 2}},
		},
		{
			name:  "set values defined map",
			build: func() *UpdateStmt { return Update("t").SetValues(fields{"c1": 1, "c2": 2}) },
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
			assertBuild(t, stmt.Clone(), tt.want)
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

func TestUpdateSetValues(t *testing.T) {
	t.Parallel()
	base := Update("t").Values(Map{"c": 0, "old": 0}).Where("id=$", 99).Returning("id")
	values := Map{"c": 1, "nullable": nil}
	stmt := base.Clone()
	if stmt.SetValues(values) != stmt {
		t.Fatal("SetValues must return its receiver")
	}
	values["c"] = 2
	delete(values, "nullable")
	values["outside"] = 3
	assertBuild(t, stmt, buildResult{
		query: "UPDATE t SET c=$1,nullable=$2 WHERE (id=$3) RETURNING id",
		args:  []any{1, nil, 99},
	})

	stmt.Set("d", 3).Values(Map{"c": 4})
	assertBuild(t, stmt, buildResult{
		query: "UPDATE t SET c=$1,d=$2,nullable=$3 WHERE (id=$4) RETURNING id",
		args:  []any{4, 3, nil, 99},
	})
	if _, ok := values["d"]; ok || values["c"] != 2 {
		t.Error("Set or Values modified the map passed to SetValues")
	}
	assertBuild(t, base, buildResult{
		query: "UPDATE t SET c=$1,old=$2 WHERE (id=$3) RETURNING id",
		args:  []any{0, 0, 99},
	})
}

func TestUpdateSetValuesValidation(t *testing.T) {
	t.Parallel()
	for _, values := range []Map{nil, {}} {
		stmt := Update("t").Set("old", 0).Where("id=$", 99).Returning("id").SetValues(values)
		assertBuild(t, stmt, buildResult{err: "pql: UPDATE values are required"})
		assertBuild(t, stmt.Set("c", 1), buildResult{
			query: "UPDATE t SET c=$1 WHERE (id=$2) RETURNING id",
			args:  []any{1, 99},
		})
		assertBuild(t, stmt.SetValues(values).Values(Map{"d": 2}), buildResult{
			query: "UPDATE t SET d=$1 WHERE (id=$2) RETURNING id",
			args:  []any{2, 99},
		})
	}
	for _, col := range []string{"", " \t"} {
		assertBuild(t, Update("t").SetValues(Map{col: 1}), buildResult{err: "pql: UPDATE column is required"})
	}
}

func TestUpdateSetReturning(t *testing.T) {
	t.Parallel()
	base := Update("t").Set("c", 1).Returning("old")
	cols := []string{"c", "c + 1 AS next"}
	stmt := base.Clone()
	if stmt.SetReturning(cols...) != stmt {
		t.Fatal("SetReturning must return its receiver")
	}
	cols[0] = "changed"
	stmt.Returning().Returning("c + 2")
	assertBuild(t, stmt, buildResult{
		query: "UPDATE t SET c=$1 RETURNING c,c + 1 AS next,c + 2",
		args:  []any{1},
	})
	assertBuild(t, base, buildResult{query: "UPDATE t SET c=$1 RETURNING old", args: []any{1}})
	assertBuild(t, stmt.SetReturning(), buildResult{query: "UPDATE t SET c=$1", args: []any{1}})
	assertBuild(t, stmt.Returning("c").SetReturning([]string(nil)...), buildResult{query: "UPDATE t SET c=$1", args: []any{1}})
	assertBuild(t, stmt.Returning("c").SetReturning([]string{}...), buildResult{query: "UPDATE t SET c=$1", args: []any{1}})
	for _, col := range []string{"", "  "} {
		assertBuild(t, stmt.SetReturning(col), buildResult{err: "pql: RETURNING expression is required"})
	}
}

func TestUpdateSetWhere(t *testing.T) {
	t.Parallel()
	base := Update("t").Set("active", true).Returning("id").Where("old=$", 99)
	stmt := base.Clone()
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
	assertBuild(t, stmt.Clone(), want)
	assertBuild(t, base, buildResult{query: "UPDATE t SET active=$1 WHERE (old=$2) RETURNING id", args: []any{true, 99}})

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
