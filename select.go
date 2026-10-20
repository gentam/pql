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
// If no result expressions are added, the statement selects *.
// The column slice is copied. [SelectStmt.Select] appends more result expressions;
// [SelectStmt.SetColumns] replaces them.
// Empty or whitespace-only expressions are rejected by [SelectStmt.Build].
func Select(cols ...string) *SelectStmt {
	return &SelectStmt{cols: slices.Clone(cols)}
}

// Clone copies the statement, including its columns, condition list, ordering,
// and LIMIT/OFFSET settings.
//
// Immutable conditions are shared; bound values are not deep-copied.
// A nil receiver returns nil. The source must not be modified concurrently.
func (s *SelectStmt) Clone() *SelectStmt {
	if s == nil {
		return nil
	}
	cloned := *s
	cloned.cols = slices.Clone(s.cols)
	cloned.order = slices.Clone(s.order)
	for i := range cloned.order {
		cloned.order[i].nulls = slices.Clone(cloned.order[i].nulls)
	}
	cloned.where = slices.Clone(s.where)
	return &cloned
}

// Select appends result expressions to the statement.
// The column slice is copied. Use [SelectStmt.SetColumns] to replace them.
// See [Select] for result expression validation.
func (s *SelectStmt) Select(cols ...string) *SelectStmt {
	s.cols = append(s.cols, cols...)
	return s
}

// Columns returns a copy of the configured result expressions in their specified
// order, without parsing them. If none are configured, it returns an empty slice;
// the default * selection is not added.
func (s *SelectStmt) Columns() []string {
	return slices.Clone(s.cols)
}

// SetColumns replaces the statement's result expressions with a copy of cols.
// Calling it without arguments makes the statement select *.
// See [Select] for result expression validation.
func (s *SelectStmt) SetColumns(cols ...string) *SelectStmt {
	s.cols = slices.Clone(cols)
	return s
}

// OffsetValue returns the configured offset and whether it is set.
// An unset offset returns (0, false); an explicit Offset(0) returns (0, true).
// Negative values are returned unchanged and rejected by [SelectStmt.Build].
func (s *SelectStmt) OffsetValue() (n int, set bool) { return s.offset, s.offsetSet }

// LimitValue returns the configured limit and whether it is set.
// An unset limit returns (0, false); an explicit Limit(0) returns (0, true).
// Negative values are returned unchanged and rejected by [SelectStmt.Build].
func (s *SelectStmt) LimitValue() (n int, set bool) { return s.limit, s.limitSet }

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
// negative values are rejected by [SelectStmt.Build].
// Use [SelectStmt.ClearLimit] to remove the limit.
func (s *SelectStmt) Limit(n int) *SelectStmt {
	s.limit = n
	s.limitSet = true
	return s
}

// ClearLimit removes the LIMIT clause.
func (s *SelectStmt) ClearLimit() *SelectStmt {
	s.limit = 0
	s.limitSet = false
	return s
}

// Offset sets the number of rows skipped.
// A value of 0 is honored as OFFSET 0 and must be set explicitly;
// negative values are rejected by [SelectStmt.Build].
// Use [SelectStmt.ClearOffset] to remove the offset.
func (s *SelectStmt) Offset(n int) *SelectStmt {
	s.offset = n
	s.offsetSet = true
	return s
}

// ClearOffset removes the OFFSET clause.
func (s *SelectStmt) ClearOffset() *SelectStmt {
	s.offset = 0
	s.offsetSet = false
	return s
}
