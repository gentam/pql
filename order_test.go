package pql

import "testing"

func TestOrder(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		desc          bool
		nulls         []NullsOrder
		wantNullsLast bool
		want          string
	}{
		{name: "ascending default", wantNullsLast: true, want: "SELECT * FROM t ORDER BY c ASC"},
		{name: "descending default", desc: true, want: "SELECT * FROM t ORDER BY c DESC"},
		{name: "ascending explicit default", nulls: []NullsOrder{NullsDefault}, wantNullsLast: true, want: "SELECT * FROM t ORDER BY c ASC"},
		{name: "descending explicit default", desc: true, nulls: []NullsOrder{NullsDefault}, want: "SELECT * FROM t ORDER BY c DESC"},
		{name: "ascending nulls first", nulls: []NullsOrder{NullsFirst}, want: "SELECT * FROM t ORDER BY c ASC NULLS FIRST"},
		{name: "ascending nulls last", nulls: []NullsOrder{NullsLast}, wantNullsLast: true, want: "SELECT * FROM t ORDER BY c ASC NULLS LAST"},
		{name: "descending nulls first", desc: true, nulls: []NullsOrder{NullsFirst}, want: "SELECT * FROM t ORDER BY c DESC NULLS FIRST"},
		{name: "descending nulls last", desc: true, nulls: []NullsOrder{NullsLast}, wantNullsLast: true, want: "SELECT * FROM t ORDER BY c DESC NULLS LAST"},
		{name: "last option wins ascending", nulls: []NullsOrder{NullsFirst, NullsLast}, wantNullsLast: true, want: "SELECT * FROM t ORDER BY c ASC NULLS LAST"},
		{name: "last option wins descending", desc: true, nulls: []NullsOrder{NullsLast, NullsFirst}, want: "SELECT * FROM t ORDER BY c DESC NULLS FIRST"},
		{name: "reset ascending to default", nulls: []NullsOrder{NullsFirst, NullsDefault}, wantNullsLast: true, want: "SELECT * FROM t ORDER BY c ASC"},
		{name: "reset descending to default", desc: true, nulls: []NullsOrder{NullsLast, NullsDefault}, want: "SELECT * FROM t ORDER BY c DESC"},
		{name: "earlier invalid option ignored", nulls: []NullsOrder{NullsOrder(99), NullsFirst}, want: "SELECT * FROM t ORDER BY c ASC NULLS FIRST"},
		{name: "invalid option reset to default", desc: true, nulls: []NullsOrder{NullsOrder(99), NullsDefault}, want: "SELECT * FROM t ORDER BY c DESC"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := Order("c", tt.desc, tt.nulls...)
			if got := order.Expr(); got != "c" {
				t.Errorf("Expr() = %q, want %q", got, "c")
			}
			if got := order.Descending(); got != tt.desc {
				t.Errorf("Descending() = %t, want %t", got, tt.desc)
			}
			if got := order.NullsLast(); got != tt.wantNullsLast {
				t.Errorf("NullsLast() = %t, want %t", got, tt.wantNullsLast)
			}
			assertBuild(t, Select().From("t").SetOrders(order), buildResult{query: tt.want})
		})
	}
}

func TestOrderSnapshotsNulls(t *testing.T) {
	t.Parallel()
	nulls := []NullsOrder{NullsFirst, NullsLast}
	order := Order("c", true, nulls...)
	nulls[1] = NullsFirst

	assertBuild(t, Select().From("t").SetOrders(order), buildResult{
		query: "SELECT * FROM t ORDER BY c DESC NULLS LAST",
	})
}

func TestOrderTermComparison(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		a, b  OrderTerm
		equal bool
	}{
		{"same configuration", Asc("id"), Order("id", false), true},
		{"explicit default equals omitted", Desc("id", NullsDefault), Desc("id"), true},
		{"only final option matters", Asc("id", NullsFirst, NullsLast), Asc("id", NullsLast), true},
		{"ignored invalid option", Asc("id", NullsOrder(99), NullsDefault), Asc("id"), true},
		{"default differs from explicit last", Asc("id"), Asc("id", NullsLast), false},
		{"default differs from explicit first", Desc("id"), Desc("id", NullsFirst), false},
		{"different expressions", Asc("a"), Asc("b"), false},
		{"different directions", Asc("id"), Desc("id"), false},
		{"different null placement", Asc("id", NullsFirst), Asc("id", NullsLast), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a == tt.b; got != tt.equal {
				t.Errorf("%#v == %#v = %t, want %t", tt.a, tt.b, got, tt.equal)
			}
		})
	}
}

func TestOrderTermReversed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		order OrderTerm
		want  string
	}{
		{
			name:  "explicit null ordering",
			order: Asc("c", NullsFirst),
			want:  "SELECT * FROM t ORDER BY c DESC NULLS LAST",
		},
		{
			name:  "default null ordering",
			order: Asc("c"),
			want:  "SELECT * FROM t ORDER BY c DESC",
		},
		{
			name:  "descending explicit null ordering",
			order: Desc("c", NullsLast),
			want:  "SELECT * FROM t ORDER BY c ASC NULLS FIRST",
		},
		{
			name:  "descending explicit default",
			order: Desc("c", NullsDefault),
			want:  "SELECT * FROM t ORDER BY c ASC",
		},
		{
			name:  "reset to default",
			order: Asc("c", NullsFirst, NullsDefault),
			want:  "SELECT * FROM t ORDER BY c DESC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := tt.order
			reversed := tt.order.Reversed()
			if tt.order != original {
				t.Error("Reversed modified the original order")
			}
			if reversed.Descending() == tt.order.Descending() {
				t.Error("reversed order did not invert direction")
			}
			if reversed.NullsLast() == tt.order.NullsLast() {
				t.Error("reversed order did not invert NULL ordering")
			}
			if reversed.Reversed() != tt.order {
				t.Error("reversing twice did not restore the original order")
			}
			assertBuild(t, Select().From("t").SetOrders(reversed), buildResult{query: tt.want})
		})
	}
}

func TestSelectOrdersCopy(t *testing.T) {
	t.Parallel()
	orders := []OrderTerm{Asc("a")}
	stmt := Select().From("t").SetOrders(orders...)

	orders[0] = Desc("b")
	got := stmt.Orders()
	got[0] = Desc("c")

	assertBuild(t, stmt, buildResult{query: "SELECT * FROM t ORDER BY a ASC"})
	assertBuild(t, stmt.SetOrders(got...), buildResult{query: "SELECT * FROM t ORDER BY c DESC"})
}

func TestSelectOrderBy(t *testing.T) {
	t.Parallel()
	stmt := Select().From("t")
	if stmt.OrderBy() != stmt {
		t.Fatal("OrderBy must return its receiver")
	}
	assertBuild(t, stmt.OrderBy([]OrderTerm(nil)...), buildResult{query: "SELECT * FROM t"})
	terms := []OrderTerm{Desc("score", NullsLast), Asc("id")}
	stmt.SetOrders(Asc("tenant"))
	if stmt.OrderBy(terms...) != stmt {
		t.Fatal("OrderBy must return its receiver")
	}
	terms[0] = Asc("changed")
	stmt.OrderBy(Desc("created_at")).Order("name", false)
	want := buildResult{query: "SELECT * FROM t ORDER BY tenant ASC,score DESC NULLS LAST,id ASC,created_at DESC,name ASC"}
	assertBuild(t, stmt, want)
	assertBuild(t, stmt.OrderBy().OrderBy([]OrderTerm(nil)...).OrderBy([]OrderTerm{}...), want)
	assertBuild(t, stmt.Clone().SetOrders(Desc("replacement")), buildResult{query: "SELECT * FROM t ORDER BY replacement DESC"})
	assertBuild(t, stmt.Clone().SetOrders(), buildResult{query: "SELECT * FROM t"})
	assertBuild(t, stmt, want)
	for _, invalid := range []OrderTerm{{}, Asc("id", NullsOrder(99)), Desc("id", NullsFirst, NullsOrder(99))} {
		if _, _, err := Select().OrderBy(Asc("valid"), invalid).Build(); err == nil {
			t.Error("OrderBy must preserve invalid terms until Build")
		}
	}
}

func TestReversedOrders(t *testing.T) {
	t.Parallel()
	orders := []OrderTerm{Desc("due_date", NullsLast), Asc("id")}
	reversed := ReversedOrders(orders)
	assertBuild(t, Select().From("t").SetOrders(reversed...), buildResult{
		query: "SELECT * FROM t ORDER BY due_date ASC NULLS FIRST,id DESC",
	})
	assertBuild(t, Select().From("t").SetOrders(ReversedOrders(reversed)...), buildResult{
		query: "SELECT * FROM t ORDER BY due_date DESC NULLS LAST,id ASC",
	})
	reversed[0] = Asc("other")
	assertBuild(t, Select().From("t").SetOrders(orders...), buildResult{
		query: "SELECT * FROM t ORDER BY due_date DESC NULLS LAST,id ASC",
	})
	if ReversedOrders(nil) != nil {
		t.Error("ReversedOrders(nil) must return nil")
	}
	assertBuild(t, Select().SetOrders(ReversedOrders([]OrderTerm{})...), buildResult{
		query: "SELECT *",
	})
	for _, invalid := range []OrderTerm{Order("", false), Asc("id", NullsOrder(99)), Desc("id", NullsFirst, NullsOrder(99))} {
		if _, _, err := Select().SetOrders(ReversedOrders([]OrderTerm{invalid})...).Build(); err == nil {
			t.Error("reversing an invalid order must preserve the build error")
		}
	}
}
