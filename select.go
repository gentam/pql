package pql

import (
	"strconv"
	"strings"
)

// SelectStmt builds a SELECT statement.
type SelectStmt struct {
	cols  []string
	table string
	where []*WhereExpr
	order []Order

	limit, offset int
}

// Select creates a SELECT statement with cols.
func Select(cols ...string) *SelectStmt {
	return &SelectStmt{cols: cols}
}

// Select appends result expressions to the statement.
func (ss *SelectStmt) Select(cols ...string) *SelectStmt {
	ss.cols = append(ss.cols, cols...)
	return ss
}

// OffsetValue returns the statement's offset.
func (ss *SelectStmt) OffsetValue() int { return ss.offset }

// Build returns SQL, positional arguments, and a validation error.
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

// From sets the table expression.
func (ss *SelectStmt) From(table string) *SelectStmt {
	ss.table = table
	return ss
}

// Where appends a condition and returns it for further chaining.
func (ss *SelectStmt) Where(col string, args ...any) *WhereExpr {
	w := &WhereExpr{stmt: ss, col: col, exprArgs: args}
	ss.where = append(ss.where, w)
	return w
}

// WhereNot appends a negated condition and returns it for further chaining.
func (ss *SelectStmt) WhereNot(col string, args ...any) *WhereExpr {
	w := &WhereExpr{stmt: ss, col: "NOT " + col, exprArgs: args}
	ss.where = append(ss.where, w)
	return w
}

// WhereCond appends col when cond is true and its negation otherwise.
func (ss *SelectStmt) WhereCond(cond bool, col string, args ...any) *WhereExpr {
	if cond {
		return ss.Where(col, args...)
	}
	return ss.WhereNot(col, args...)
}

// Apply appends w to the statement. A nil expression is ignored.
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

// Asc appends an ascending order for col.
func (ss *SelectStmt) Asc(col string, nulls ...NullsOrder) *SelectStmt {
	ss.order = append(ss.order, Asc(col, nulls...))
	return ss
}

// Desc appends a descending order for col.
func (ss *SelectStmt) Desc(col string, nulls ...NullsOrder) *SelectStmt {
	ss.order = append(ss.order, Desc(col, nulls...))
	return ss
}

// Order appends an order for col, descending when desc is true.
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

// Limit sets the maximum number of rows returned.
func (ss *SelectStmt) Limit(n int) *SelectStmt {
	ss.limit = n
	return ss
}

// Offset sets the number of rows skipped.
func (ss *SelectStmt) Offset(n int) *SelectStmt {
	ss.offset = n
	return ss
}
