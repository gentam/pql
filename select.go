package pql

import "strings"

func Select(cols ...string) *SelectStmt {
	return &SelectStmt{cols: cols}
}

type SelectStmt struct {
	cols  []string
	table string
	where *WhereCls
}

func (ss *SelectStmt) Build() string {
	sb := &strings.Builder{}
	sb.WriteString("SELECT ")

	if ss.cols != nil {
		for i, col := range ss.cols {
			if i != 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(col)
		}
	} else {
		sb.WriteByte('*')
	}

	if ss.table != "" {
		sb.WriteString(" FROM ")
		sb.WriteString(ss.table)
	}

	if ss.where != nil {
		sb.WriteString(" WHERE ")
		sb.WriteString(ss.where.col)
	}

	return sb.String()
}

func (ss *SelectStmt) From(table string) *SelectStmt {
	ss.table = table
	return ss
}

func (ss *SelectStmt) Where(col string) *WhereCls {
	ss.where = &WhereCls{ss: ss, col: col}
	return ss.where
}

type WhereCls struct {
	ss  *SelectStmt
	col string
}

func (wc *WhereCls) Build() string {
	return wc.ss.Build()
}
