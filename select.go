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
		args = buildWhere(ss.where, sb, args)
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
	w := &WhereCls{stmt: ss, col: col, args: args}
	ss.where = append(ss.where, w)
	return w
}

func (ss *SelectStmt) WhereNot(col string, args ...interface{}) *WhereCls {
	w := &WhereCls{stmt: ss, col: "NOT " + col, args: args}
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
