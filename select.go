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

func (ss *SelectStmt) Where(col string, args ...interface{}) *WhereCls {
	w := &WhereCls{ss: ss, col: col, args: args}
	ss.where = append(ss.where, w)
	return w
}

func (ss *SelectStmt) WhereNot(col string, args ...interface{}) *WhereCls {
	w := &WhereCls{ss: ss, col: "NOT " + col, args: args}
	ss.where = append(ss.where, w)
	return w
}

type WhereCls struct {
	ss   *SelectStmt
	col  string
	op   string
	args []interface{}

	and *WhereCls
	or  *WhereCls
}

func (wc *WhereCls) Build() (string, []interface{}) {
	return wc.ss.Build()
}

func (wc *WhereCls) build(sb *strings.Builder, args []interface{}) []interface{} {
	if wc.op != "" {
		sb.WriteString(wc.col)
		sb.WriteString(wc.op)
		args = append(args, wc.args...)
		sb.WriteString("$" + strconv.Itoa(len(args)))
	} else if wc.args != nil {
		for _, arg := range wc.args {
			args = append(args, arg)
			wc.col = strings.Replace(wc.col, "?", "$"+strconv.Itoa(len(args)), 1)
		}
		sb.WriteString(wc.col)
	} else {
		sb.WriteString(wc.col)
	}

	if wc.and != nil {
		sb.WriteString(" AND (")
		args = wc.and.build(sb, args)
		sb.WriteByte(')')
	}
	if wc.or != nil {
		sb.WriteString(" OR (")
		args = wc.or.build(sb, args)
		sb.WriteByte(')')
	}

	return args
}

func (wc *WhereCls) And(col string) *WhereCls {
	wc.and = &WhereCls{ss: wc.ss, col: col}
	return wc.and
}

func (wc *WhereCls) Or(col string) *WhereCls {
	wc.or = &WhereCls{ss: wc.ss, col: col}
	return wc.or
}

func (wc *WhereCls) Eq(v interface{}) *WhereCls {
	wc.op = "="
	wc.args = append(wc.args, v)
	return wc
}
