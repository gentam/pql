package pql

import "strings"

func Select(cols ...string) *SelectStmt {
	return &SelectStmt{cols: cols}
}

type SelectStmt struct {
	cols  []string
	table string
}

func (ss *SelectStmt) Build() string {
	sb := &strings.Builder{}
	sb.WriteString("SELECT ")

	for i, col := range ss.cols {
		if i != 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(col)
	}

	if ss.table != "" {
		sb.WriteString(" FROM ")
		sb.WriteString(ss.table)
	}

	return sb.String()
}

func (ss *SelectStmt) From(table string) *SelectStmt {
	ss.table = table
	return ss
}
