package pql

import (
	"strconv"
	"strings"
)

func Update(table string) *UpdateStmt {
	return &UpdateStmt{table: table, m: make(map[string]interface{})}
}

type UpdateStmt struct {
	table string
	m     map[string]interface{}
	where []*WhereCls
}

func (us *UpdateStmt) Build() (string, []interface{}) {
	sb := &strings.Builder{}
	sb.WriteString("UPDATE ")
	sb.WriteString(us.table)

	sb.WriteString(" SET ")
	var args []interface{}
	for col, val := range us.m {
		if args != nil {
			sb.WriteByte(',')
		}
		sb.WriteString(col)
		sb.WriteString("=$")
		args = append(args, val)
		sb.WriteString(strconv.Itoa(len(args)))
	}

	if us.where != nil {
		args = buildWhere(us.where, sb, args)
	}

	return sb.String(), args
}

func (us *UpdateStmt) Set(col string, val interface{}) *UpdateStmt {
	us.m[col] = val
	return us
}

func (us *UpdateStmt) Where(col string, args ...interface{}) *WhereCls {
	w := &WhereCls{stmt: us, col: col, args: args}
	us.where = append(us.where, w)
	return w
}

func (us *UpdateStmt) WhereNot(col string, args ...interface{}) *WhereCls {
	w := &WhereCls{stmt: us, col: "NOT " + col, args: args}
	us.where = append(us.where, w)
	return w
}
