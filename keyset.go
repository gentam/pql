package pql

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"
)

// After creates a condition matching rows strictly after values in
// orders. Supply one value per order, including a unique tie-breaker for paging.
// Mixed directions and NULL placement are respected; nil represents SQL NULL.
//
// Values must be encodable by pgx for the corresponding SQL expression's type.
// For example, use time.Time or pgtype.Date for a date, converting serialized
// cursor strings before calling this function. pql does not decode cursor values.
//
// The SQL depends on which values are NULL, so that PostgreSQL can seek an
// index even with the generic plans it may choose for statements that pgx
// prepares and caches. nil, nil pointers, slices, and maps, and
// [database/sql/driver.Valuer] implementations whose Value returns nil are
// SQL NULL and are not bound. Booleans, numbers, strings, arrays, time.Time,
// pointers to them, and other Valuer implementations are not NULL. For any
// other type, whether the value is NULL is decided in SQL, which is correct
// but limits generic plans to filtering rows.
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
	exprs := make([]string, len(k.orders))
	for i, order := range k.orders {
		expr, err := order.sql()
		if err != nil {
			return "", nil, err
		}
		exprs[i] = "(" + expr + ")"
	}

	// Bind each cursor value once and reuse it. Values that are SQL NULL are
	// not bound: their conditions are written without a parameter.
	keys := make([]keysetKey, len(k.orders))
	for i, order := range k.orders {
		key := keysetKey{expr: exprs[i], null: cursorNullness(k.values[i])}
		if key.null != nullYes {
			args = append(args, k.values[i])
			key.arg = "$" + strconv.Itoa(len(args))
		}
		key.op = ">="
		if order.desc != k.before {
			key.op = "<="
		}
		key.nullsBefore = order.NullsLast() == k.before
		keys[i] = key
	}
	b := &strings.Builder{}
	k.buildRowBound(b, keys)
	for i, key := range keys {
		// First restrict this key to the inclusive range. This exposes a leading
		// range comparison to the planner while handling NULLs as ordered values.
		b.WriteByte('(')
		key.writeRange(b)
		b.WriteByte(')')
		if i < len(keys)-1 {
			// A strictly different key is enough; equal keys defer to the suffix.
			b.WriteString(" AND (")
			key.writeDistinct(b)
			b.WriteString(" OR (")
		} else if !k.inclusive {
			b.WriteString(" AND ")
			key.writeDistinct(b)
		}
	}
	b.WriteString(strings.Repeat("))", len(keys)-1))
	return b.String(), args, nil
}

// nullness classifies a cursor value by whether pgx encodes it as SQL NULL.
type nullness uint8

const (
	// nullUnknown values are checked for NULL in SQL.
	nullUnknown nullness = iota
	nullNo
	nullYes
)

// cursorNullness reports whether v is encoded as SQL NULL, as far as Go can
// tell. nil, nil pointers, slices, and maps, and driver.Valuer implementations
// returning nil are NULL. Non-nil pointers are dereferenced. Booleans, numbers,
// strings, arrays, non-nil slices and maps, time.Time, and Valuer
// implementations returning a value are not NULL. Other values are unknown.
//
// Knowing a value statically keeps NULL checks on parameters out of the SQL,
// so that generic plans of prepared statements can still seek an index.
func cursorNullness(v any) nullness {
	for v != nil {
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Map:
			if rv.IsNil() {
				return nullYes
			}
		}
		// Check after nil pointers: calling Value through one could panic.
		if valuer, ok := v.(driver.Valuer); ok {
			value, err := valuer.Value()
			switch {
			case err != nil:
				return nullUnknown
			case value == nil:
				return nullYes
			default:
				return nullNo
			}
		}
		switch rv.Kind() {
		case reflect.Pointer:
			v = rv.Elem().Interface()
			continue
		case reflect.Bool, reflect.String, reflect.Array, reflect.Slice, reflect.Map,
			reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Float32, reflect.Float64:
			return nullNo
		}
		if _, ok := v.(time.Time); ok {
			return nullNo
		}
		return nullUnknown
	}
	return nullYes
}

// keysetKey holds one ordered key of a keyset condition.
type keysetKey struct {
	expr        string   // parenthesized ORDER BY expression
	arg         string   // parameter, or empty when the cursor value is NULL
	null        nullness // whether the cursor value is NULL
	op          string   // inclusive comparison in the requested direction
	nullsBefore bool     // NULL keys precede the requested range
}

// writeRange writes the condition that the key is at or beyond the cursor.
func (k keysetKey) writeRange(b *strings.Builder) {
	switch {
	case k.null == nullYes && k.nullsBefore:
		b.WriteString("TRUE") // Every key is at or beyond a leading NULL.
	case k.null == nullYes:
		fmt.Fprintf(b, "%s IS NULL", k.expr)
	case k.null == nullNo && k.nullsBefore:
		// Keep the NULL check so that the condition is FALSE, not NULL, under NOT.
		fmt.Fprintf(b, "%s %s %s AND %s IS NOT NULL", k.expr, k.op, k.arg, k.expr)
	case k.null == nullNo:
		fmt.Fprintf(b, "%s %s %s OR %s IS NULL", k.expr, k.op, k.arg, k.expr)
	case k.nullsBefore:
		fmt.Fprintf(b, "%s %s %s AND %s IS NOT NULL OR %s IS NULL", k.expr, k.op, k.arg, k.expr, k.arg)
	default:
		fmt.Fprintf(b, "%s %s %s AND %s IS NOT NULL OR %s IS NULL", k.expr, k.op, k.arg, k.arg, k.expr)
	}
}

// writeDistinct writes the condition that the key differs from the cursor.
func (k keysetKey) writeDistinct(b *strings.Builder) {
	if k.null == nullYes {
		fmt.Fprintf(b, "%s IS NOT NULL", k.expr)
		return
	}
	fmt.Fprintf(b, "%s IS DISTINCT FROM %s", k.expr, k.arg)
}

// buildRowBound adds a redundant bound that PostgreSQL can use to seek into a
// multicolumn index, rather than filtering all equal values of the leading key.
// A row comparison is safe for the same-direction prefix whose NULLs precede
// the requested range and whose cursor values are not NULL: it is an inclusive
// bound on that prefix, implied by the full predicate, which still handles
// NULLs and ties.
func (k *keysetExpr) buildRowBound(b *strings.Builder, keys []keysetKey) {
	n := 0
	for i, key := range keys {
		if k.orders[i].desc != k.orders[0].desc || !key.nullsBefore || key.null == nullYes {
			break
		}
		n++
	}
	if n < 2 {
		return
	}
	exprs, args := make([]string, n), make([]string, n)
	for i, key := range keys[:n] {
		exprs[i], args[i] = key.expr, key.arg
	}
	// Keep the bound inclusive even for strict paging: the full predicate removes
	// the cursor itself. This also avoids underestimating a large equality group
	// as an almost-empty strict range on the leading key.
	fmt.Fprintf(b, "(ROW(%s) %s ROW(%s)", strings.Join(exprs, ","), keys[0].op, strings.Join(args, ","))
	// A cursor value that Go cannot classify may still be NULL at run time,
	// which disables this optional bound.
	for _, key := range keys[:n] {
		if key.null == nullUnknown {
			fmt.Fprintf(b, " OR %s IS NULL", key.arg)
		}
	}
	b.WriteString(") AND ")
}
