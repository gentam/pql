package pql

import "strings"

func Select(cols ...string) *SelectStmt {
	return &SelectStmt{cols: cols}
}

type SelectStmt struct {
	cols []string
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

	return sb.String()
}
