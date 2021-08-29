package pql

import (
	"strconv"
	"strings"
)

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

func (wc *WhereCls) Neq(v interface{}) *WhereCls {
	wc.op = "<>"
	wc.args = append(wc.args, v)
	return wc
}

func (wc *WhereCls) Lt(v interface{}) *WhereCls {
	wc.op = "<"
	wc.args = append(wc.args, v)
	return wc
}

func (wc *WhereCls) Gt(v interface{}) *WhereCls {
	wc.op = ">"
	wc.args = append(wc.args, v)
	return wc
}

func (wc *WhereCls) Le(v interface{}) *WhereCls {
	wc.op = "<="
	wc.args = append(wc.args, v)
	return wc
}

func (wc *WhereCls) Ge(v interface{}) *WhereCls {
	wc.op = ">="
	wc.args = append(wc.args, v)
	return wc
}

func (wc *WhereCls) Like(v interface{}) *WhereCls {
	wc.op = " LIKE "
	wc.args = append(wc.args, v)
	return wc
}

func (wc *WhereCls) Ilike(v interface{}) *WhereCls {
	wc.op = " ILIKE "
	wc.args = append(wc.args, v)
	return wc
}

func (wc *WhereCls) Contains(v interface{}) *WhereCls {
	wc.op = "@>"
	wc.args = append(wc.args, v)
	return wc
}

func (wc *WhereCls) ContainedBy(v interface{}) *WhereCls {
	wc.op = "<@"
	wc.args = append(wc.args, v)
	return wc
}
