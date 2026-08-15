package pql

import "testing"

func TestOrder(t *testing.T) {
	tests := []struct {
		name          string
		order         Order
		wantNullsLast bool
	}{
		{name: "ascending default", order: Asc("c"), wantNullsLast: true},
		{name: "descending default", order: Desc("c"), wantNullsLast: false},
		{name: "ascending nulls first", order: Asc("c", NullsFirst), wantNullsLast: false},
		{name: "ascending nulls last", order: Asc("c", NullsLast), wantNullsLast: true},
		{name: "descending nulls first", order: Desc("c", NullsFirst), wantNullsLast: false},
		{name: "descending nulls last", order: Desc("c", NullsLast), wantNullsLast: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.order.Column(); got != "c" {
				t.Errorf("Column() = %q, want %q", got, "c")
			}
			if got := tt.order.NullsLast(); got != tt.wantNullsLast {
				t.Errorf("NullsLast() = %t, want %t", got, tt.wantNullsLast)
			}
		})
	}
}

func TestOrderReversed(t *testing.T) {
	tests := []struct {
		name  string
		order Order
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reversed := tt.order.Reversed()
			if tt.order.IsDescending() {
				t.Error("Reversed modified the original order")
			}
			if !reversed.IsDescending() {
				t.Error("reversed order is not descending")
			}
			if reversed.NullsLast() == tt.order.NullsLast() {
				t.Error("reversed order did not invert NULL ordering")
			}
			assertBuild(t, Select().From("t").SetOrders(reversed), buildResult{query: tt.want})
		})
	}
}

func TestSelectOrdersCopy(t *testing.T) {
	orders := []Order{Asc("a")}
	stmt := Select().From("t").SetOrders(orders...)

	orders[0] = Desc("b")
	got := stmt.Orders()
	got[0] = Desc("c")

	assertBuild(t, stmt, buildResult{query: "SELECT * FROM t ORDER BY a ASC"})
	assertBuild(t, stmt.SetOrders(got...), buildResult{query: "SELECT * FROM t ORDER BY c DESC"})
}
