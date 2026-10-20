package pql

import (
	"errors"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// UpdateStmt builds an UPDATE statement.
type UpdateStmt struct {
	table     string
	m         Map
	where     []WhereExpr
	returning []string
}

// Update creates an UPDATE statement for table.
func Update(table string) *UpdateStmt {
	return &UpdateStmt{table: table, m: Map{}}
}

// Build returns SQL, positional arguments, and a validation error.
// The table must not be empty or whitespace-only, and at least one column
// assignment is required.
func (s *UpdateStmt) Build() (string, []any, error) {
	if strings.TrimSpace(s.table) == "" {
		return "", nil, errors.New("pql: UPDATE table is required")
	}
	if len(s.m) == 0 {
		return "", nil, errors.New("pql: UPDATE values are required")
	}

	b := &strings.Builder{}
	b.WriteString("UPDATE ")
	b.WriteString(terminateLineComment(s.table))

	b.WriteString(" SET ")
	cols := slices.Sorted(maps.Keys(s.m))
	args := make([]any, len(cols))
	for i, col := range cols {
		if strings.TrimSpace(col) == "" {
			return "", nil, errors.New("pql: UPDATE column is required")
		}
		if i != 0 {
			b.WriteByte(',')
		}
		b.WriteString(terminateLineComment(col))
		b.WriteString("=$")
		args[i] = s.m[col]
		b.WriteString(strconv.Itoa(i + 1))
	}

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

// Set assigns val to col.
// Empty or whitespace-only column names are rejected by [UpdateStmt.Build].
func (s *UpdateStmt) Set(col string, val any) *UpdateStmt {
	if s.m == nil {
		s.m = Map{}
	}
	s.m[col] = val
	return s
}

// Values merges m into the statement's values.
// Entries are copied, replacing earlier values for the same columns;
// bound values are not deep-copied.
// Empty or whitespace-only column names are rejected by [UpdateStmt.Build].
func (s *UpdateStmt) Values(m Map) *UpdateStmt {
	if s.m == nil {
		s.m = Map{}
	}
	maps.Copy(s.m, m)
	return s
}

// Where appends a complete SQL condition with AND and returns the statement.
// See [Where] for expression syntax and argument handling.
func (s *UpdateStmt) Where(expr string, args ...any) *UpdateStmt {
	return s.Apply(Where(expr, args...))
}

// WhereNot appends the negation of a complete SQL condition with AND and returns
// the statement. The entire supplied expression is negated; later conditions
// are unaffected. See [Where] for expression syntax and argument handling.
func (s *UpdateStmt) WhereNot(expr string, args ...any) *UpdateStmt {
	return s.Apply(WhereNot(expr, args...))
}

// SetWhere replaces all WHERE conditions with exprs, combined with AND.
// The input slice is copied; immutable expressions and bound values are shared.
// Calling it without arguments removes the WHERE clause.
// A zero-valued expression is rejected by [UpdateStmt.Build].
// SetWhere modifies and returns the statement.
func (s *UpdateStmt) SetWhere(exprs ...WhereExpr) *UpdateStmt {
	s.where = slices.Clone(exprs)
	return s
}

// Apply appends each expression as a whole condition with AND and returns the statement.
// The input slice is copied; immutable expressions and bound values are shared.
// Calling it without arguments leaves the WHERE clause unchanged.
// Use [UpdateStmt.SetWhere] to replace all conditions.
// A zero-valued expression is rejected by [UpdateStmt.Build].
func (s *UpdateStmt) Apply(exprs ...WhereExpr) *UpdateStmt {
	s.where = append(s.where, exprs...)
	return s
}

// Returning appends cols to the RETURNING clause.
// The column slice is copied. Calling it without arguments leaves the clause unchanged.
// Empty or whitespace-only expressions are rejected by [UpdateStmt.Build].
func (s *UpdateStmt) Returning(cols ...string) *UpdateStmt {
	s.returning = append(s.returning, cols...)
	return s
}
