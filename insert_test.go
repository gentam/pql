package pql

import "testing"

func TestInsertBuild(t *testing.T) {
	tests := []struct {
		name  string
		stmt  *InsertStmt
		wants []buildResult
	}{
		{
			name: "set",
			stmt: Insert("t").Set("c1", 1).Set("c2", 2),
			wants: []buildResult{
				{query: "INSERT INTO t (c1,c2) VALUES ($1,$2)", args: []interface{}{1, 2}},
				{query: "INSERT INTO t (c2,c1) VALUES ($1,$2)", args: []interface{}{2, 1}},
			},
		},
		{
			name: "values",
			stmt: Insert("t").Values(Map{"c1": 1, "c2": 2}),
			wants: []buildResult{
				{query: "INSERT INTO t (c1,c2) VALUES ($1,$2)", args: []interface{}{1, 2}},
				{query: "INSERT INTO t (c2,c1) VALUES ($1,$2)", args: []interface{}{2, 1}},
			},
		},
		{
			name: "returning",
			stmt: Insert("t").Set("c", 1).Returning("c1", "c2,c3").Returning("c4"),
			wants: []buildResult{
				{query: "INSERT INTO t (c) VALUES ($1) RETURNING c1,c2,c3,c4", args: []interface{}{1}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertBuildOneOf(t, tt.stmt, tt.wants...)
		})
	}
}
