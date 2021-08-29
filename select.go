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

	limit, offset int
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

	if ss.limit != 0 {
		sb.WriteString(" LIMIT ")
		sb.WriteString(strconv.Itoa(ss.limit))
	}
	if ss.offset != 0 {
		sb.WriteString(" OFFSET ")
		sb.WriteString(strconv.Itoa(ss.offset))
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

func (ss *SelectStmt) Limit(n int) *SelectStmt {
	ss.limit = n
	return ss
}

func (ss *SelectStmt) Offset(n int) *SelectStmt {
	ss.offset = n
	return ss
}

type WhereCls struct {
	ss     *SelectStmt
	col    string
	op     string
	opType int // default bin
	args   []interface{}

	and *WhereCls
	or  *WhereCls
}

const (
	bin = iota
	monoPost
)

func (wc *WhereCls) Build() (string, []interface{}) {
	return wc.ss.Build()
}

func (wc *WhereCls) build(sb *strings.Builder, args []interface{}) []interface{} {
	if wc.op != "" {
		switch wc.opType {
		case bin:
			sb.WriteString(wc.col)
			sb.WriteString(wc.op)
			args = append(args, wc.args...)
			sb.WriteString("$" + strconv.Itoa(len(args)))
		case monoPost:
			sb.WriteString(wc.col)
			sb.WriteString(wc.op)
		}
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

func (wc *WhereCls) And(col string, args ...interface{}) *WhereCls {
	wc.and = &WhereCls{ss: wc.ss, col: col, args: args}
	return wc.and
}

func (wc *WhereCls) Or(col string, args ...interface{}) *WhereCls {
	wc.or = &WhereCls{ss: wc.ss, col: col, args: args}
	return wc.or
}

func (wc *WhereCls) IsNull() *WhereCls {
	wc.op = " IS NULL"
	wc.opType = monoPost
	return wc
}

func (wc *WhereCls) IsNotNull() *WhereCls {
	wc.op = " IS NOT NULL"
	wc.opType = monoPost
	return wc
}

func (wc *WhereCls) Eq(v interface{}) *WhereCls {
	wc.op = "="
	wc.args = append(wc.args, v)
	return wc
}
