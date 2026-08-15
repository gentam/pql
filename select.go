package pql

import (
	"strconv"
	"strings"
)

type SelectStmt struct {
	cols  []string
	table string
	where []*WhereExpr
	order []Order

	limit, offset int
}

func Select(cols ...string) *SelectStmt {
	return &SelectStmt{cols: cols}
}

func (ss *SelectStmt) Select(cols ...string) *SelectStmt {
	ss.cols = append(ss.cols, cols...)
	return ss
}

func (ss *SelectStmt) OffsetValue() int { return ss.offset }

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
		if err := buildOrder(b, ss.order); err != nil {
			return "", nil, err
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

func (ss *SelectStmt) Where(col string, args ...any) *WhereExpr {
	w := &WhereExpr{stmt: ss, col: col, exprArgs: args}
	ss.where = append(ss.where, w)
	return w
}

func (ss *SelectStmt) WhereNot(col string, args ...any) *WhereExpr {
	w := &WhereExpr{stmt: ss, col: "NOT " + col, exprArgs: args}
	ss.where = append(ss.where, w)
	return w
}

func (ss *SelectStmt) WhereCond(cond bool, col string, args ...any) *WhereExpr {
	if cond {
		return ss.Where(col, args...)
	}
	return ss.WhereNot(col, args...)
}

func (ss *SelectStmt) Apply(w *WhereExpr) *SelectStmt {
	if w == nil {
		return ss
	}
	if w.root == nil {
		w.root = w
	}
	w.root.stmt = ss
	ss.where = append(ss.where, w.root)
	return ss
}

func (ss *SelectStmt) Asc(col string, nulls ...NullsOrder) *SelectStmt {
	ss.order = append(ss.order, Asc(col, nulls...))
	return ss
}

func (ss *SelectStmt) Desc(col string, nulls ...NullsOrder) *SelectStmt {
	ss.order = append(ss.order, Desc(col, nulls...))
	return ss
}

func (ss *SelectStmt) Order(col string, desc bool, nulls ...NullsOrder) *SelectStmt {
	ss.order = append(ss.order, newOrder(col, desc, nulls...))
	return ss
}

// Orders returns a copy of the statement's ordering.
func (ss *SelectStmt) Orders() []Order {
	return append([]Order(nil), ss.order...)
}

// SetOrders replaces the statement's ordering with a copy of orders.
func (ss *SelectStmt) SetOrders(orders ...Order) *SelectStmt {
	ss.order = append([]Order(nil), orders...)
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
