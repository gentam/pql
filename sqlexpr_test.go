package pql

import "testing"

func TestSQLExprValues(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		stmt Builder
		want buildResult
	}{
		{
			name: "update without arguments",
			stmt: Update("t").Set("a", 1).Set("updated_at", SQL("now()")).Where("id=$", 2),
			want: buildResult{query: "UPDATE t SET a=$1,updated_at=now() WHERE (id=$2)", args: []any{1, 2}},
		},
		{
			name: "update with arguments in order",
			stmt: Update("t").Values(Map{"a": SQL("a + $ * $", 2, 3), "b": 4, "c": SQL("$::date", "2026-01-31")}).Where("id=$", 5),
			want: buildResult{query: "UPDATE t SET a=a + $1 * $2,b=$3,c=$4::date WHERE (id=$5)", args: []any{2, 3, 4, "2026-01-31", 5}},
		},
		{
			name: "update column expression",
			stmt: Update("t").Set("flag", SQL("NOT flag")).Where("id=$", 1).Returning("flag"),
			want: buildResult{query: "UPDATE t SET flag=NOT flag WHERE (id=$1) RETURNING flag", args: []any{1}},
		},
		{
			name: "insert",
			stmt: Insert("t").Set("a", 1).Set("b", SQL("DEFAULT")).Set("c", SQL("lower($)", "X")).Returning("id"),
			want: buildResult{query: "INSERT INTO t (a,b,c) VALUES ($1,DEFAULT,lower($2)) RETURNING id", args: []any{1, "X"}},
		},
		{
			name: "build at offset",
			stmt: buildAt{Insert("t").Set("a", SQL("$ || $", "x", "y")).BuildAt, 3},
			want: buildResult{query: "INSERT INTO t (a) VALUES ($4 || $5)", args: []any{"x", "y"}},
		},
		{
			name: "literals and comments",
			stmt: Update("t").Set("a", SQL("'$' || $ -- note", "x")).All(),
			want: buildResult{query: "UPDATE t SET a='$' || $1 -- note\n", args: []any{"x"}},
		},
		{"empty update expression", Update("t").Set("a", SQL(" ")).All(), buildResult{err: "pql: SET expression is required"}},
		{"zero insert expression", Insert("t").Set("a", SQLExpr{}), buildResult{err: "pql: VALUES expression is required"}},
		{"missing argument", Update("t").Set("a", SQL("$")).All(), buildResult{err: "pql: SET placeholder count 1 does not match argument count 0"}},
		{"extra argument", Insert("t").Set("a", SQL("now()", 1)), buildResult{err: "pql: VALUES placeholder count 0 does not match argument count 1"}},
		{"numbered placeholder", Update("t").Set("a", SQL("$1", 1)).All(), buildResult{err: "pql: numbered placeholders are not supported in SET expressions; use $"}},
		{"named placeholder", Insert("t").Set("a", SQL("$x", 1)), buildResult{err: "pql: invalid VALUES placeholder; use $"}},
		{"unterminated string", Update("t").Set("a", SQL("'x")).All(), buildResult{err: "pql: unterminated string in SET expression"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertBuild(t, tt.stmt, tt.want)
		})
	}
}

func TestSQLExprCopiesArguments(t *testing.T) {
	t.Parallel()
	args := []any{1}
	e := SQL("a + $", args...)
	args[0] = 2
	stmt := Update("t").Set("a", e).All()
	clone := stmt.Clone().Set("b", SQL("b - $", 3))
	assertBuild(t, stmt, buildResult{query: "UPDATE t SET a=a + $1", args: []any{1}})
	assertBuild(t, clone, buildResult{query: "UPDATE t SET a=a + $1,b=b - $2", args: []any{1, 3}})
}
