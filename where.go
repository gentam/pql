package pql

import (
	"fmt"
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
	Build() (string, []any, error)
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

	if wc.op != "" {
		switch wc.opType {
		case bin:
			if len(wc.args) != 1 {
				return nil, fmt.Errorf("pql: WHERE operator %q requires 1 argument, got %d", wc.op, len(wc.args))
			}
			b.WriteString(wc.col)
			b.WriteString(wc.op)
			args = append(args, wc.args...)
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(len(args)))
		case monoPost:
			if len(wc.args) != 0 {
				return nil, fmt.Errorf("pql: WHERE operator %q requires 0 arguments, got %d", strings.TrimSpace(wc.op), len(wc.args))
			}
			b.WriteString(wc.col)
			b.WriteString(wc.op)
		}
	} else if wc.args != nil {
		col := wc.col
		if placeholders := strings.Count(col, "?"); placeholders != len(wc.args) {
			return nil, fmt.Errorf("pql: WHERE placeholder count %d does not match argument count %d", placeholders, len(wc.args))
		}
		for _, arg := range wc.args {
			args = append(args, arg)
			col = strings.Replace(col, "?", "$"+strconv.Itoa(len(args)), 1)
		}
		b.WriteString(col)
	} else {
		b.WriteString(wc.col)
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
