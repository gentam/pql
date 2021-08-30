package pql

import (
	"strconv"
	"strings"
)

type InsertStmt struct {
	table string
	m     map[string]interface{}
}

func Insert(table string) *InsertStmt {
	return &InsertStmt{table: table, m: make(map[string]interface{})}
}

func (is *InsertStmt) Build() (string, []interface{}) {
	sb := &strings.Builder{}
	sb.WriteString("INSERT INTO ")
	sb.WriteString(is.table)

	l := len(is.m)
	args := make([]interface{}, 0, l)
	sb.WriteString(" (")
	for col, val := range is.m {
		if len(args) != 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(col)
		args = append(args, val)
	}

	sb.WriteString(") VALUES ($1")
	for i := 2; i <= l; i++ {
		sb.WriteString(",$")
		sb.WriteString(strconv.Itoa(i))
	}
	sb.WriteByte(')')

	return sb.String(), args
}

func (is *InsertStmt) Set(col string, val interface{}) *InsertStmt {
	is.m[col] = val
	return is
}
