package pql

import (
	"errors"
	"slices"
	"strings"
)

// DeleteStmt builds a DELETE statement.
type DeleteStmt struct {
	table     string
	where     []WhereExpr
	returning []string
}

// Delete creates a DELETE statement for table.
func Delete(table string) *DeleteStmt {
	return &DeleteStmt{table: table}
}

// Clone copies the statement, including its condition and RETURNING lists.
// Immutable conditions are shared; bound values are not deep-copied.
// A nil receiver returns nil. The source must not be modified concurrently.
func (s *DeleteStmt) Clone() *DeleteStmt {
	if s == nil {
		return nil
	}
	cloned := *s
	cloned.where = slices.Clone(s.where)
	cloned.returning = slices.Clone(s.returning)
	return &cloned
}

// Build returns SQL, positional arguments, and a validation error.
// The table must not be empty or whitespace-only.
func (s *DeleteStmt) Build() (string, []any, error) {
	if strings.TrimSpace(s.table) == "" {
		return "", nil, errors.New("pql: DELETE table is required")
	}

	b := &strings.Builder{}
	b.WriteString("DELETE FROM ")
	b.WriteString(terminateLineComment(s.table))

	var args []any
	if len(s.where) != 0 {
		var err error
		args, err = buildWhere(s.where, b, args)
		if err != nil {
			return "", nil, err
		}
	}

	if len(s.returning) != 0 {
		if err := buildReturning(b, s.returning); err != nil {
			return "", nil, err
		}
	}

	return b.String(), args, nil
}

// Where appends a complete SQL condition with AND and returns the statement.
// See [Where] for expression syntax and argument handling.
func (s *DeleteStmt) Where(expr string, args ...any) *DeleteStmt {
	return s.Apply(Where(expr, args...))
}

// WhereNot appends the negation of a complete SQL condition with AND and returns
// the statement. The entire supplied expression is negated; later conditions
// are unaffected. See [Where] for expression syntax and argument handling.
func (s *DeleteStmt) WhereNot(expr string, args ...any) *DeleteStmt {
	return s.Apply(WhereNot(expr, args...))
}

// SetWhere replaces all WHERE conditions with exprs, combined with AND.
// The input slice is copied; immutable expressions and bound values are shared.
// Calling it without arguments removes the WHERE clause.
// A zero-valued expression is rejected by [DeleteStmt.Build].
// SetWhere modifies and returns the statement.
func (s *DeleteStmt) SetWhere(exprs ...WhereExpr) *DeleteStmt {
	s.where = slices.Clone(exprs)
	return s
}

// Apply appends each expression as a whole condition with AND and returns the statement.
// The input slice is copied; immutable expressions and bound values are shared.
// Calling it without arguments leaves the WHERE clause unchanged.
// Use [DeleteStmt.SetWhere] to replace all conditions.
// A zero-valued expression is rejected by [DeleteStmt.Build].
func (s *DeleteStmt) Apply(exprs ...WhereExpr) *DeleteStmt {
	s.where = append(s.where, exprs...)
	return s
}

// Returning appends cols to the RETURNING clause.
// The column slice is copied. Calling it without arguments leaves the clause unchanged.
// Use [DeleteStmt.SetReturning] to replace the result expressions.
// Empty or whitespace-only expressions are rejected by [DeleteStmt.Build].
func (s *DeleteStmt) Returning(cols ...string) *DeleteStmt {
	s.returning = append(s.returning, cols...)
	return s
}

// SetReturning replaces the RETURNING result expressions with a copy of cols.
// Calling it without arguments removes the RETURNING clause.
// See [DeleteStmt.Returning] for result expression validation.
func (s *DeleteStmt) SetReturning(cols ...string) *DeleteStmt {
	s.returning = slices.Clone(cols)
	return s
}
