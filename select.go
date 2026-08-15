package pql

import (
	"strconv"
	"strings"
)

type SelectStmt struct {
	cols  []string
	table string
	where []*WhereCls
	order []order

	limit, offset int
}

type order struct {
	col  string
	desc bool
}

func Select(cols ...string) *SelectStmt {
	return &SelectStmt{cols: cols}
}

func (ss *SelectStmt) Select(cols ...string) *SelectStmt {
	ss.cols = append(ss.cols, cols...)
	return ss
}

func (ss *SelectStmt) GetOffset() int { return ss.offset }

func (ss *SelectStmt) Build() (string, []any, error) {
	b := &strings.Builder{}
	b.WriteString("SELECT ")

	if len(ss.cols) != 0 {
		b.WriteString(strings.Join(ss.cols, ","))
	} else {
		b.WriteByte('*')
	}

	if ss.table != "" {
		b.WriteString(" FROM ")
		b.WriteString(ss.table)
	}

	var args []any
	if ss.where != nil {
		var err error
		args, err = buildWhere(ss.where, b, args)
		if err != nil {
			return "", nil, err
		}
	}

	if ss.order != nil {
		b.WriteString(" ORDER BY ")
		for i, ord := range ss.order {
			if i != 0 {
				b.WriteByte(',')
			}
			b.WriteString(ord.col)
			if ord.desc {
				b.WriteString(" DESC")
			} else {
				b.WriteString(" ASC")
			}
		}
	}

	if ss.limit != 0 {
		b.WriteString(" LIMIT ")
		b.WriteString(strconv.Itoa(ss.limit))
	}
	if ss.offset != 0 {
		b.WriteString(" OFFSET ")
		b.WriteString(strconv.Itoa(ss.offset))
	}

	return b.String(), args, nil
}

func (ss *SelectStmt) From(table string) *SelectStmt {
	ss.table = table
	return ss
}

func (ss *SelectStmt) Where(col string, args ...any) *WhereCls {
	w := &WhereCls{stmt: ss, col: col, exprArgs: args}
	ss.where = append(ss.where, w)
	return w
}

func (ss *SelectStmt) WhereNot(col string, args ...any) *WhereCls {
	w := &WhereCls{stmt: ss, col: "NOT " + col, exprArgs: args}
	ss.where = append(ss.where, w)
	return w
}

func (ss *SelectStmt) WhereCond(cond bool, col string, args ...any) *WhereCls {
	if cond {
		return ss.Where(col, args...)
	}
	return ss.WhereNot(col, args...)
}

func (ss *SelectStmt) Apply(w *WhereCls) *SelectStmt {
	w.root.stmt = ss
	ss.where = append(ss.where, w.root)
	return ss
}

func (ss *SelectStmt) Asc(col string) *SelectStmt {
	ss.order = append(ss.order, order{col: col})
	return ss
}

func (ss *SelectStmt) Desc(col string) *SelectStmt {
	ss.order = append(ss.order, order{col: col, desc: true})
	return ss
}

func (ss *SelectStmt) Order(col string, desc bool) *SelectStmt {
	ss.order = append(ss.order, order{col: col, desc: desc})
	return ss
}

func (ss *SelectStmt) Limit(n int) *SelectStmt {
	ss.limit = n
	return ss
}

func (ss *SelectStmt) Offset(n int) *SelectStmt {
	ss.offset = n
	return ss
}
