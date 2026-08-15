package pql

import "strings"

type DeleteStmt struct {
	table     string
	where     []*WhereCls
	returning []string
}

func Delete(table string) *DeleteStmt {
	return &DeleteStmt{table: table}
}

func (ds *DeleteStmt) Build() (string, []any) {
	b := &strings.Builder{}
	b.WriteString("DELETE FROM ")
	b.WriteString(ds.table)

	var args []any
	if ds.where != nil {
		args = buildWhere(ds.where, b, args)
	}

	if ds.returning != nil {
		buildReturning(b, ds.returning)
	}

	return b.String(), args
}

func (ds *DeleteStmt) Where(col string, args ...any) *WhereCls {
	wc := &WhereCls{stmt: ds, col: col, args: args}
	ds.where = append(ds.where, wc)
	return wc
}

func (ds *DeleteStmt) WhereNot(col string, args ...any) *WhereCls {
	wc := &WhereCls{stmt: ds, col: "NOT " + col, args: args}
	ds.where = append(ds.where, wc)
	return wc
}

func (ds *DeleteStmt) Apply(w *WhereCls) *DeleteStmt {
	w.root.stmt = ds
	ds.where = append(ds.where, w.root)
	return ds
}

func (ds *DeleteStmt) Returning(cols ...string) *DeleteStmt {
	ds.returning = append(ds.returning, cols...)
	return ds
}
