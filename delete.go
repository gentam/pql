package pql

import (
	"fmt"
	"strings"
)

// DeleteStmt builds a DELETE statement.
type DeleteStmt struct {
	table     string
	where     []*WhereExpr
	returning []string
}

// Delete creates a DELETE statement for table.
func Delete(table string) *DeleteStmt {
	return &DeleteStmt{table: table}
}

// Build returns SQL, positional arguments, and a validation error.
func (ds *DeleteStmt) Build() (string, []any, error) {
	if strings.TrimSpace(ds.table) == "" {
		return "", nil, fmt.Errorf("pql: DELETE table is required")
	}

	b := &strings.Builder{}
	b.WriteString("DELETE FROM ")
	b.WriteString(ds.table)

	var args []any
	if ds.where != nil {
		var err error
		args, err = buildWhere(ds.where, b, args)
		if err != nil {
			return "", nil, err
		}
	}

	if ds.returning != nil {
		buildReturning(b, ds.returning)
	}

	return b.String(), args, nil
}

// Where appends a condition and returns it for further chaining.
func (ds *DeleteStmt) Where(col string, args ...any) *WhereExpr {
	wc := &WhereExpr{stmt: ds, col: col, exprArgs: args}
	ds.where = append(ds.where, wc)
	return wc
}

// WhereNot appends a negated condition and returns it for further chaining.
func (ds *DeleteStmt) WhereNot(col string, args ...any) *WhereExpr {
	wc := &WhereExpr{stmt: ds, col: "NOT " + col, exprArgs: args}
	ds.where = append(ds.where, wc)
	return wc
}

// Apply appends w to the statement. A nil expression is ignored.
func (ds *DeleteStmt) Apply(w *WhereExpr) *DeleteStmt {
	if w == nil {
		return ds
	}
	if w.root == nil {
		w.root = w
	}
	w.root.stmt = ds
	ds.where = append(ds.where, w.root)
	return ds
}

// Returning appends cols to the RETURNING clause.
func (ds *DeleteStmt) Returning(cols ...string) *DeleteStmt {
	ds.returning = append(ds.returning, cols...)
	return ds
}
