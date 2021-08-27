package pql

import (
	"strconv"
	"strings"
)

func Select(cols ...string) *SelectStmt {
	return &SelectStmt{cols: cols}
}

type SelectStmt struct {
	cols  []string
	table string
	where []*WhereCls
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
		sb.WriteString(" WHERE (")
		for i, w := range ss.where {
			if i != 0 {
				sb.WriteString(") AND (")
			}
			args = w.build(sb, args)
		}
		sb.WriteByte(')')
	}

	return sb.String(), args
}

func (ss *SelectStmt) From(table string) *SelectStmt {
	ss.table = table
	return ss
}

func (ss *SelectStmt) Where(col string) *WhereCls {
	w := &WhereCls{ss: ss, col: col}
	ss.where = append(ss.where, w)
	return w
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

func (wc *WhereCls) build(sb *strings.Builder, args []interface{}) []interface{} {
	sb.WriteString(wc.col)

	if wc.op != "" {
		sb.WriteString(wc.op)
		args = append(args, wc.arg)
		sb.WriteByte('$')
		sb.WriteString(strconv.Itoa(len(args)))
	}

	return args
}

func (wc *WhereCls) Eq(v interface{}) *WhereCls {
	wc.op = "="
	wc.arg = v
	return wc
}
