package pql

import "strings"

type DeleteStmt struct {
	table string
	where []*WhereCls
}

func Delete(table string) *DeleteStmt {
	return &DeleteStmt{table: table}
}

func (ds *DeleteStmt) Build() (string, []interface{}) {
	sb := &strings.Builder{}
	sb.WriteString("DELETE FROM ")
	sb.WriteString(ds.table)

	var args []interface{}
	if ds.where != nil {
		args = buildWhere(ds.where, sb, args)
	}

	return sb.String(), args
}

func (ds *DeleteStmt) Where(col string, args ...interface{}) *WhereCls {
	wc := &WhereCls{stmt: ds, col: col, args: args}
	ds.where = append(ds.where, wc)
	return wc
}

func (ds *DeleteStmt) WhereNot(col string, args ...interface{}) *WhereCls {
	wc := &WhereCls{stmt: ds, col: "NOT " + col, args: args}
	ds.where = append(ds.where, wc)
	return wc
}

func (ds *DeleteStmt) Apply(w *WhereCls) *DeleteStmt {
	w.root.stmt = ds
	ds.where = append(ds.where, w.root)
	return ds
}
