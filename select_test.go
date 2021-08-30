package pql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSelect(t *testing.T) {
	q, a := Select("a", "1", "now()").Build()
	assert.Equal(t, "SELECT a,1,now()", q)
	assert.Empty(t, a)

	q, _ = Select().From("t").Build()
	assert.Equal(t, "SELECT * FROM t", q)
}

func TestSelectWhere1(t *testing.T) {
	q, a := Select().From("t").Where("true").Build()
	assert.Equal(t, "SELECT * FROM t WHERE (true)", q)
	assert.Empty(t, a)

	q, a = Select().From("t").WhereNot("true").Build()
	assert.Equal(t, "SELECT * FROM t WHERE (NOT true)", q)
	assert.Empty(t, a)

	q, a = Select().From("t").Where("expire <= now()").Build()
	assert.Equal(t, "SELECT * FROM t WHERE (expire <= now())", q)
	assert.Empty(t, a)

	q, a = Select().From("t").Where("c > ?", 1).Build()
	assert.Equal(t, "SELECT * FROM t WHERE (c > $1)", q)
	assert.Equal(t, []interface{}{1}, a)

	s := Select().From("t1")
	s.WhereNot("c1 in (?,?,?)", 1, 2, 3)
	s.Where("c2 = any (select id from t2 where c <> ?)", "4")
	q, a = s.Build()
	assert.Equal(t, "SELECT * FROM t1 WHERE (NOT c1 in ($1,$2,$3)) AND (c2 = any (select id from t2 where c <> $4))", q)
	assert.Equal(t, []interface{}{1, 2, 3, "4"}, a)
}

func TestSelectWhere2(t *testing.T) {
	q, a := Select().From("t").Where("c").Eq(1).Build()
	assert.Equal(t, "SELECT * FROM t WHERE (c=$1)", q)
	assert.Equal(t, []interface{}{1}, a)

	s := Select().From("t")
	s.Where("ca").IsNull()
	s.Where("cb").IsNotNull()
	s.Where("c1").Eq(1)
	s.Where("c2").Neq(2)
	s.Where("c3").Lt(3)
	s.Where("c4").Gt(4)
	s.Where("c5").Le(5)
	s.Where("c6").Ge(6)
	s.Where("c7").Like("7")
	s.Where("c8").Ilike("8")
	s.Where("c9").Contains([]int{1, 2, 3})
	s.Where("c10").ContainedBy([]int{4, 5, 6})
	q, a = s.Build()
	assert.Equal(t, "SELECT * FROM t WHERE (ca IS NULL) AND (cb IS NOT NULL) AND (c1=$1) AND (c2<>$2) AND (c3<$3) AND (c4>$4) AND (c5<=$5) AND (c6>=$6) "+
		"AND (c7 LIKE $7) AND (c8 ILIKE $8) AND (c9@>$9) AND (c10<@$10)",
		q)
	assert.Equal(t, []interface{}{1, 2, 3, 4, 5, 6, "7", "8", []int{1, 2, 3}, []int{4, 5, 6}}, a)

	s = Select().From("t")
	s.Where("c1").Or("c2").Eq(2).And("c3 = ?", 3)
	s.WhereNot("c4").And("c5 < ?", 5).Or("c6").Eq("6")
	q, a = s.Build()
	assert.Equal(t, "SELECT * FROM t WHERE (c1 OR (c2=$1 AND (c3 = $2))) AND (NOT c4 AND (c5 < $3 OR (c6=$4)))", q)
	assert.Equal(t, []interface{}{2, 3, 5, "6"}, a)
}

func TestSelectOrder(t *testing.T) {
	q1, _ := Select().From("t").Asc("c").Build()
	q2, _ := Select().From("t").Desc("c").Build()
	assert.Equal(t, "SELECT * FROM t ORDER BY c ASC", q1)
	assert.Equal(t, "SELECT * FROM t ORDER BY c DESC", q2)
}

func TestSelectLimitOffset(t *testing.T) {
	q, _ := Select().From("t").Limit(1).Offset(10).Build()
	assert.Equal(t, "SELECT * FROM t LIMIT 1 OFFSET 10", q)
}
