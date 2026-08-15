package pql

import "testing"

func TestWhereOperator(t *testing.T) {
	tests := []struct {
		name  string
		apply func(*WhereExpr)
		want  buildResult
	}{
		{name: "is null", apply: func(w *WhereExpr) { w.IsNull() }, want: buildResult{query: "SELECT * FROM t WHERE (c IS NULL)"}},
		{name: "is not null", apply: func(w *WhereExpr) { w.IsNotNull() }, want: buildResult{query: "SELECT * FROM t WHERE (c IS NOT NULL)"}},
		{name: "equal", apply: func(w *WhereExpr) { w.Eq(1) }, want: buildResult{query: "SELECT * FROM t WHERE (c=$1)", args: []any{1}}},
		{name: "equal nil", apply: func(w *WhereExpr) { w.Eq(nil) }, want: buildResult{query: "SELECT * FROM t WHERE (c=$1)", args: []any{nil}}},
		{name: "not equal", apply: func(w *WhereExpr) { w.Neq(1) }, want: buildResult{query: "SELECT * FROM t WHERE (c<>$1)", args: []any{1}}},
		{name: "less than", apply: func(w *WhereExpr) { w.Lt(1) }, want: buildResult{query: "SELECT * FROM t WHERE (c<$1)", args: []any{1}}},
		{name: "greater than", apply: func(w *WhereExpr) { w.Gt(1) }, want: buildResult{query: "SELECT * FROM t WHERE (c>$1)", args: []any{1}}},
		{name: "less or equal", apply: func(w *WhereExpr) { w.Le(1) }, want: buildResult{query: "SELECT * FROM t WHERE (c<=$1)", args: []any{1}}},
		{name: "greater or equal", apply: func(w *WhereExpr) { w.Ge(1) }, want: buildResult{query: "SELECT * FROM t WHERE (c>=$1)", args: []any{1}}},
		{name: "like", apply: func(w *WhereExpr) { w.Like("a%") }, want: buildResult{query: "SELECT * FROM t WHERE (c LIKE $1)", args: []any{"a%"}}},
		{name: "ilike", apply: func(w *WhereExpr) { w.Ilike("a%") }, want: buildResult{query: "SELECT * FROM t WHERE (c ILIKE $1)", args: []any{"a%"}}},
		{name: "contains", apply: func(w *WhereExpr) { w.Contains([]int{1, 2}) }, want: buildResult{query: "SELECT * FROM t WHERE (c@>$1)", args: []any{[]int{1, 2}}}},
		{name: "contained by", apply: func(w *WhereExpr) { w.ContainedBy([]int{1, 2}) }, want: buildResult{query: "SELECT * FROM t WHERE (c<@$1)", args: []any{[]int{1, 2}}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Select().From("t")
			tt.apply(s.Where("c"))
			assertBuild(t, s, tt.want)
		})
	}
}

func TestWhereLogicalChain(t *testing.T) {
	s := Select().From("t")
	s.Where("c1").Or("c2").Eq(2).And("c3 = ?", 3)
	s.WhereNot("c4").And("c5 < ?", 5).Or("c6").Eq("6")

	assertBuild(t, s, buildResult{
		query: "SELECT * FROM t WHERE (c1 OR (c2=$1 AND (c3 = $2))) AND (NOT c4 AND (c5 < $3 OR (c6=$4)))",
		args:  []any{2, 3, 5, "6"},
	})
}

func TestWhereBuild(t *testing.T) {
	t.Run("detached clause", func(t *testing.T) {
		assertBuild(t, Where("c"), buildResult{err: "pql: cannot build detached WHERE clause"})
	})

	t.Run("attached clause", func(t *testing.T) {
		s := Select().From("t")
		w := s.Where("c").Eq(1)
		assertBuild(t, w, buildResult{query: "SELECT * FROM t WHERE (c=$1)", args: []any{1}})
	})

	t.Run("nested clause attached after creation", func(t *testing.T) {
		root := Where("c1").Eq(1)
		child := root.And("c2").Eq(2)
		Select().From("t").Apply(root)
		assertBuild(t, child, buildResult{
			query: "SELECT * FROM t WHERE (c1=$1 AND (c2=$2))",
			args:  []any{1, 2},
		})
	})

	t.Run("build after statement mutation", func(t *testing.T) {
		s := Update("t").Set("b", 2)
		s.Where("c=?", 3)
		assertBuild(t, s, buildResult{query: "UPDATE t SET b=$1 WHERE (c=$2)", args: []any{2, 3}})

		s.Set("a", 1)
		assertBuild(t, s, buildResult{query: "UPDATE t SET a=$1,b=$2 WHERE (c=$3)", args: []any{1, 2, 3}})
	})

	t.Run("reuse detached clause", func(t *testing.T) {
		w := Where("c=?", 2)
		assertBuild(t, Update("t").Set("a", 1).Apply(w), buildResult{
			query: "UPDATE t SET a=$1 WHERE (c=$2)",
			args:  []any{1, 2},
		})
		assertBuild(t, Select().From("t").Apply(w), buildResult{
			query: "SELECT * FROM t WHERE (c=$1)",
			args:  []any{2},
		})
		assertBuild(t, Delete("t").Apply(w), buildResult{
			query: "DELETE FROM t WHERE (c=$1)",
			args:  []any{2},
		})
	})
}
