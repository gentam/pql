package pql

import "testing"

func TestWriteStatementClone(t *testing.T) {
	t.Parallel()
	t.Run("insert", func(t *testing.T) {
		var absent *InsertStmt
		if absent.Clone() != nil {
			t.Fatal("nil receiver clone must be nil")
		}
		// Spare capacity exposes shared backing arrays during independent appends.
		source := &InsertStmt{table: "t", returning: make([]string, 0, 4)}
		empty := source.Clone()
		source.Set("a", 1).Returning("id")
		assertBuild(t, empty, buildResult{err: "pql: INSERT values are required"})
		clone := source.Clone()
		source.Set("a", 2).Set("b", 3).Returning("b")
		clone.Values(Map{"a": 4, "c": 5}).Returning("c")
		assertBuild(t, source, buildResult{query: "INSERT INTO t (a,b) VALUES ($1,$2) RETURNING id,b", args: []any{2, 3}})
		assertBuild(t, clone, buildResult{query: "INSERT INTO t (a,c) VALUES ($1,$2) RETURNING id,c", args: []any{4, 5}})
	})
	t.Run("update", func(t *testing.T) {
		var absent *UpdateStmt
		if absent.Clone() != nil {
			t.Fatal("nil receiver clone must be nil")
		}
		source := &UpdateStmt{table: "t", where: make([]WhereExpr, 0, 4), returning: make([]string, 0, 4)}
		empty := source.Clone()
		source.Set("a", 1).Where("tenant=$", 10).Returning("id")
		assertBuild(t, empty, buildResult{err: "pql: UPDATE values are required"})
		clone := source.Clone()
		source.Set("a", 2).Set("b", 3).Where("id=$", 20).Returning("b")
		clone.Values(Map{"a": 4, "c": 5}).WhereNot("id=$", 30).Returning("c")
		assertBuild(t, source, buildResult{query: "UPDATE t SET a=$1,b=$2 WHERE (tenant=$3) AND (id=$4) RETURNING id,b", args: []any{2, 3, 10, 20}})
		assertBuild(t, clone, buildResult{query: "UPDATE t SET a=$1,c=$2 WHERE (tenant=$3) AND (NOT (id=$4)) RETURNING id,c", args: []any{4, 5, 10, 30}})
	})
	t.Run("delete", func(t *testing.T) {
		var absent *DeleteStmt
		if absent.Clone() != nil {
			t.Fatal("nil receiver clone must be nil")
		}
		source := &DeleteStmt{table: "t", where: make([]WhereExpr, 0, 4), returning: make([]string, 0, 4)}
		empty := source.Clone()
		source.Where("tenant=$", 10).Returning("id")
		assertBuild(t, empty, buildResult{query: "DELETE FROM t"})
		clone := source.Clone()
		source.Where("id=$", 20).Returning("a")
		clone.WhereNot("id=$", 30).Returning("b")
		assertBuild(t, source, buildResult{query: "DELETE FROM t WHERE (tenant=$1) AND (id=$2) RETURNING id,a", args: []any{10, 20}})
		assertBuild(t, clone, buildResult{query: "DELETE FROM t WHERE (tenant=$1) AND (NOT (id=$2)) RETURNING id,b", args: []any{10, 30}})
	})
}

func TestWriteStatementCloneBoundValues(t *testing.T) {
	t.Parallel()
	ids := []int{1, 2}
	insert := Insert("t").Set("ids", ids)
	update := Update("t").Set("ids", ids).Where("id=ANY($)", ids)
	insertValues := Insert("t").SetValues(Map{"ids": ids})
	updateValues := Update("t").SetValues(Map{"ids": ids}).Where("id=ANY($)", ids)
	del := Delete("t").Where("id=ANY($)", ids)
	insertCopy, updateCopy, deleteCopy := insert.Clone(), update.Clone(), del.Clone()
	insertValuesCopy, updateValuesCopy := insertValues.Clone(), updateValues.Clone()
	ids[0] = 3
	for _, stmt := range []Builder{insert, insertCopy, insertValues, insertValuesCopy} {
		assertBuild(t, stmt, buildResult{query: "INSERT INTO t (ids) VALUES ($1)", args: []any{[]int{3, 2}}})
	}
	for _, stmt := range []Builder{update, updateCopy, updateValues, updateValuesCopy} {
		assertBuild(t, stmt, buildResult{query: "UPDATE t SET ids=$1 WHERE (id=ANY($2))", args: []any{[]int{3, 2}, []int{3, 2}}})
	}
	for _, stmt := range []Builder{del, deleteCopy} {
		assertBuild(t, stmt, buildResult{query: "DELETE FROM t WHERE (id=ANY($1))", args: []any{[]int{3, 2}}})
	}
}

func TestWriteStatementCloneConcurrentReuse(t *testing.T) {
	t.Parallel()
	insert := Insert("t").Set("a", 1).Returning("id")
	update := Update("t").Set("a", 1).Where("tenant=$", 10).Returning("id")
	del := Delete("t").Where("tenant=$", 10).Returning("id")
	for i := range 16 {
		t.Run("clone", func(t *testing.T) {
			t.Parallel()
			assertBuild(t, insert.Clone().Set("a", i).Returning("a"), buildResult{query: "INSERT INTO t (a) VALUES ($1) RETURNING id,a", args: []any{i}})
			assertBuild(t, update.Clone().Set("a", i).Where("id=$", i).Returning("a"), buildResult{query: "UPDATE t SET a=$1 WHERE (tenant=$2) AND (id=$3) RETURNING id,a", args: []any{i, 10, i}})
			assertBuild(t, del.Clone().Where("id=$", i).Returning("a"), buildResult{query: "DELETE FROM t WHERE (tenant=$1) AND (id=$2) RETURNING id,a", args: []any{10, i}})
			assertBuild(t, insert, buildResult{query: "INSERT INTO t (a) VALUES ($1) RETURNING id", args: []any{1}})
			assertBuild(t, update, buildResult{query: "UPDATE t SET a=$1 WHERE (tenant=$2) RETURNING id", args: []any{1, 10}})
			assertBuild(t, del, buildResult{query: "DELETE FROM t WHERE (tenant=$1) RETURNING id", args: []any{10}})
		})
	}
}
