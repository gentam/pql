package pql

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
)

type InsertStmt struct {
	table     string
	m         Map
	returning []string
}

func Insert(table string) *InsertStmt {
	return &InsertStmt{table: table, m: Map{}}
}

func (is *InsertStmt) Build() (string, []any, error) {
	if strings.TrimSpace(is.table) == "" {
		return "", nil, fmt.Errorf("pql: INSERT table is required")
	}
	if len(is.m) == 0 {
		return "", nil, fmt.Errorf("pql: INSERT values are required")
	}

	b := &strings.Builder{}
	b.WriteString("INSERT INTO ")
	b.WriteString(is.table)

	l := len(is.m)
	cols := slices.Sorted(maps.Keys(is.m))
	args := make([]any, l)
	for i, col := range cols {
		args[i] = is.m[col]
	}

	b.WriteString(" (")
	b.WriteString(strings.Join(cols, ","))
	b.WriteString(") VALUES ($1")
	for i := 2; i <= l; i++ {
		b.WriteString(",$")
		b.WriteString(strconv.Itoa(i))
	}
	b.WriteByte(')')

	if is.returning != nil {
		buildReturning(b, is.returning)
	}

	return b.String(), args, nil
}

func (is *InsertStmt) Set(col string, val any) *InsertStmt {
	if is.m == nil {
		is.m = Map{}
	}
	is.m[col] = val
	return is
}

func (is *InsertStmt) Values(m Map) *InsertStmt {
	is.m = maps.Clone(m)
	return is
}

func (is *InsertStmt) Returning(cols ...string) *InsertStmt {
	is.returning = append(is.returning, cols...)
	return is
}
