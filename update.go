package pql

import (
	"fmt"
	"maps"
	"slices"
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

func (us *UpdateStmt) Build() (string, []any, error) {
	if strings.TrimSpace(us.table) == "" {
		return "", nil, fmt.Errorf("pql: UPDATE table is required")
	}
	if len(us.m) == 0 {
		return "", nil, fmt.Errorf("pql: UPDATE values are required")
	}

	b := &strings.Builder{}
	b.WriteString("UPDATE ")
	b.WriteString(us.table)

	b.WriteString(" SET ")
	cols := slices.Sorted(maps.Keys(us.m))
	args := make([]any, len(cols))
	for i, col := range cols {
		if i != 0 {
			b.WriteByte(',')
		}
		b.WriteString(col)
		b.WriteString("=$")
		args[i] = us.m[col]
		b.WriteString(strconv.Itoa(i + 1))
	}

	if us.where != nil {
		var err error
		args, err = buildWhere(us.where, b, args)
		if err != nil {
			return "", nil, err
		}
	}

	if us.returning != nil {
		buildReturning(b, us.returning)
	}

	return b.String(), args, nil
}

func (us *UpdateStmt) Set(col string, val any) *UpdateStmt {
	us.m[col] = val
	return us
}

func (us *UpdateStmt) Values(m Map) *UpdateStmt {
	us.m = m
	return us
}

func (us *UpdateStmt) Where(col string, args ...any) *WhereCls {
	w := &WhereCls{stmt: us, col: col, args: args}
	us.where = append(us.where, w)
	return w
}

func (us *UpdateStmt) WhereNot(col string, args ...any) *WhereCls {
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
