package pql

import (
	"strconv"
	"strings"
)

type WhereCls struct {
	stmt   Builder
	col    string
	op     string
	opType int // default bin
	args   []any

	and  *WhereCls
	or   *WhereCls
	root *WhereCls
}

type Builder interface {
	Build() (string, []any)
}

func Where(col string, args ...any) *WhereCls {
	wc := &WhereCls{col: col, args: args}
	wc.root = wc
	return wc
}

const (
	bin = iota
	monoPost
)

func buildWhere(ws []*WhereCls, b *strings.Builder, args []any) []any {
	b.WriteString(" WHERE (")
	for i, w := range ws {
		if i != 0 {
			b.WriteString(") AND (")
		}
		args = w.build(b, args)
	}
	b.WriteByte(')')
	return args
}

func (wc *WhereCls) Build() (string, []any) {
	if wc.stmt != nil {
		return wc.stmt.Build()
	}
	if wc.root.stmt == nil {
		return "error: cannot build detatched WHERE clause", nil
	}
	return wc.root.stmt.Build()
}

func (wc *WhereCls) build(b *strings.Builder, args []any) []any {
	if wc.op != "" {
		switch wc.opType {
		case bin:
			b.WriteString(wc.col)
			b.WriteString(wc.op)
			args = append(args, wc.args...)
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(len(args)))
		case monoPost:
			b.WriteString(wc.col)
			b.WriteString(wc.op)
		}
	} else if wc.args != nil {
		for _, arg := range wc.args {
			args = append(args, arg)
			wc.col = strings.Replace(wc.col, "?", "$"+strconv.Itoa(len(args)), 1)
		}
		b.WriteString(wc.col)
	} else {
		b.WriteString(wc.col)
	}

	if wc.and != nil {
		b.WriteString(" AND (")
		args = wc.and.build(b, args)
		b.WriteByte(')')
	}
	if wc.or != nil {
		b.WriteString(" OR (")
		args = wc.or.build(b, args)
		b.WriteByte(')')
	}

	return args
}

func (wc *WhereCls) And(col string, args ...any) *WhereCls {
	wc.and = &WhereCls{stmt: wc.stmt, col: col, args: args, root: wc.root}
	return wc.and
}

func (wc *WhereCls) Or(col string, args ...any) *WhereCls {
	wc.or = &WhereCls{stmt: wc.stmt, col: col, args: args, root: wc.root}
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

func (wc *WhereCls) Eq(v any) *WhereCls {
	wc.op = "="
	wc.args = append(wc.args, v)
	return wc
}

func (wc *WhereCls) Neq(v any) *WhereCls {
	wc.op = "<>"
	wc.args = append(wc.args, v)
	return wc
}

func (wc *WhereCls) Lt(v any) *WhereCls {
	wc.op = "<"
	wc.args = append(wc.args, v)
	return wc
}

func (wc *WhereCls) Gt(v any) *WhereCls {
	wc.op = ">"
	wc.args = append(wc.args, v)
	return wc
}

func (wc *WhereCls) Le(v any) *WhereCls {
	wc.op = "<="
	wc.args = append(wc.args, v)
	return wc
}

func (wc *WhereCls) Ge(v any) *WhereCls {
	wc.op = ">="
	wc.args = append(wc.args, v)
	return wc
}

func (wc *WhereCls) Like(v any) *WhereCls {
	wc.op = " LIKE "
	wc.args = append(wc.args, v)
	return wc
}

func (wc *WhereCls) Ilike(v any) *WhereCls {
	wc.op = " ILIKE "
	wc.args = append(wc.args, v)
	return wc
}

func (wc *WhereCls) Contains(v any) *WhereCls {
	wc.op = "@>"
	wc.args = append(wc.args, v)
	return wc
}

func (wc *WhereCls) ContainedBy(v any) *WhereCls {
	wc.op = "<@"
	wc.args = append(wc.args, v)
	return wc
}
