package pql

import (
	"errors"
	"slices"
	"strconv"
	"strings"
)

// SelectStmt builds a SELECT statement.
type SelectStmt struct {
	cols  []string
	table string
	where []WhereExpr
	order []Order

	limit, offset       int
	limitSet, offsetSet bool
}

// Select creates a SELECT statement with cols.
func Select(cols ...string) *SelectStmt {
	return &SelectStmt{cols: cols}
}

// Select appends result expressions to the statement.
func (s *SelectStmt) Select(cols ...string) *SelectStmt {
	s.cols = append(s.cols, cols...)
	return s
}

// OffsetValue returns the statement's offset.
func (s *SelectStmt) OffsetValue() int { return s.offset }

// Build returns SQL, positional arguments, and a validation error.
func (s *SelectStmt) Build() (string, []any, error) {
	b := &strings.Builder{}
	b.WriteString("SELECT ")

	if len(s.cols) != 0 {
		if err := writeSQLList(b, s.cols, "SELECT expression"); err != nil {
			return "", nil, err
		}
	} else {
		b.WriteByte('*')
	}

	if s.table != "" {
		if strings.TrimSpace(s.table) == "" {
			return "", nil, errors.New("pql: FROM expression is required")
		}
		b.WriteString(" FROM ")
		b.WriteString(terminateLineComment(s.table))
	}

	var args []any
	if len(s.where) != 0 {
		var err error
		args, err = buildWhere(s.where, b, args)
		if err != nil {
			return "", nil, err
		}
	}

	if len(s.order) != 0 {
		if err := buildOrder(b, s.order); err != nil {
			return "", nil, err
		}
	}

	if s.limitSet {
		if s.limit < 0 {
			return "", nil, errors.New("pql: LIMIT must not be negative")
		}
		b.WriteString(" LIMIT ")
		b.WriteString(strconv.Itoa(s.limit))
	}
	if s.offsetSet {
		if s.offset < 0 {
			return "", nil, errors.New("pql: OFFSET must not be negative")
		}
		b.WriteString(" OFFSET ")
		b.WriteString(strconv.Itoa(s.offset))
	}

	return b.String(), args, nil
}

// From sets the table expression.
// An empty string removes the FROM clause.
// A non-empty, whitespace-only expression is rejected by [SelectStmt.Build].
func (s *SelectStmt) From(table string) *SelectStmt {
	s.table = table
	return s
}

// Where appends a complete SQL condition with AND and returns the statement.
// See [Where] for expression syntax and argument handling.
func (s *SelectStmt) Where(expr string, args ...any) *SelectStmt {
	return s.Apply(Where(expr, args...))
}

// WhereNot appends the negation of a complete SQL condition with AND and returns
// the statement. The entire supplied expression is negated; later conditions
// are unaffected. See [Where] for expression syntax and argument handling.
func (s *SelectStmt) WhereNot(expr string, args ...any) *SelectStmt {
	return s.Apply(WhereNot(expr, args...))
}

// SetWhere replaces all WHERE conditions with exprs, combined with AND.
// The input slice is copied; immutable expressions and bound values are shared.
// Calling it without arguments removes the WHERE clause.
// A zero-valued expression is rejected by [SelectStmt.Build].
// SetWhere modifies and returns the statement.
func (s *SelectStmt) SetWhere(exprs ...WhereExpr) *SelectStmt {
	s.where = slices.Clone(exprs)
	return s
}

// Apply appends each expression as a whole condition with AND and returns the statement.
// The input slice is copied; immutable expressions and bound values are shared.
// Calling it without arguments leaves the WHERE clause unchanged.
// Use [SelectStmt.SetWhere] to replace all conditions.
// A zero-valued expression is rejected by [SelectStmt.Build].
func (s *SelectStmt) Apply(exprs ...WhereExpr) *SelectStmt {
	s.where = append(s.where, exprs...)
	return s
}

// Asc appends an ascending order for col.
func (s *SelectStmt) Asc(col string, nulls ...NullsOrder) *SelectStmt {
	s.order = append(s.order, Asc(col, nulls...))
	return s
}

// Desc appends a descending order for col.
func (s *SelectStmt) Desc(col string, nulls ...NullsOrder) *SelectStmt {
	s.order = append(s.order, Desc(col, nulls...))
	return s
}

// Order appends an order for col, descending when desc is true.
func (s *SelectStmt) Order(col string, desc bool, nulls ...NullsOrder) *SelectStmt {
	s.order = append(s.order, newOrder(col, desc, nulls...))
	return s
}

// Orders returns a copy of the statement's ordering.
func (s *SelectStmt) Orders() []Order {
	return append([]Order(nil), s.order...)
}

// SetOrders replaces the statement's ordering with a copy of orders.
func (s *SelectStmt) SetOrders(orders ...Order) *SelectStmt {
	s.order = append([]Order(nil), orders...)
	return s
}

// Limit sets the maximum number of rows returned.
// A value of 0 is honored as LIMIT 0 and must be set explicitly;
// negative values are rejected by Build.
func (s *SelectStmt) Limit(n int) *SelectStmt {
	s.limit = n
	s.limitSet = true
	return s
}

// Offset sets the number of rows skipped.
// A value of 0 is honored as OFFSET 0 and must be set explicitly;
// negative values are rejected by Build.
func (s *SelectStmt) Offset(n int) *SelectStmt {
	s.offset = n
	s.offsetSet = true
	return s
}
