package pql

import "testing"

// buildAt adapts a BuildAt method to [Builder] for assertBuild.
type buildAt struct {
	build  func(int) (string, []any, error)
	offset int
}

func (b buildAt) Build() (string, []any, error) { return b.build(b.offset) }

func TestStatementBuildAt(t *testing.T) {
	t.Parallel()
	orders := []OrderTerm{Desc("created_at"), Desc("id")}
	for _, tt := range []struct {
		name   string
		build  func(int) (string, []any, error)
		offset int
		want   buildResult
	}{
		{
			name:   "select",
			build:  Select("id").From("t").Where("a=$", 1).Apply(After(orders, "c", 2)).Limit(5).BuildAt,
			offset: 2,
			want: buildResult{
				query: "SELECT id FROM t WHERE (a=$3) AND ((ROW((created_at),(id)) <= ROW($4,$5)) AND ((created_at) <= $4 AND (created_at) IS NOT NULL) AND ((created_at) IS DISTINCT FROM $4 OR (((id) <= $5 AND (id) IS NOT NULL) AND (id) IS DISTINCT FROM $5))) LIMIT 5",
				args:  []any{1, "c", 2},
			},
		},
		{
			name:   "select without arguments",
			build:  Select("id").From("t").Where("active").BuildAt,
			offset: 3,
			want:   buildResult{query: "SELECT id FROM t WHERE (active)"},
		},
		{
			name:   "insert",
			build:  Insert("t").Set("b", 2).Set("a", 1).Returning("id").BuildAt,
			offset: 2,
			want:   buildResult{query: "INSERT INTO t (a,b) VALUES ($3,$4) RETURNING id", args: []any{1, 2}},
		},
		{
			name:   "update",
			build:  Update("t").Set("a", 1).Where("id=$", 2).BuildAt,
			offset: 1,
			want:   buildResult{query: "UPDATE t SET a=$2 WHERE (id=$3)", args: []any{1, 2}},
		},
		{
			name:   "delete",
			build:  Delete("t").Where("id=$", 1).Returning("id").BuildAt,
			offset: 4,
			want:   buildResult{query: "DELETE FROM t WHERE (id=$5) RETURNING id", args: []any{1}},
		},
		{
			name:   "delete all",
			build:  Delete("t").All().BuildAt,
			offset: 4,
			want:   buildResult{query: "DELETE FROM t"},
		},
		{
			name:   "zero offset",
			build:  Update("t").Set("a", 1).Where("id=$", 2).BuildAt,
			offset: 0,
			want:   buildResult{query: "UPDATE t SET a=$1 WHERE (id=$2)", args: []any{1, 2}},
		},
		{
			name:   "validation error",
			build:  Update("t").Set("a", 1).BuildAt,
			offset: 2,
			want:   buildResult{err: "pql: UPDATE requires a WHERE condition; call All to update every row"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertBuild(t, buildAt{tt.build, tt.offset}, tt.want)
		})
	}
}

func TestBuildAtNegativeOffset(t *testing.T) {
	t.Parallel()
	want := buildResult{err: "pql: parameter offset must not be negative"}
	for name, build := range map[string]func(int) (string, []any, error){
		"select": Select().From("t").BuildAt,
		"insert": Insert("t").Set("a", 1).BuildAt,
		"update": Update("t").Set("a", 1).All().BuildAt,
		"delete": Delete("t").All().BuildAt,
		"where":  Where("a").BuildAt,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assertBuild(t, buildAt{build, -1}, want)
		})
	}
}

func TestWhereExprBuildAt(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name   string
		expr   WhereExpr
		offset int
		want   buildResult
	}{
		{"leaf", Where("a=$ AND b=$", 1, 2), 6, buildResult{query: "(a=$7 AND b=$8)", args: []any{1, 2}}},
		{"no arguments", Where("active"), 2, buildResult{query: "(active)"}},
		{"zero offset", Where("a=$", 1), 0, buildResult{query: "(a=$1)", args: []any{1}}},
		{"composition", Or(Where("a=$", 1), Not(Where("b=$", 2))), 1, buildResult{query: "((a=$2) OR (NOT (b=$3)))", args: []any{1, 2}}},
		{"empty and", And(), 1, buildResult{query: "(TRUE)"}},
		{"keyset", After([]OrderTerm{Asc("id")}, 5), 1, buildResult{query: "(((id) >= $2 OR (id) IS NULL) AND (id) IS DISTINCT FROM $2)", args: []any{5}}},
		{"trailing comment", Where("a=$ -- note", 1), 1, buildResult{query: "(a=$2 -- note\n)", args: []any{1}}},
		{"zero value", WhereExpr{}, 1, buildResult{err: "pql: WHERE condition is required"}},
		{"argument mismatch", Where("a=$"), 1, buildResult{err: "pql: WHERE placeholder count 1 does not match argument count 0"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertBuild(t, buildAt{tt.expr.BuildAt, tt.offset}, tt.want)
		})
	}
}
