package pql

import (
	"errors"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// InsertStmt builds an INSERT statement.
type InsertStmt struct {
	table     string
	m         Map
	returning []string
}

// Insert creates an INSERT statement for table.
func Insert(table string) *InsertStmt {
	return &InsertStmt{table: table, m: Map{}}
}

// Build returns SQL, positional arguments, and a validation error.
// The table must not be empty or whitespace-only, and at least one column
// assignment is required.
func (s *InsertStmt) Build() (string, []any, error) {
	if strings.TrimSpace(s.table) == "" {
		return "", nil, errors.New("pql: INSERT table is required")
	}
	if len(s.m) == 0 {
		return "", nil, errors.New("pql: INSERT values are required")
	}

	b := &strings.Builder{}
	b.WriteString("INSERT INTO ")
	b.WriteString(terminateLineComment(s.table))

	n := len(s.m)
	cols := slices.Sorted(maps.Keys(s.m))
	args := make([]any, n)
	for i, col := range cols {
		args[i] = s.m[col]
	}

	b.WriteString(" (")
	if err := writeSQLList(b, cols, "INSERT column"); err != nil {
		return "", nil, err
	}
	b.WriteString(") VALUES ($1")
	for i := 2; i <= n; i++ {
		b.WriteString(",$")
		b.WriteString(strconv.Itoa(i))
	}
	b.WriteByte(')')

	if len(s.returning) != 0 {
		if err := buildReturning(b, s.returning); err != nil {
			return "", nil, err
		}
	}

	return b.String(), args, nil
}

// Set assigns val to col.
// Empty or whitespace-only column names are rejected by [InsertStmt.Build].
func (s *InsertStmt) Set(col string, val any) *InsertStmt {
	if s.m == nil {
		s.m = Map{}
	}
	s.m[col] = val
	return s
}

// Values merges m into the statement's values.
// Entries are copied, replacing earlier values for the same columns;
// bound values are not deep-copied.
// Empty or whitespace-only column names are rejected by [InsertStmt.Build].
func (s *InsertStmt) Values(m Map) *InsertStmt {
	if s.m == nil {
		s.m = Map{}
	}
	maps.Copy(s.m, m)
	return s
}

// Returning appends cols to the RETURNING clause.
// The column slice is copied. Calling it without arguments leaves the clause unchanged.
// Empty or whitespace-only expressions are rejected by [InsertStmt.Build].
func (s *InsertStmt) Returning(cols ...string) *InsertStmt {
	s.returning = append(s.returning, cols...)
	return s
}
