package pql

import (
	"errors"
	"slices"
	"strings"
)

// WhereExpr is an immutable value representing a complete WHERE condition.
// Values can be copied and reused without cloning; internal data is shared.
// Create conditions with [Where] or [WhereNot], and combine them with
// [And], [Or], and [Not], or their method equivalents. Composition never changes
// its inputs; use the returned expression to retain the result.
// Build and execute SQL through a statement that uses the expression.
// Bound values such as slices, maps, and pointers are not deep-copied and must
// not be modified during concurrent builds. The zero value is invalid.
type WhereExpr struct {
	expr     string
	args     []any
	logical  string
	children []WhereExpr
}

// Where creates a complete WHERE condition from trusted SQL and bound arguments.
// Write comparisons and other operators directly in SQL using unnumbered $
// placeholders:
//
//	Where("id = $", id)
//	Where("created_at BETWEEN $ AND $", from, to)
//	Where("deleted_at IS NULL")
//	Where("payload ? $", "request_id")
//
// Empty or whitespace-only conditions are rejected when the statement is built.
//
// $ placeholders outside 'strings', identifiers ("price$" or price$usd),
// comments (-- ... or /* ... */), and $tag$dollar-quoted text$tag$
// are numbered in statement order. The number of placeholders must match
// len(args), including when no arguments are supplied. Numbered or named
// placeholders such as $1 or $name, unterminated quoted text or block comments,
// and argument-count mismatches are reported when the statement is built.
// A trailing line comment is terminated with a newline.
//
// String handling assumes PostgreSQL's default standard_conforming_strings=on.
// Use E'...' strings for backslash escapes. Unquoted identifiers may contain $;
// for example, price$usd is an identifier, while price=$ contains a placeholder.
//
// The argument list is copied; bound values are not deep-copied.
func Where(expr string, args ...any) WhereExpr {
	return WhereExpr{expr: expr, args: slices.Clone(args)}
}

// WhereNot creates the negation of a complete SQL condition, equivalent to
// Not(Where(expr, args...)). The entire supplied expression is negated.
// See [Where] for expression syntax and argument handling.
func WhereNot(expr string, args ...any) WhereExpr {
	return Not(Where(expr, args...))
}

// And creates the conjunction of exprs, grouping each input as a whole.
// It copies the input slice without changing any input expression.
// With no inputs it represents TRUE. A zero-valued input is rejected at Build time.
// Bound values are not deep-copied.
func And(exprs ...WhereExpr) WhereExpr {
	return WhereExpr{logical: "AND", children: slices.Clone(exprs)}
}

// Or creates the disjunction of exprs, grouping each input as a whole.
// It copies the input slice without changing any input expression.
// With no inputs it represents FALSE. A zero-valued input is rejected at Build time.
// Bound values are not deep-copied.
func Or(exprs ...WhereExpr) WhereExpr {
	return WhereExpr{logical: "OR", children: slices.Clone(exprs)}
}

// Not negates expr as a whole without changing it.
// A zero-valued input is rejected at Build time. SQL NULL retains its three-valued
// logic: NOT NULL is NULL. Bound values are not deep-copied.
func Not(expr WhereExpr) WhereExpr {
	return WhereExpr{logical: "NOT", children: []WhereExpr{expr}}
}

// And returns a new conjunction of w and exprs, delegating to [And] with w first.
// The receiver and inputs are unchanged; use the returned expression.
// With no arguments the result is logically equivalent to w. A zero-valued receiver
// or input is rejected at Build time. See [And] for argument ownership.
func (w WhereExpr) And(exprs ...WhereExpr) WhereExpr {
	return And(append([]WhereExpr{w}, exprs...)...)
}

// Or returns a new disjunction of w and exprs, delegating to [Or] with w first.
// The receiver and inputs are unchanged; use the returned expression.
// With no arguments the result is logically equivalent to w. A zero-valued receiver
// or input is rejected at Build time. See [Or] for argument ownership.
// Chaining operates on the whole result: a.And(b).Or(c) means (a AND b) OR c.
func (w WhereExpr) Or(exprs ...WhereExpr) WhereExpr {
	return Or(append([]WhereExpr{w}, exprs...)...)
}

// Not returns a new negation of w as a whole, equivalent to Not(w).
// The receiver is unchanged; use the returned expression.
// A zero-valued receiver is rejected at Build time. See [Not] for SQL NULL handling.
func (w WhereExpr) Not() WhereExpr {
	return Not(w)
}

func buildWhere(ws []WhereExpr, b *strings.Builder, args []any) ([]any, error) {
	b.WriteString(" WHERE ")
	return buildConditions(ws, "AND", b, args)
}

func buildConditions(ws []WhereExpr, op string, b *strings.Builder, args []any) ([]any, error) {
	for i, w := range ws {
		if i != 0 {
			b.WriteByte(' ')
			b.WriteString(op)
			b.WriteByte(' ')
		}
		b.WriteByte('(')
		var err error
		args, err = w.build(b, args)
		if err != nil {
			return nil, err
		}
		b.WriteByte(')')
	}
	return args, nil
}

func (w WhereExpr) build(b *strings.Builder, args []any) ([]any, error) {
	switch w.logical {
	case "AND", "OR":
		if len(w.children) == 0 {
			if w.logical == "AND" {
				b.WriteString("TRUE")
			} else {
				b.WriteString("FALSE")
			}
			return args, nil
		}
		return buildConditions(w.children, w.logical, b, args)
	case "NOT":
		b.WriteString("NOT ")
		return buildConditions(w.children, "", b, args)
	}
	if strings.TrimSpace(w.expr) == "" {
		return nil, errors.New("pql: WHERE condition is required")
	}
	sql, err := numberPlaceholders(w.expr, len(args), len(w.args))
	if err != nil {
		return nil, err
	}
	b.WriteString(sql)
	return append(args, w.args...), nil
}
