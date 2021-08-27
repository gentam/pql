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

func (ss *SelectStmt) Build() (string, []interface{}) {
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

	var args []interface{}
	if ss.where != nil {
		sb.WriteString(" WHERE ")
		sb.WriteString(ss.where.col)
		if ss.where.op != "" {
			sb.WriteString(ss.where.op)
			sb.WriteString("$1")
			args = append(args, ss.where.arg)
		}
	}

	return sb.String(), args
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
	op  string
	arg interface{}
}

func (wc *WhereCls) Build() (string, []interface{}) {
	return wc.ss.Build()
}

func (wc *WhereCls) Eq(v interface{}) *WhereCls {
	wc.op = "="
	wc.arg = v
	return wc
}
