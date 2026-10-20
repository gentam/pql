package pql

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// After creates a condition matching rows strictly after values in
// orders. Supply one value per order, including a unique tie-breaker for paging.
// Mixed directions and NULL placement are respected; nil represents SQL NULL.
//
// Values must be encodable by pgx for the corresponding SQL expression's type.
// For example, use time.Time or pgtype.Date for a date, converting serialized
// cursor strings before calling this function. pql does not decode cursor values.
//
// Orders must contain scalar SQL expressions usable in WHERE, not SELECT aliases or
// output column numbers. The condition does not change a statement's ORDER BY.
//
// Order and value slices are copied; bound values are not deep-copied.
// Invalid orders and value counts are reported when the receiving statement is built.
func After(orders []OrderTerm, values ...any) WhereExpr {
	return newKeyset(orders, values, false, false)
}

// Before is like [After] but matches rows strictly before values in orders.
func Before(orders []OrderTerm, values ...any) WhereExpr {
	return newKeyset(orders, values, true, false)
}

// AtOrAfter is like [After] but also matches rows equal to all cursor values.
// Two SQL NULLs compare equal when determining the boundary.
func AtOrAfter(orders []OrderTerm, values ...any) WhereExpr {
	return newKeyset(orders, values, false, true)
}

// AtOrBefore is like [Before] but also matches rows equal to all cursor values.
// Two SQL NULLs compare equal when determining the boundary.
func AtOrBefore(orders []OrderTerm, values ...any) WhereExpr {
	return newKeyset(orders, values, true, true)
}

type keysetExpr struct {
	orders    []OrderTerm
	values    []any
	before    bool
	inclusive bool
}

func newKeyset(orders []OrderTerm, values []any, before, inclusive bool) WhereExpr {
	return WhereExpr{keyset: &keysetExpr{
		orders:    slices.Clone(orders),
		values:    slices.Clone(values),
		before:    before,
		inclusive: inclusive,
	}}
}

func (k *keysetExpr) build(args []any) (string, []any, error) {
	if len(k.orders) == 0 {
		return "", nil, errors.New("pql: keyset requires at least one order")
	}
	if len(k.orders) != len(k.values) {
		return "", nil, fmt.Errorf("pql: keyset order count %d does not match value count %d", len(k.orders), len(k.values))
	}
	for _, order := range k.orders {
		if err := order.validate(); err != nil {
			return "", nil, err
		}
	}

	// Reuse each bound parameter in comparisons and NULL checks. Its comparison
	// with the column lets PostgreSQL infer the type even for a NULL cursor value.
	start := len(args)
	args = append(args, k.values...)
	b := &strings.Builder{}
	k.buildRowBound(b, start)
	for i, order := range k.orders {
		expr := "(" + terminateLineComment(order.expr) + ")"
		arg := "$" + strconv.Itoa(start+i+1)
		op := ">="
		if order.desc != k.before {
			op = "<="
		}
		nonnull, nullable := arg, expr
		if order.NullsLast() == k.before {
			nonnull, nullable = expr, arg
		}
		// First restrict this key to the inclusive range. This exposes a leading
		// range comparison to the planner while handling NULLs as ordered values.
		fmt.Fprintf(b, "(%s %s %s AND %s IS NOT NULL OR %s IS NULL)", expr, op, arg, nonnull, nullable)
		if i < len(k.orders)-1 {
			// A strictly different key is enough; equal keys defer to the suffix.
			fmt.Fprintf(b, " AND (%s IS DISTINCT FROM %s OR (", expr, arg)
		} else if !k.inclusive {
			fmt.Fprintf(b, " AND %s IS DISTINCT FROM %s", expr, arg)
		}
	}
	b.WriteString(strings.Repeat("))", len(k.orders)-1))
	return b.String(), args, nil
}

// buildRowBound adds a redundant bound that PostgreSQL can use to seek into a
// multicolumn index, rather than filtering all equal values of the leading key.
// A row comparison is safe for the same-direction prefix whose NULLs precede
// the requested range. The full predicate below still handles NULLs and ties.
func (k *keysetExpr) buildRowBound(b *strings.Builder, start int) {
	n := 0
	for _, order := range k.orders {
		if order.desc != k.orders[0].desc || order.NullsLast() != k.before {
			break
		}
		n++
	}
	if n < 2 {
		return
	}
	exprs, args := make([]string, n), make([]string, n)
	for i, order := range k.orders[:n] {
		exprs[i] = "(" + terminateLineComment(order.expr) + ")"
		args[i] = "$" + strconv.Itoa(start+i+1)
	}
	// Keep the bound inclusive even for strict paging: the full predicate removes
	// the cursor itself. This also avoids underestimating a large equality group
	// as an almost-empty strict range on the leading key.
	op := ">="
	if k.orders[0].desc != k.before {
		op = "<="
	}
	fmt.Fprintf(b, "(ROW(%s) %s ROW(%s)", strings.Join(exprs, ","), op, strings.Join(args, ","))
	// SQL, not Go, decides whether a parameter is NULL, including typed nils and
	// nullable driver values. A NULL cursor disables this optional bound. Custom
	// plans can fold these guards away for non-NULL parameters.
	for _, arg := range args {
		fmt.Fprintf(b, " OR %s IS NULL", arg)
	}
	b.WriteString(") AND ")
}
