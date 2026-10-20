package pql

import "testing"

func TestInsertBuild(t *testing.T) {
	t.Parallel()
	type fields map[string]any
	tests := []struct {
		name string
		stmt *InsertStmt
		want buildResult
	}{
		{
			name: "set",
			stmt: Insert("t").Set("c1", 1).Set("c2", 2),
			want: buildResult{query: "INSERT INTO t (c1,c2) VALUES ($1,$2)", args: []any{1, 2}},
		},
		{
			name: "values",
			stmt: Insert("t").Values(Map{"c1": 1, "c2": 2}),
			want: buildResult{query: "INSERT INTO t (c1,c2) VALUES ($1,$2)", args: []any{1, 2}},
		},
		{
			name: "plain map",
			stmt: Insert("t").Values(map[string]any{"c1": 1, "c2": 2}),
			want: buildResult{query: "INSERT INTO t (c1,c2) VALUES ($1,$2)", args: []any{1, 2}},
		},
		{
			name: "defined map",
			stmt: Insert("t").Values(fields{"c1": 1, "c2": 2}),
			want: buildResult{query: "INSERT INTO t (c1,c2) VALUES ($1,$2)", args: []any{1, 2}},
		},
		{
			name: "set values plain map",
			stmt: Insert("t").SetValues(map[string]any{"c1": 1, "c2": 2}),
			want: buildResult{query: "INSERT INTO t (c1,c2) VALUES ($1,$2)", args: []any{1, 2}},
		},
		{
			name: "set values defined map",
			stmt: Insert("t").SetValues(fields{"c1": 1, "c2": 2}),
			want: buildResult{query: "INSERT INTO t (c1,c2) VALUES ($1,$2)", args: []any{1, 2}},
		},
		{
			name: "values merge with existing values",
			stmt: Insert("t").Set("c1", 1).Set("c2", 2).
				Values(Map{"c2": 20, "c3": 3}),
			want: buildResult{
				query: "INSERT INTO t (c1,c2,c3) VALUES ($1,$2,$3)",
				args:  []any{1, 20, 3},
			},
		},
		{
			name: "set after nil values",
			stmt: Insert("t").Values(nil).Set("c", 1),
			want: buildResult{query: "INSERT INTO t (c) VALUES ($1)", args: []any{1}},
		},
		{
			name: "returning",
			stmt: Insert("t").Set("c", 1).Returning("c1", "c2,c3").Returning("c4"),
			want: buildResult{query: "INSERT INTO t (c) VALUES ($1) RETURNING c1,c2,c3,c4", args: []any{1}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertBuild(t, tt.stmt, tt.want)
			assertBuild(t, tt.stmt.Clone(), tt.want)
		})
	}
}

func TestInsertValuesClone(t *testing.T) {
	t.Parallel()
	values := Map{"c": 1}
	stmt := Insert("t").Values(values)

	values["c"] = 2
	stmt.Set("d", 3)

	assertBuild(t, stmt, buildResult{
		query: "INSERT INTO t (c,d) VALUES ($1,$2)",
		args:  []any{1, 3},
	})
	if _, ok := values["d"]; ok {
		t.Error("Set modified the map passed to Values")
	}
}

func TestInsertSetValues(t *testing.T) {
	t.Parallel()
	base := Insert("t").Values(Map{"c": 0, "old": 0}).Returning("id")
	values := Map{"c": 1, "nullable": nil}
	stmt := base.Clone()
	if stmt.SetValues(values) != stmt {
		t.Fatal("SetValues must return its receiver")
	}
	values["c"] = 2
	delete(values, "nullable")
	values["outside"] = 3
	assertBuild(t, stmt, buildResult{
		query: "INSERT INTO t (c,nullable) VALUES ($1,$2) RETURNING id",
		args:  []any{1, nil},
	})

	stmt.Set("d", 3).Values(Map{"c": 4})
	assertBuild(t, stmt, buildResult{
		query: "INSERT INTO t (c,d,nullable) VALUES ($1,$2,$3) RETURNING id",
		args:  []any{4, 3, nil},
	})
	if _, ok := values["d"]; ok || values["c"] != 2 {
		t.Error("Set or Values modified the map passed to SetValues")
	}
	assertBuild(t, base, buildResult{
		query: "INSERT INTO t (c,old) VALUES ($1,$2) RETURNING id",
		args:  []any{0, 0},
	})
}

func TestInsertSetValuesValidation(t *testing.T) {
	t.Parallel()
	for _, values := range []Map{nil, {}} {
		stmt := Insert("t").Set("old", 0).Returning("id").SetValues(values)
		assertBuild(t, stmt, buildResult{err: "pql: INSERT values are required"})
		assertBuild(t, stmt.Set("c", 1), buildResult{
			query: "INSERT INTO t (c) VALUES ($1) RETURNING id",
			args:  []any{1},
		})
		assertBuild(t, stmt.SetValues(values).Values(Map{"d": 2}), buildResult{
			query: "INSERT INTO t (d) VALUES ($1) RETURNING id",
			args:  []any{2},
		})
	}
	for _, col := range []string{"", " \t"} {
		assertBuild(t, Insert("t").SetValues(Map{col: 1}), buildResult{err: "pql: INSERT column is required"})
	}
}

func TestInsertSetReturning(t *testing.T) {
	t.Parallel()
	base := Insert("t").Set("c", 1).Returning("old")
	cols := []string{"c", "c + 1 AS next"}
	stmt := base.Clone()
	if stmt.SetReturning(cols...) != stmt {
		t.Fatal("SetReturning must return its receiver")
	}
	cols[0] = "changed"
	stmt.Returning().Returning("c + 2")
	assertBuild(t, stmt, buildResult{
		query: "INSERT INTO t (c) VALUES ($1) RETURNING c,c + 1 AS next,c + 2",
		args:  []any{1},
	})
	assertBuild(t, base, buildResult{query: "INSERT INTO t (c) VALUES ($1) RETURNING old", args: []any{1}})
	assertBuild(t, stmt.SetReturning(), buildResult{query: "INSERT INTO t (c) VALUES ($1)", args: []any{1}})
	assertBuild(t, stmt.Returning("c").SetReturning([]string(nil)...), buildResult{query: "INSERT INTO t (c) VALUES ($1)", args: []any{1}})
	assertBuild(t, stmt.Returning("c").SetReturning([]string{}...), buildResult{query: "INSERT INTO t (c) VALUES ($1)", args: []any{1}})
	for _, col := range []string{"", "  "} {
		assertBuild(t, stmt.SetReturning(col), buildResult{err: "pql: RETURNING expression is required"})
	}
}
