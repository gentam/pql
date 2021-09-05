package pql

import (
	"strconv"
	"strings"
)

type UpdateStmt struct {
	table     string
	m         Map
	where     []*WhereCls
	returning []string
}

func Update(table string) *UpdateStmt {
	return &UpdateStmt{table: table, m: Map{}}
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

	if us.returning != nil {
		buildReturning(sb, us.returning)
	}

	return sb.String(), args
}

func (us *UpdateStmt) Set(col string, val interface{}) *UpdateStmt {
	us.m[col] = val
	return us
}

func (us *UpdateStmt) Values(m Map) *UpdateStmt {
	us.m = m
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

func (us *UpdateStmt) Apply(w *WhereCls) *UpdateStmt {
	w.root.stmt = us
	us.where = append(us.where, w.root)
	return us
}

func (us *UpdateStmt) Returning(cols ...string) *UpdateStmt {
	us.returning = append(us.returning, cols...)
	return us
}
