package pql

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// NullsOrder specifies where NULL values appear in an ORDER BY term.
type NullsOrder uint8

const (
	// NullsDefault omits the NULLS clause and uses PostgreSQL's default ordering.
	NullsDefault NullsOrder = iota
	// NullsFirst emits NULLS FIRST.
	NullsFirst
	// NullsLast emits NULLS LAST.
	NullsLast
)

// OrderTerm is an immutable, comparable value representing one ORDER BY term:
// an expression, a sort direction, and NULL placement.
// Create a term with [Order], [Asc], or [Desc].
//
// Ordering functions and methods use the last supplied NULL placement option;
// earlier options are ignored. If omitted or [NullsDefault], PostgreSQL's defaults
// apply: NULLS LAST for ascending and NULLS FIRST for descending order.
// Empty or whitespace-only expressions and invalid final options are reported
// when the statement is built. The zero value is invalid.
//
// Values compare equal when their expression, direction, and selected NULL
// placement match. Omitted placement equals NullsDefault, but differs from
// explicit NullsFirst or NullsLast even when the effective ordering is the same.
type OrderTerm struct {
	expr  string
	desc  bool
	nulls NullsOrder
}

// Order creates an order for expr, descending when desc is true and ascending
// otherwise. The last NULL placement option wins; see [OrderTerm] for defaults
// and validation.
func Order(expr string, desc bool, nulls ...NullsOrder) OrderTerm {
	o := OrderTerm{expr: expr, desc: desc}
	if len(nulls) != 0 {
		o.nulls = nulls[len(nulls)-1]
	}
	return o
}

// Asc creates an ascending order for expr.
// NULLs appear last by default. See [OrderTerm] for NULL placement and validation.
func Asc(expr string, nulls ...NullsOrder) OrderTerm {
	return Order(expr, false, nulls...)
}

// Desc creates a descending order for expr.
// NULLs appear first by default. See [OrderTerm] for NULL placement and validation.
func Desc(expr string, nulls ...NullsOrder) OrderTerm {
	return Order(expr, true, nulls...)
}

// Expr returns the ORDER BY expression.
func (o OrderTerm) Expr() string {
	return o.expr
}

// Descending reports whether the order is descending.
func (o OrderTerm) Descending() bool {
	return o.desc
}

// NullsLast reports the effective NULL ordering, including PostgreSQL's
// default of NULLS LAST for ascending and NULLS FIRST for descending order.
func (o OrderTerm) NullsLast() bool {
	if o.nulls == NullsDefault {
		return !o.desc
	}
	return o.nulls == NullsLast
}

// Reversed returns the exact inverse ordering without modifying o.
func (o OrderTerm) Reversed() OrderTerm {
	o.desc = !o.desc
	switch o.nulls {
	case NullsFirst:
		o.nulls = NullsLast
	case NullsLast:
		o.nulls = NullsFirst
	}
	return o
}

// ReversedOrders returns a copy with each term's direction and NULL placement
// reversed. It preserves the terms' priority, without modifying orders.
// A nil input returns nil. Invalid terms remain invalid when built.
func ReversedOrders(orders []OrderTerm) []OrderTerm {
	reversed := slices.Clone(orders)
	for i, order := range reversed {
		reversed[i] = order.Reversed()
	}
	return reversed
}

func buildOrder(b *strings.Builder, orders []OrderTerm) error {
	b.WriteString(" ORDER BY ")
	for i, order := range orders {
		if err := order.validate(); err != nil {
			return err
		}
		if i != 0 {
			b.WriteByte(',')
		}
		b.WriteString(terminateLineComment(order.expr))
		if order.desc {
			b.WriteString(" DESC")
		} else {
			b.WriteString(" ASC")
		}

		switch order.nulls {
		case NullsFirst:
			b.WriteString(" NULLS FIRST")
		case NullsLast:
			b.WriteString(" NULLS LAST")
		}
	}
	return nil
}

func (o OrderTerm) validate() error {
	if strings.TrimSpace(o.expr) == "" {
		return errors.New("pql: ORDER BY expression is required")
	}
	switch o.nulls {
	case NullsDefault, NullsFirst, NullsLast:
	default:
		return fmt.Errorf("pql: invalid NULLS order %d", o.nulls)
	}
	return nil
}
