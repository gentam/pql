package pql

import "testing"

func TestInsertBuild(t *testing.T) {
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
		})
	}
}

func TestInsertValuesClone(t *testing.T) {
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
