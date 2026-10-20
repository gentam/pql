package pql

import (
	"reflect"
	"testing"
)

func TestWhereJSONOperators(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		expr string
		arg  any
		want string
	}{
		{"key", "payload ? $", "key", "payload ? $1"},
		{"any key", "payload ?| $", []string{"a", "b"}, "payload ?| $1"},
		{"all keys", "payload ?& $", []string{"a", "b"}, "payload ?& $1"},
		{"path", "payload @? $", "$.a[*] ? (@ > 2)", "payload @? $1"},
		{"literal key with bound condition", "payload ? 'key' AND id = $", 1, "payload ? 'key' AND id = $1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stmt := Select().From("t").Apply(Where(tt.expr, tt.arg))
			assertBuild(t, stmt, buildResult{
				query: "SELECT * FROM t WHERE (" + tt.want + ")",
				args:  []any{tt.arg},
			})
		})
	}
}

func TestWherePlaceholderArgumentLists(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		expr string
		args []any
		want buildResult
	}{
		{"nil bound value", `id=$`, []any{nil}, buildResult{query: `SELECT * FROM t WHERE (id=$1)`, args: []any{nil}}},
		{"numbered placeholder", `id=$1`, []any{1}, buildResult{err: "pql: numbered placeholders are not supported in WHERE expressions; use $"}},
		{"named placeholder", `id=$foo`, []any{1}, buildResult{err: "pql: invalid WHERE placeholder; use $"}},
		{"unterminated string after placeholder", `id=$ AND body='$`, []any{1}, buildResult{err: "pql: unterminated string in WHERE expression"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stmt := Select().From("t").Apply(Where(tt.expr, tt.args...))
			assertBuild(t, stmt, tt.want)
			assertBuild(t, stmt.Clone(), tt.want)
		})
	}
}

func TestWhereWithoutArguments(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		expr string
		want buildResult
	}{
		{"raw condition", `active`, buildResult{query: `SELECT * FROM t WHERE (active)`}},
		{"protected dollars", `"$col" = '$' AND price$usd = $tag$$1 $name$tag$ /* $ */`, buildResult{query: `SELECT * FROM t WHERE ("$col" = '$' AND price$usd = $tag$$1 $name$tag$ /* $ */)`}},
		{"missing argument", `body='$' AND id=$`, buildResult{err: "pql: WHERE placeholder count 1 does not match argument count 0"}},
		{"numbered placeholder", `id=$1`, buildResult{err: "pql: numbered placeholders are not supported in WHERE expressions; use $"}},
		{"named placeholder", `id=$foo`, buildResult{err: "pql: invalid WHERE placeholder; use $"}},
		{"unterminated string", `body='text`, buildResult{err: "pql: unterminated string in WHERE expression"}},
		{"unterminated identifier", `"id`, buildResult{err: "pql: unterminated quoted identifier in WHERE expression"}},
		{"unterminated dollar quote", `body=$tag$text`, buildResult{err: "pql: unterminated dollar-quoted string in WHERE expression"}},
		{"unterminated block comment", `true /*`, buildResult{err: "pql: unterminated block comment in WHERE expression"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for _, call := range []struct {
				name string
				w    WhereExpr
			}{
				{"omitted", Where(tt.expr)},
				{"nil slice", Where(tt.expr, []any(nil)...)},
				{"empty slice", Where(tt.expr, []any{}...)},
			} {
				t.Run(call.name, func(t *testing.T) {
					t.Parallel()
					stmt := Select().From("t").Apply(call.w)
					assertBuild(t, stmt, tt.want)
					assertBuild(t, stmt.Clone(), tt.want)
				})
			}
		})
	}
}

func TestWhereTrailingLineComments(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		w    WhereExpr
		want buildResult
	}{
		{"bound argument", Where("a=$ -- comment", 1), buildResult{query: "SELECT * FROM t WHERE (a=$1 -- comment\n)", args: []any{1}}},
		{"nil arguments", Where("true -- comment"), buildResult{query: "SELECT * FROM t WHERE (true -- comment\n)"}},
		{"empty arguments", Where("true -- comment", []any{}...), buildResult{query: "SELECT * FROM t WHERE (true -- comment\n)"}},
		{"native parameter", Where("a=$1 -- comment"), buildResult{err: "pql: numbered placeholders are not supported in WHERE expressions; use $"}},
		{"binary operator", Where("coalesce($, 0)=$ -- comment", 1, 1), buildResult{query: "SELECT * FROM t WHERE (coalesce($1, 0)=$2 -- comment\n)", args: []any{1, 1}}},
		{"postfix operator", Where("a IS NULL -- comment"), buildResult{query: "SELECT * FROM t WHERE (a IS NULL -- comment\n)"}},
		{"and chain", And(Where("a=$ -- first", 1), Where("b -- last")), buildResult{query: "SELECT * FROM t WHERE ((a=$1 -- first\n) AND (b -- last\n))", args: []any{1}}},
		{"or chain", Or(Where("a=$ -- first", 1), Where("b -- last")), buildResult{query: "SELECT * FROM t WHERE ((a=$1 -- first\n) OR (b -- last\n))", args: []any{1}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stmt := Select().From("t").Apply(tt.w)
			assertBuild(t, stmt, tt.want)
			assertBuild(t, stmt.Clone(), tt.want)
		})
	}
	t.Run("negation and following condition", func(t *testing.T) {
		t.Parallel()
		stmt := Select().From("t").Asc("id").Limit(2)
		stmt.WhereNot("a=$ -- first", 1).Where("b=$ -- last", 2)
		assertBuild(t, stmt, buildResult{
			query: "SELECT * FROM t WHERE (NOT (a=$1 -- first\n)) AND (b=$2 -- last\n) ORDER BY id ASC LIMIT 2",
			args:  []any{1, 2},
		})
	})
}

func TestWhereComposition(t *testing.T) {
	t.Parallel()
	a, b, c := Where("a=$", 1), Where("b=$", 2), Where("c=$", 3)
	for _, tt := range []struct {
		name  string
		w     WhereExpr
		query string
		args  []any
	}{
		{"and", And(a, b, c), "(a=$1) AND (b=$2) AND (c=$3)", []any{1, 2, 3}},
		{"or", Or(a, b, c), "(a=$1) OR (b=$2) OR (c=$3)", []any{1, 2, 3}},
		{"and with or", And(a, Or(b, c)), "(a=$1) AND ((b=$2) OR (c=$3))", []any{1, 2, 3}},
		{"or with and", Or(And(a, b), c), "((a=$1) AND (b=$2)) OR (c=$3)", []any{1, 2, 3}},
		{"negate whole group", Not(Or(a, b)), "NOT ((a=$1) OR (b=$2))", []any{1, 2}},
		{"double negation", Not(Not(a)), "NOT (NOT (a=$1))", []any{1}},
		{"raw operand grouping", And(Where("a OR b"), Where("c OR d")), "(a OR b) AND (c OR d)", nil},
		{"raw negation", WhereNot("a=$ OR b=$", 1, 2), "NOT (a=$1 OR b=$2)", []any{1, 2}},
		{"empty and", And(), "TRUE", nil},
		{"empty or", Or(), "FALSE", nil},
		{"nil slice and", And([]WhereExpr(nil)...), "TRUE", nil},
		{"empty slice or", Or([]WhereExpr{}...), "FALSE", nil},
		{"single and", And(a), "(a=$1)", []any{1}},
		{"single or", Or(a), "(a=$1)", []any{1}},
		{"repeated operand", And(a, Not(a)), "(a=$1) AND (NOT (a=$2))", []any{1, 1}},
		{"null value", WhereNot("a=$", nil), "NOT (a=$1)", []any{nil}},
		{"range", And(Where("n >= $", 18), Where("n < $", 65)), "(n >= $1) AND (n < $2)", []any{18, 65}},
		{"method and", a.And(b, c), "(a=$1) AND (b=$2) AND (c=$3)", []any{1, 2, 3}},
		{"method or", a.Or(b, c), "(a=$1) OR (b=$2) OR (c=$3)", []any{1, 2, 3}},
		{"method chain", a.And(b).Or(c), "((a=$1) AND (b=$2)) OR (c=$3)", []any{1, 2, 3}},
		{"method nested", a.And(b.Or(c)), "(a=$1) AND ((b=$2) OR (c=$3))", []any{1, 2, 3}},
		{"method negate whole group", a.Or(b).Not(), "NOT ((a=$1) OR (b=$2))", []any{1, 2}},
		{"method double negation", a.Not().Not(), "NOT (NOT (a=$1))", []any{1}},
		{"method empty and", a.And(), "(a=$1)", []any{1}},
		{"method empty or", a.Or(), "(a=$1)", []any{1}},
		{"method nil slice", a.And([]WhereExpr(nil)...), "(a=$1)", []any{1}},
		{"method empty slice", a.Or([]WhereExpr{}...), "(a=$1)", []any{1}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stmt := Select().From("t").Apply(tt.w)
			want := buildResult{query: "SELECT * FROM t WHERE (" + tt.query + ")", args: tt.args}
			assertBuild(t, stmt, want)
			assertBuild(t, stmt.Clone(), want)
		})
	}
}

func TestWhereCompositionValidation(t *testing.T) {
	t.Parallel()
	var zeroExpr WhereExpr
	for _, tt := range []struct {
		name string
		w    WhereExpr
		err  string
	}{
		{"empty WhereNot", WhereNot(""), "pql: WHERE condition is required"},
		{"WhereNot missing argument", WhereNot("a=$"), "pql: WHERE placeholder count 1 does not match argument count 0"},
		{"zero and", And(WhereExpr{}), "pql: WHERE condition is required"},
		{"zero or", Or(Where("true"), WhereExpr{}), "pql: WHERE condition is required"},
		{"zero not", Not(WhereExpr{}), "pql: WHERE condition is required"},
		{"method zero and receiver", zeroExpr.And(), "pql: WHERE condition is required"},
		{"method zero or receiver", zeroExpr.Or(Where("true")), "pql: WHERE condition is required"},
		{"method zero not receiver", zeroExpr.Not(), "pql: WHERE condition is required"},
		{"method zero and input", Where("true").And(WhereExpr{}), "pql: WHERE condition is required"},
		{"method zero or input", Where("false").Or(WhereExpr{}), "pql: WHERE condition is required"},
		{"method invalid receiver", Where("").Not(), "pql: WHERE condition is required"},
		{"nested zero", And(Where("a=$", 1), Not(Or(WhereExpr{}))), "pql: WHERE condition is required"},
		{"zero value", WhereExpr{}, "pql: WHERE condition is required"},
		{"empty leaf", Not(Where(" \t")), "pql: WHERE condition is required"},
		{"missing value", Or(Where("true"), Where("b=$")), "pql: WHERE placeholder count 1 does not match argument count 0"},
		{"invalid keyset", And(Or(), After(nil)), "pql: keyset requires at least one order"},
		{"unterminated leaf", And(Where("a=$", 1), Where("b='")), "pql: unterminated string in WHERE expression"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, stmt := range []Builder{
				Select().From("t").Apply(tt.w),
				Update("t").Set("x", 0).Apply(tt.w),
				Delete("t").Apply(tt.w),
			} {
				assertBuild(t, stmt, buildResult{err: tt.err})
			}
		})
	}
}

func TestWhereCompositionIndependentReuse(t *testing.T) {
	t.Parallel()
	args := []any{1, "unused"}
	base := Where("tenant=$", args[:1]...)
	unused := Where("unused")
	children := []WhereExpr{base, unused}
	and := And(children[:1]...)
	or := Or(children[:1]...)
	negated := Not(base)
	args[0] = 99
	children[0] = Where("changed")
	applied := Select().Apply(and)
	derived := And(and, Where("active=$", true))
	if args[1] != "unused" || !reflect.DeepEqual(children[1], unused) {
		t.Fatal("construction modified caller-owned slices")
	}

	assertBuild(t, Select().Apply(base), buildResult{query: "SELECT * WHERE (tenant=$1)", args: []any{1}})
	assertBuild(t, applied, buildResult{query: "SELECT * WHERE ((tenant=$1))", args: []any{1}})
	assertBuild(t, Select().Apply(or), buildResult{query: "SELECT * WHERE ((tenant=$1))", args: []any{1}})
	assertBuild(t, Select().Apply(negated), buildResult{query: "SELECT * WHERE (NOT (tenant=$1))", args: []any{1}})
	assertBuild(t, Select().Apply(derived), buildResult{query: "SELECT * WHERE (((tenant=$1)) AND (active=$2))", args: []any{1, true}})

	// A Build result is owned by the caller, not the reusable expression.
	s := Select().Apply(derived)
	_, builtArgs, err := s.Build()
	if err != nil {
		t.Fatal(err)
	}
	builtArgs[0] = 100
	assertBuild(t, s, buildResult{query: "SELECT * WHERE (((tenant=$1)) AND (active=$2))", args: []any{1, true}})
}

func TestWhereBoundValuesRemainShared(t *testing.T) {
	t.Parallel()
	ids := []int{1, 2}
	w := And(Where("id = ANY($)", ids), Not(Where("deleted")))
	ids[0] = 3
	assertBuild(t, Select().Apply(w), buildResult{
		query: "SELECT * WHERE ((id = ANY($1)) AND (NOT (deleted)))", args: []any{[]int{3, 2}},
	})
}

func TestWhereConcurrentComposition(t *testing.T) {
	t.Parallel()
	base := And(Where("a=$", 1), Not(Where("b=$", 2)))
	shared := Select().From("t").Apply(base)
	for i := range 16 {
		t.Run("reuse", func(t *testing.T) {
			t.Parallel()
			derived := base.Or(Where("c=$", i))
			assertBuild(t, Select().From("t").Apply(derived), buildResult{
				query: "SELECT * FROM t WHERE (((a=$1) AND (NOT (b=$2))) OR (c=$3))", args: []any{1, 2, i},
			})
			assertBuild(t, Update("t").Set("x", 0).Apply(base).Where("c=$", i), buildResult{
				query: "UPDATE t SET x=$1 WHERE ((a=$2) AND (NOT (b=$3))) AND (c=$4)", args: []any{0, 1, 2, i},
			})
			assertBuild(t, Delete("t").Apply(base), buildResult{
				query: "DELETE FROM t WHERE ((a=$1) AND (NOT (b=$2)))", args: []any{1, 2},
			})
			assertBuild(t, shared, buildResult{query: "SELECT * FROM t WHERE ((a=$1) AND (NOT (b=$2)))", args: []any{1, 2}})
			assertBuild(t, shared.Clone(), buildResult{query: "SELECT * FROM t WHERE ((a=$1) AND (NOT (b=$2)))", args: []any{1, 2}})
		})
	}
}

func TestWhereProtectedPlaceholders(t *testing.T) {
	t.Parallel()
	w := And(
		Where(`payload ? '$' AND "price$" > $ /* $ */`, 10),
		Not(Where(`coalesce(price$usd, $tag$$tag$) || $ = $`, "USD", "JPY")),
	)
	assertBuild(t, Update("t").Set("active", true).Apply(w), buildResult{
		query: `UPDATE t SET active=$1 WHERE ((payload ? '$' AND "price$" > $2 /* $ */) AND (NOT (coalesce(price$usd, $tag$$tag$) || $3 = $4)))`,
		args:  []any{true, 10, "USD", "JPY"},
	})
}

func TestStatementWhereReturnsReceiver(t *testing.T) {
	t.Parallel()
	s := Select("id").From("t")
	if s.Where("a=$", 1) != s {
		t.Fatal("Select.Where did not return its receiver")
	}
	assertBuild(t, s.Where("b=$", 2).Limit(1), buildResult{query: "SELECT id FROM t WHERE (a=$1) AND (b=$2) LIMIT 1", args: []any{1, 2}})
	u := Update("t").Set("x", 0)
	if u.Where("a=$", 1) != u {
		t.Fatal("Update.Where did not return its receiver")
	}
	assertBuild(t, u.Where("b=$", 2).Returning("id"), buildResult{query: "UPDATE t SET x=$1 WHERE (a=$2) AND (b=$3) RETURNING id", args: []any{0, 1, 2}})
	d := Delete("t")
	if d.Where("a=$", 1) != d {
		t.Fatal("Delete.Where did not return its receiver")
	}
	assertBuild(t, d.Where("b=$", 2).Returning("id"), buildResult{query: "DELETE FROM t WHERE (a=$1) AND (b=$2) RETURNING id", args: []any{1, 2}})
}

func TestStatementApplyMultiple(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		setup func() (Builder, func(...WhereExpr) Builder)
		base  buildResult
		want  buildResult
	}{
		{
			name: "select",
			setup: func() (Builder, func(...WhereExpr) Builder) {
				s := Select().From("t")
				return s, func(exprs ...WhereExpr) Builder { return s.Apply(exprs...) }
			},
			base: buildResult{query: "SELECT * FROM t"},
			want: buildResult{query: "SELECT * FROM t WHERE (tenant=$1) AND (a=$2 OR b=$3) AND (enabled) AND (NOT (hidden))", args: []any{7, 1, 2}},
		},
		{
			name: "update",
			setup: func() (Builder, func(...WhereExpr) Builder) {
				s := Update("t").Set("x", 0)
				return s, func(exprs ...WhereExpr) Builder { return s.Apply(exprs...) }
			},
			base: buildResult{query: "UPDATE t SET x=$1", args: []any{0}},
			want: buildResult{query: "UPDATE t SET x=$1 WHERE (tenant=$2) AND (a=$3 OR b=$4) AND (enabled) AND (NOT (hidden))", args: []any{0, 7, 1, 2}},
		},
		{
			name: "delete",
			setup: func() (Builder, func(...WhereExpr) Builder) {
				s := Delete("t")
				return s, func(exprs ...WhereExpr) Builder { return s.Apply(exprs...) }
			},
			base: buildResult{query: "DELETE FROM t"},
			want: buildResult{query: "DELETE FROM t WHERE (tenant=$1) AND (a=$2 OR b=$3) AND (enabled) AND (NOT (hidden))", args: []any{7, 1, 2}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stmt, apply := tt.setup()
			for _, empty := range [][]WhereExpr{nil, {}} {
				if apply(empty...) != stmt {
					t.Fatal("Apply must return its receiver")
				}
				assertBuild(t, stmt, tt.base)
			}
			apply(Where("tenant=$", 7))
			exprs := []WhereExpr{Where("a=$ OR b=$", 1, 2), Where("enabled")}
			if apply(exprs...) != stmt {
				t.Fatal("Apply must return its receiver")
			}
			exprs[0] = Where("changed")
			apply(WhereNot("hidden"))
			assertBuild(t, stmt, tt.want)
			assertBuild(t, apply(), tt.want)
			assertBuild(t, apply([]WhereExpr(nil)...), tt.want)
			assertBuild(t, apply([]WhereExpr{}...), tt.want)
			assertBuild(t, apply(Where("valid"), WhereExpr{}), buildResult{err: "pql: WHERE condition is required"})
		})
	}
}

func TestWhereExprMethodsIndependentReuse(t *testing.T) {
	t.Parallel()
	base := Where("tenant=$", 1)
	applied := Select().Apply(base)
	for _, tt := range []struct {
		name    string
		compose func(...WhereExpr) WhereExpr
		op      string
	}{
		{"and", base.And, "AND"},
		{"or", base.Or, "OR"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			extra, unused := Where("active=$", true), Where("unused")
			inputs := []WhereExpr{extra, unused, unused}
			derived := tt.compose(inputs[:1]...)
			// Supplying a slice with spare capacity must not change any part of it.
			if !reflect.DeepEqual(inputs, []WhereExpr{extra, unused, unused}) {
				t.Fatal("composition changed the caller's operand slice")
			}
			inputs[0] = Where("changed")
			want := buildResult{
				query: "SELECT * WHERE ((tenant=$1) " + tt.op + " (active=$2))",
				args:  []any{1, true},
			}
			assertBuild(t, Select().Apply(derived), want)
			negated := derived.Not()
			assertBuild(t, Select().Apply(negated), buildResult{
				query: "SELECT * WHERE (NOT ((tenant=$1) " + tt.op + " (active=$2)))",
				args:  []any{1, true},
			})
			assertBuild(t, Select().Apply(derived), want)
			assertBuild(t, applied, buildResult{query: "SELECT * WHERE (tenant=$1)", args: []any{1}})
			assertBuild(t, Select().Apply(extra), buildResult{query: "SELECT * WHERE (active=$1)", args: []any{true}})
		})
	}
}

func TestWhereValueCopies(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		expr WhereExpr
		sql  string
		args []any
	}{
		{"raw", Where("a=$", 1), "a=$1", []any{1}},
		{"composed", Where("a=$", 1).Or(Where("b=$", 2)), "(a=$1) OR (b=$2)", []any{1, 2}},
		{"keyset", After([]OrderTerm{Asc("id")}, 10), "((id) >= $1 AND $1 IS NOT NULL OR (id) IS NULL) AND (id) IS DISTINCT FROM $1", []any{10}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			original := tt.expr
			copied := original
			applied := Select().Apply(original)
			replaced := Select().SetWhere(original)
			composed := And(original)
			negate := original.Not
			// Reassigning a value cannot change copies, captured receivers, or statements.
			original = Where("changed")
			assertBuild(t, Select().Apply(original), buildResult{query: "SELECT * WHERE (changed)"})
			want := buildResult{query: "SELECT * WHERE (" + tt.sql + ")", args: tt.args}
			assertBuild(t, Select().Apply(copied), want)
			assertBuild(t, applied, want)
			assertBuild(t, replaced, want)
			assertBuild(t, Select().Apply(composed), buildResult{query: "SELECT * WHERE ((" + tt.sql + "))", args: tt.args})
			assertBuild(t, Select().Apply(negate()), buildResult{query: "SELECT * WHERE (NOT (" + tt.sql + "))", args: tt.args})
		})
	}
}
