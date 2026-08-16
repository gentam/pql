package pql

import (
	"fmt"
	"strconv"
	"strings"
)

// WhereExpr represents a chain of WHERE conditions.
type WhereExpr struct {
	stmt     Builder
	col      string
	exprArgs []any
	op       string
	opArg    any
	hasOpArg bool

	and  *WhereExpr
	or   *WhereExpr
	root *WhereExpr
}

// Builder builds SQL and its positional arguments.
type Builder interface {
	// Build returns SQL, positional arguments, and a validation error.
	Build() (string, []any, error)
}

// Where creates a detached WHERE expression.
func Where(col string, args ...any) *WhereExpr {
	wc := &WhereExpr{col: col, exprArgs: args}
	wc.root = wc
	return wc
}

func buildWhere(ws []*WhereExpr, b *strings.Builder, args []any) ([]any, error) {
	b.WriteString(" WHERE (")
	for i, w := range ws {
		if i != 0 {
			b.WriteString(") AND (")
		}
		var err error
		args, err = w.build(b, args)
		if err != nil {
			return nil, err
		}
	}
	b.WriteByte(')')
	return args, nil
}

// Build builds the statement to which the expression is attached.
func (wc *WhereExpr) Build() (string, []any, error) {
	if wc.stmt != nil {
		return wc.stmt.Build()
	}
	if wc.root == nil || wc.root.stmt == nil {
		return "", nil, fmt.Errorf("pql: cannot build detached WHERE clause")
	}
	return wc.root.stmt.Build()
}

func (wc *WhereExpr) build(b *strings.Builder, args []any) ([]any, error) {
	if strings.TrimSpace(wc.col) == "" {
		return nil, fmt.Errorf("pql: WHERE condition is required")
	}

	col := wc.col
	if wc.exprArgs != nil {
		if placeholders := strings.Count(col, "?"); placeholders != len(wc.exprArgs) {
			return nil, fmt.Errorf("pql: WHERE placeholder count %d does not match argument count %d", placeholders, len(wc.exprArgs))
		}
		for _, arg := range wc.exprArgs {
			args = append(args, arg)
			col = strings.Replace(col, "?", "$"+strconv.Itoa(len(args)), 1)
		}
	}

	switch {
	case wc.op == "":
		b.WriteString(col)
	case wc.hasOpArg:
		b.WriteString(col)
		b.WriteString(wc.op)
		args = append(args, wc.opArg)
		b.WriteByte('$')
		b.WriteString(strconv.Itoa(len(args)))
	default:
		b.WriteString(col)
		b.WriteString(wc.op)
	}

	if wc.and != nil {
		b.WriteString(" AND (")
		var err error
		args, err = wc.and.build(b, args)
		if err != nil {
			return nil, err
		}
		b.WriteByte(')')
	}
	if wc.or != nil {
		b.WriteString(" OR (")
		var err error
		args, err = wc.or.build(b, args)
		if err != nil {
			return nil, err
		}
		b.WriteByte(')')
	}

	return args, nil
}

// And appends a right-grouped AND condition and returns it.
func (wc *WhereExpr) And(col string, args ...any) *WhereExpr {
	wc.and = &WhereExpr{stmt: wc.stmt, col: col, exprArgs: args, root: wc.root}
	return wc.and
}

// Or appends a right-grouped OR condition and returns it.
func (wc *WhereExpr) Or(col string, args ...any) *WhereExpr {
	wc.or = &WhereExpr{stmt: wc.stmt, col: col, exprArgs: args, root: wc.root}
	return wc.or
}

// IsNull applies the IS NULL operator.
func (wc *WhereExpr) IsNull() *WhereExpr {
	return wc.setPostfix(" IS NULL")
}

// IsNotNull applies the IS NOT NULL operator.
func (wc *WhereExpr) IsNotNull() *WhereExpr {
	return wc.setPostfix(" IS NOT NULL")
}

// Eq applies the equality operator with v.
func (wc *WhereExpr) Eq(v any) *WhereExpr {
	return wc.setBinary("=", v)
}

// Neq applies the inequality operator with v.
func (wc *WhereExpr) Neq(v any) *WhereExpr {
	return wc.setBinary("<>", v)
}

// Lt applies the less-than operator with v.
func (wc *WhereExpr) Lt(v any) *WhereExpr {
	return wc.setBinary("<", v)
}

// Gt applies the greater-than operator with v.
func (wc *WhereExpr) Gt(v any) *WhereExpr {
	return wc.setBinary(">", v)
}

// Le applies the less-than-or-equal operator with v.
func (wc *WhereExpr) Le(v any) *WhereExpr {
	return wc.setBinary("<=", v)
}

// Ge applies the greater-than-or-equal operator with v.
func (wc *WhereExpr) Ge(v any) *WhereExpr {
	return wc.setBinary(">=", v)
}

// Like applies the LIKE operator with v.
func (wc *WhereExpr) Like(v any) *WhereExpr {
	return wc.setBinary(" LIKE ", v)
}

// Ilike applies the ILIKE operator with v.
func (wc *WhereExpr) Ilike(v any) *WhereExpr {
	return wc.setBinary(" ILIKE ", v)
}

// Contains applies the containment operator with v.
func (wc *WhereExpr) Contains(v any) *WhereExpr {
	return wc.setBinary("@>", v)
}

// ContainedBy applies the contained-by operator with v.
func (wc *WhereExpr) ContainedBy(v any) *WhereExpr {
	return wc.setBinary("<@", v)
}

func (wc *WhereExpr) setBinary(op string, arg any) *WhereExpr {
	wc.op = op
	wc.opArg = arg
	wc.hasOpArg = true
	return wc
}

func (wc *WhereExpr) setPostfix(op string) *WhereExpr {
	wc.op = op
	wc.opArg = nil
	wc.hasOpArg = false
	return wc
}
