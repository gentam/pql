package pql

import (
	"fmt"
	"strconv"
	"strings"
)

type WhereCls struct {
	stmt     Builder
	col      string
	exprArgs []any
	op       string
	opArg    any
	hasOpArg bool

	and  *WhereCls
	or   *WhereCls
	root *WhereCls
}

type Builder interface {
	Build() (string, []any, error)
}

func Where(col string, args ...any) *WhereCls {
	wc := &WhereCls{col: col, exprArgs: args}
	wc.root = wc
	return wc
}

func buildWhere(ws []*WhereCls, b *strings.Builder, args []any) ([]any, error) {
	b.WriteString(" WHERE (")
	for i, w := range ws {
		if i != 0 {
			b.WriteString(") AND (")
		}
		var err error
		args, err = w.build(b, args)
		if err != nil {
			return nil, err
		}
	}
	b.WriteByte(')')
	return args, nil
}

func (wc *WhereCls) Build() (string, []any, error) {
	if wc.stmt != nil {
		return wc.stmt.Build()
	}
	if wc.root == nil || wc.root.stmt == nil {
		return "", nil, fmt.Errorf("pql: cannot build detached WHERE clause")
	}
	return wc.root.stmt.Build()
}

func (wc *WhereCls) build(b *strings.Builder, args []any) ([]any, error) {
	if strings.TrimSpace(wc.col) == "" {
		return nil, fmt.Errorf("pql: WHERE condition is required")
	}

	col := wc.col
	if wc.exprArgs != nil {
		if placeholders := strings.Count(col, "?"); placeholders != len(wc.exprArgs) {
			return nil, fmt.Errorf("pql: WHERE placeholder count %d does not match argument count %d", placeholders, len(wc.exprArgs))
		}
		for _, arg := range wc.exprArgs {
			args = append(args, arg)
			col = strings.Replace(col, "?", "$"+strconv.Itoa(len(args)), 1)
		}
	}

	switch {
	case wc.op == "":
		b.WriteString(col)
	case wc.hasOpArg:
		b.WriteString(col)
		b.WriteString(wc.op)
		args = append(args, wc.opArg)
		b.WriteByte('$')
		b.WriteString(strconv.Itoa(len(args)))
	default:
		b.WriteString(col)
		b.WriteString(wc.op)
	}

	if wc.and != nil {
		b.WriteString(" AND (")
		var err error
		args, err = wc.and.build(b, args)
		if err != nil {
			return nil, err
		}
		b.WriteByte(')')
	}
	if wc.or != nil {
		b.WriteString(" OR (")
		var err error
		args, err = wc.or.build(b, args)
		if err != nil {
			return nil, err
		}
		b.WriteByte(')')
	}

	return args, nil
}

func (wc *WhereCls) And(col string, args ...any) *WhereCls {
	wc.and = &WhereCls{stmt: wc.stmt, col: col, exprArgs: args, root: wc.root}
	return wc.and
}

func (wc *WhereCls) Or(col string, args ...any) *WhereCls {
	wc.or = &WhereCls{stmt: wc.stmt, col: col, exprArgs: args, root: wc.root}
	return wc.or
}

func (wc *WhereCls) IsNull() *WhereCls {
	return wc.setPostfix(" IS NULL")
}

func (wc *WhereCls) IsNotNull() *WhereCls {
	return wc.setPostfix(" IS NOT NULL")
}

func (wc *WhereCls) Eq(v any) *WhereCls {
	return wc.setBinary("=", v)
}

func (wc *WhereCls) Neq(v any) *WhereCls {
	return wc.setBinary("<>", v)
}

func (wc *WhereCls) Lt(v any) *WhereCls {
	return wc.setBinary("<", v)
}

func (wc *WhereCls) Gt(v any) *WhereCls {
	return wc.setBinary(">", v)
}

func (wc *WhereCls) Le(v any) *WhereCls {
	return wc.setBinary("<=", v)
}

func (wc *WhereCls) Ge(v any) *WhereCls {
	return wc.setBinary(">=", v)
}

func (wc *WhereCls) Like(v any) *WhereCls {
	return wc.setBinary(" LIKE ", v)
}

func (wc *WhereCls) Ilike(v any) *WhereCls {
	return wc.setBinary(" ILIKE ", v)
}

func (wc *WhereCls) Contains(v any) *WhereCls {
	return wc.setBinary("@>", v)
}

func (wc *WhereCls) ContainedBy(v any) *WhereCls {
	return wc.setBinary("<@", v)
}

func (wc *WhereCls) setBinary(op string, arg any) *WhereCls {
	wc.op = op
	wc.opArg = arg
	wc.hasOpArg = true
	return wc
}

func (wc *WhereCls) setPostfix(op string) *WhereCls {
	wc.op = op
	wc.opArg = nil
	wc.hasOpArg = false
	return wc
}
