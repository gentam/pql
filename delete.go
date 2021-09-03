package pql

import "strings"

type DeleteStatement struct {
	table string
	where []*WhereCls
}

func Delete(table string) *DeleteStatement {
	return &DeleteStatement{table: table}
}

func (ds *DeleteStatement) Build() (string, []interface{}) {
	sb := &strings.Builder{}
	sb.WriteString("DELETE FROM ")
	sb.WriteString(ds.table)

	var args []interface{}
	if ds.where != nil {
		args = buildWhere(ds.where, sb, args)
	}

	return sb.String(), args
}

func (ds *DeleteStatement) Where(col string, args ...interface{}) *WhereCls {
	wc := &WhereCls{stmt: ds, col: col, args: args}
	ds.where = append(ds.where, wc)
	return wc
}

func (ds *DeleteStatement) WhereNot(col string, args ...interface{}) *WhereCls {
	wc := &WhereCls{stmt: ds, col: "NOT " + col, args: args}
	ds.where = append(ds.where, wc)
	return wc
}

func (ds *DeleteStatement) Apply(w *WhereCls) *DeleteStatement {
	w.root.stmt = ds
	ds.where = append(ds.where, w.root)
	return ds
}
