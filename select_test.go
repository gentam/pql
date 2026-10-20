package pql

import (
	"slices"
	"testing"
)

func TestSelectColumns(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		stmt *SelectStmt
		want []string
	}{
		{"zero value", &SelectStmt{}, nil},
		{"unspecified", Select(), nil},
		{"empty slice", Select([]string{}...), nil},
		{"cleared", Select("id").SetColumns(), nil},
		{"explicit star", Select("*"), []string{"*"}},
		{"expressions preserved", Select("a,b", "count(*) AS total", " ' ' "), []string{"a,b", "count(*) AS total", " ' ' "}},
		{"appended", Select("id").Select("name"), []string{"id", "name"}},
		{"replaced", Select("old").SetColumns("id", "name"), []string{"id", "name"}},
		{"validation deferred", Select(""), []string{""}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.stmt.Columns(); !slices.Equal(got, tt.want) {
				t.Fatalf("Columns() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestSelectColumnsIndependentCopy(t *testing.T) {
	t.Parallel()
	// Spare capacity exposes shared backing arrays when both slices append.
	stmt := (&SelectStmt{cols: make([]string, 0, 4)}).Select("id", "name")
	cols := stmt.Columns()
	cols[0] = "other_id"
	cols = append(cols, "client_only")
	stmt.Select("server_only")
	assertBuild(t, stmt, buildResult{query: "SELECT id,name,server_only"})
	if !slices.Equal(cols, []string{"other_id", "name", "client_only"}) {
		t.Fatalf("statement changed the returned columns: %v", cols)
	}
	assertBuild(t, stmt.Clone().SetColumns(cols...), buildResult{query: "SELECT other_id,name,client_only"})
	assertBuild(t, stmt, buildResult{query: "SELECT id,name,server_only"})
}

func TestSelectBuild(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		stmt *SelectStmt
		want buildResult
	}{
		{name: "default columns", stmt: Select(), want: buildResult{query: "SELECT *"}},
		{name: "empty columns", stmt: Select([]string{}...), want: buildResult{query: "SELECT *"}},
		{name: "columns", stmt: Select("a", "1", "now()"), want: buildResult{query: "SELECT a,1,now()"}},
		{name: "whitespace in quoted result expressions", stmt: Select("''", " ' ' ", `" "`), want: buildResult{query: `SELECT '', ' ' ," "`}},
		{name: "empty appended columns", stmt: Select("a").Select().Select([]string{}...), want: buildResult{query: "SELECT a"}},
		{name: "from and chained select", stmt: Select("a").From("t").Select("b"), want: buildResult{query: "SELECT a,b FROM t"}},
		{name: "empty from", stmt: Select("1").From(""), want: buildResult{query: "SELECT 1"}},
		{name: "clear from", stmt: Select("1").From("t").From(""), want: buildResult{query: "SELECT 1"}},
		{name: "whitespace-only from", stmt: Select("1").From(" \t\n "), want: buildResult{err: "pql: FROM expression is required"}},
		{name: "unicode whitespace-only from", stmt: Select("1").From("\u3000"), want: buildResult{err: "pql: FROM expression is required"}},
		{name: "preserve from whitespace", stmt: Select("1").From(" users "), want: buildResult{query: "SELECT 1 FROM  users "}},
		{name: "replace columns then append", stmt: Select("old").From("t").Select("unused").SetColumns("a", "b").Select("c"), want: buildResult{query: "SELECT a,b,c FROM t"}},
		{name: "clear columns", stmt: Select("a").From("t").SetColumns(), want: buildResult{query: "SELECT * FROM t"}},
		{name: "replace columns with empty slice", stmt: Select("a").From("t").SetColumns([]string{}...), want: buildResult{query: "SELECT * FROM t"}},
		{name: "append after clearing columns", stmt: Select("a").From("t").SetColumns().Select("b"), want: buildResult{query: "SELECT b FROM t"}},
		{name: "ascending order", stmt: Select().From("t").Asc("c"), want: buildResult{query: "SELECT * FROM t ORDER BY c ASC"}},
		{name: "order expression", stmt: Select().From("t").Order(" COALESCE(a, b) ", true), want: buildResult{query: "SELECT * FROM t ORDER BY  COALESCE(a, b)  DESC"}},
		{
			name: "mixed order helpers",
			stmt: Select().From("t").Desc("a").Asc("b").Order("c", true).Order("d", false),
			want: buildResult{query: "SELECT * FROM t ORDER BY a DESC,b ASC,c DESC,d ASC"},
		},
		{
			name: "null ordering",
			stmt: Select().From("t").
				Asc("a", NullsFirst).
				Asc("b", NullsLast).
				Desc("c", NullsFirst).
				Desc("d", NullsLast),
			want: buildResult{query: "SELECT * FROM t ORDER BY a ASC NULLS FIRST,b ASC NULLS LAST,c DESC NULLS FIRST,d DESC NULLS LAST"},
		},
		{name: "limit and offset", stmt: Select().From("t").Limit(1).Offset(10), want: buildResult{query: "SELECT * FROM t LIMIT 1 OFFSET 10"}},
		{name: "explicit zero limit", stmt: Select().From("t").Limit(0), want: buildResult{query: "SELECT * FROM t LIMIT 0"}},
		{name: "explicit zero offset", stmt: Select().From("t").Offset(0), want: buildResult{query: "SELECT * FROM t OFFSET 0"}},
		{name: "zero limit and offset", stmt: Select().From("t").Limit(0).Offset(0), want: buildResult{query: "SELECT * FROM t LIMIT 0 OFFSET 0"}},
		{name: "negative limit", stmt: Select().From("t").Limit(-1), want: buildResult{err: "pql: LIMIT must not be negative"}},
		{name: "negative offset", stmt: Select().From("t").Offset(-1), want: buildResult{err: "pql: OFFSET must not be negative"}},
		{name: "clear limit", stmt: Select().From("t").Limit(10).Offset(5).ClearLimit(), want: buildResult{query: "SELECT * FROM t OFFSET 5"}},
		{name: "clear offset", stmt: Select().From("t").Limit(10).Offset(5).ClearOffset(), want: buildResult{query: "SELECT * FROM t LIMIT 10"}},
		{name: "clear unset pagination", stmt: Select().From("t").ClearLimit().ClearOffset(), want: buildResult{query: "SELECT * FROM t"}},
		{name: "clear zero pagination", stmt: Select().From("t").Limit(0).Offset(0).ClearLimit().ClearOffset(), want: buildResult{query: "SELECT * FROM t"}},
		{name: "clear negative pagination", stmt: Select().From("t").Limit(-1).Offset(-1).ClearLimit().ClearOffset(), want: buildResult{query: "SELECT * FROM t"}},
		{name: "clear pagination repeatedly", stmt: Select().From("t").Limit(10).Offset(5).ClearLimit().ClearOffset().ClearLimit().ClearOffset(), want: buildResult{query: "SELECT * FROM t"}},
		{name: "set pagination after clearing", stmt: Select().From("t").Limit(10).Offset(5).ClearLimit().ClearOffset().Limit(0).Offset(0), want: buildResult{query: "SELECT * FROM t LIMIT 0 OFFSET 0"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertBuild(t, tt.stmt, tt.want)
			t.Run("clone", func(t *testing.T) {
				assertBuild(t, tt.stmt.Clone(), tt.want)
			})
		})
	}
}

func TestSelectCopiesColumns(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name       string
		selectCols func(...string) *SelectStmt
	}{
		{"constructor", Select},
		{"method", func(cols ...string) *SelectStmt { return Select().Select(cols...) }},
		{"set columns", func(cols ...string) *SelectStmt { return Select("old").SetColumns(cols...) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Spare capacity exposes writes to the caller's backing array.
			cols := []string{"a", "unused"}
			stmt := tt.selectCols(cols[:1]...).From("t").Select("b")
			if cols[1] != "unused" {
				t.Error("Select modified the caller's column slice")
			}
			cols[0] = "changed"
			assertBuild(t, stmt, buildResult{query: "SELECT a,b FROM t"})
		})
	}
}

func TestSelectClone(t *testing.T) {
	t.Parallel()
	t.Run("nil receiver", func(t *testing.T) {
		var s *SelectStmt
		if clone := s.Clone(); clone != nil {
			t.Fatalf("Clone() = %v, want nil", clone)
		}
	})

	t.Run("bound values remain shared", func(t *testing.T) {
		ids := []int{1, 2}
		s := Select().From("t")
		s.Where("id = ANY($)", ids)
		clone := s.Clone()
		ids[0] = 3
		assertBuild(t, clone, buildResult{
			query: "SELECT * FROM t WHERE (id = ANY($1))",
			args:  []any{[]int{3, 2}},
		})
	})
}

func TestSelectCloneIndependentChanges(t *testing.T) {
	t.Parallel()
	// Spare capacity exposes shared backing arrays when both statements append.
	source := &SelectStmt{
		cols:  make([]string, 0, 4),
		where: make([]WhereExpr, 0, 4),
		order: make([]Order, 0, 4),
	}
	source.Select("id").From("t").Asc("id", NullsLast).Limit(10).Offset(1)
	source.Apply(And(Not(Where("a > $", 2)), Where("b=$", 3)))
	filter := Where("c IS NULL")
	source.Apply(filter)
	clone := source.Clone()

	source.From("source").Select("source_col").Desc("source_order").Limit(20).Offset(2)
	source.Where("source_only=$", 4)

	clone.From("copy").Select("copy_col").Desc("copy_order").Limit(30).Offset(3)
	clone.Where("copy_only=$", 5)

	wantSource := buildResult{
		query: "SELECT id,source_col FROM source WHERE ((NOT (a > $1)) AND (b=$2)) AND (c IS NULL) AND (source_only=$3) ORDER BY id ASC NULLS LAST,source_order DESC LIMIT 20 OFFSET 2",
		args:  []any{2, 3, 4},
	}
	wantClone := buildResult{
		query: "SELECT id,copy_col FROM copy WHERE ((NOT (a > $1)) AND (b=$2)) AND (c IS NULL) AND (copy_only=$3) ORDER BY id ASC NULLS LAST,copy_order DESC LIMIT 30 OFFSET 3",
		args:  []any{2, 3, 5},
	}
	assertBuild(t, source, wantSource)
	assertBuild(t, clone, wantClone)
}

func TestSelectCloneConcurrentReuse(t *testing.T) {
	t.Parallel()
	base := Select("id").From("items").Asc("id")
	base.Where("a=$", 1)
	for i := range 16 {
		t.Run("clone", func(t *testing.T) {
			t.Parallel()
			clone := base.Clone().Select("title").Limit(10)
			clone.Where("b=$", i)
			clone.Where("active=$", true)
			assertBuild(t, clone, buildResult{
				query: "SELECT id,title FROM items WHERE (a=$1) AND (b=$2) AND (active=$3) ORDER BY id ASC LIMIT 10",
				args:  []any{1, i, true},
			})
			assertBuild(t, base, buildResult{
				query: "SELECT id FROM items WHERE (a=$1) ORDER BY id ASC",
				args:  []any{1},
			})
		})
	}
}

func TestSelectPaginationValues(t *testing.T) {
	t.Parallel()
	for _, api := range []struct {
		name  string
		get   func(*SelectStmt) (int, bool)
		set   func(*SelectStmt, int) *SelectStmt
		clear func(*SelectStmt) *SelectStmt
	}{
		{"offset", (*SelectStmt).OffsetValue, (*SelectStmt).Offset, (*SelectStmt).ClearOffset},
		{"limit", (*SelectStmt).LimitValue, (*SelectStmt).Limit, (*SelectStmt).ClearLimit},
	} {
		t.Run(api.name, func(t *testing.T) {
			check := func(s *SelectStmt, want int, wantSet bool) {
				t.Helper()
				if got, set := api.get(s); got != want || set != wantSet {
					t.Fatalf("value = (%d, %t), want (%d, %t)", got, set, want, wantSet)
				}
			}
			s := Select()
			check(s, 0, false)
			check(s.Clone(), 0, false)
			for _, n := range []int{0, 10, -1} {
				api.set(s, n)
				check(s, n, true)
				clone := s.Clone()
				check(clone, n, true)
				api.clear(clone)
				check(clone, 0, false)
				check(s, n, true)
				api.clear(s)
				check(s, 0, false)
			}
		})
	}
}

func TestSelectWhere(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		build func() *SelectStmt
		want  buildResult
	}{
		{
			name: "raw condition",
			build: func() *SelectStmt {
				s := Select().From("t")
				s.Where("true")
				return s
			},
			want: buildResult{query: "SELECT * FROM t WHERE (true)"},
		},
		{
			name: "negated raw condition",
			build: func() *SelectStmt {
				s := Select().From("t")
				s.WhereNot("true")
				return s
			},
			want: buildResult{query: "SELECT * FROM t WHERE (NOT (true))"},
		},
		{
			name: "multiple raw placeholders",
			build: func() *SelectStmt {
				s := Select().From("t1")
				s.WhereNot("c1 in ($,$,$)", 1, 2, 3).
					Where("c2 = any (select id from t2 where c <> $)", "4")
				return s
			},
			want: buildResult{
				query: "SELECT * FROM t1 WHERE (NOT (c1 in ($1,$2,$3))) AND (c2 = any (select id from t2 where c <> $4))",
				args:  []any{1, 2, 3, "4"},
			},
		},
		{
			name: "applied condition",
			build: func() *SelectStmt {
				return Select().From("t").Apply(Or(Where("c1=$", 1), Where("c2=$", 2)))
			},
			want: buildResult{query: "SELECT * FROM t WHERE ((c1=$1) OR (c2=$2))", args: []any{1, 2}},
		},
		{
			name:  "zero applied clause",
			build: func() *SelectStmt { return Select().From("t").Apply(WhereExpr{}) },
			want:  buildResult{err: "pql: WHERE condition is required"},
		},
		{
			name: "postfix operator on expression with placeholders",
			build: func() *SelectStmt {
				s := Select().From("t")
				s.Where("(select 1 from t2 where c1>$ and c2<$) IS NULL", 1, 2)
				return s
			},
			want: buildResult{
				query: "SELECT * FROM t WHERE ((select 1 from t2 where c1>$1 and c2<$2) IS NULL)",
				args:  []any{1, 2},
			},
		},
		{
			name: "binary operator on expression with placeholders",
			build: func() *SelectStmt {
				s := Select().From("t")
				s.Where("coalesce($,$)=$", 1, 2, 3)
				return s
			},
			want: buildResult{
				query: "SELECT * FROM t WHERE (coalesce($1,$2)=$3)",
				args:  []any{1, 2, 3},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt := tt.build()
			assertBuild(t, stmt, tt.want)
			t.Run("clone", func(t *testing.T) {
				assertBuild(t, stmt.Clone(), tt.want)
			})
		})
	}
}

func TestSelectSetWhere(t *testing.T) {
	t.Parallel()
	base := Select("id").From("t").Desc("id").Limit(5).Where("old=$", 99)
	stmt := base.Clone()
	exprs := []WhereExpr{Or(Where("a=$", 1), Where("b=$", 2)), Where("c=$", 3)}
	if stmt.SetWhere(exprs...) != stmt {
		t.Fatal("SetWhere must return its receiver")
	}
	exprs[0] = Where("changed")
	stmt.Apply(Where("d=$", 4)).Where("enabled").WhereNot("hidden")
	want := buildResult{
		query: "SELECT id FROM t WHERE ((a=$1) OR (b=$2)) AND (c=$3) AND (d=$4) AND (enabled) AND (NOT (hidden)) ORDER BY id DESC LIMIT 5",
		args:  []any{1, 2, 3, 4},
	}
	assertBuild(t, stmt, want)
	assertBuild(t, stmt.Clone(), want)
	assertBuild(t, base, buildResult{query: "SELECT id FROM t WHERE (old=$1) ORDER BY id DESC LIMIT 5", args: []any{99}})

	cleared := buildResult{query: "SELECT id FROM t ORDER BY id DESC LIMIT 5", args: nil}
	assertBuild(t, stmt.SetWhere(), cleared)
	assertBuild(t, stmt.Where("old").SetWhere([]WhereExpr(nil)...), cleared)
	assertBuild(t, stmt.Where("old").SetWhere([]WhereExpr{}...), cleared)
	assertBuild(t, stmt.SetWhere(Where("fresh=$", 5)), buildResult{query: "SELECT id FROM t WHERE (fresh=$1) ORDER BY id DESC LIMIT 5", args: []any{5}})
	for _, exprs := range [][]WhereExpr{{{}}, {Where("valid"), {}}} {
		assertBuild(t, stmt.SetWhere(exprs...), buildResult{err: "pql: WHERE condition is required"})
	}
	assertBuild(t, stmt.SetWhere(Where("")), buildResult{err: "pql: WHERE condition is required"})
	assertBuild(t, stmt.SetWhere(), cleared)
}
