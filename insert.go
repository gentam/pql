package pql

import (
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

func (is *InsertStmt) Build() (string, []any) {
	sb := &strings.Builder{}
	sb.WriteString("INSERT INTO ")
	sb.WriteString(is.table)

	l := len(is.m)
	cols := slices.Sorted(maps.Keys(is.m))
	args := make([]any, l)
	for i, col := range cols {
		args[i] = is.m[col]
	}

	sb.WriteString(" (")
	sb.WriteString(strings.Join(cols, ","))
	sb.WriteString(") VALUES ($1")
	for i := 2; i <= l; i++ {
		sb.WriteString(",$")
		sb.WriteString(strconv.Itoa(i))
	}
	sb.WriteByte(')')

	if is.returning != nil {
		buildReturning(sb, is.returning)
	}

	return sb.String(), args
}

func (is *InsertStmt) Set(col string, val any) *InsertStmt {
	is.m[col] = val
	return is
}

func (is *InsertStmt) Values(m Map) *InsertStmt {
	is.m = m
	return is
}

func (is *InsertStmt) Returning(cols ...string) *InsertStmt {
	is.returning = append(is.returning, cols...)
	return is
}
