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
	s.Where("c1").Eq(1)
	s.Where("c2").Eq("2")
	s.WhereNot("c3").Eq(3)
	q, a = s.Build()
	assert.Equal(t, "SELECT * FROM t WHERE (c1=$1) AND (c2=$2) AND (NOT c3=$3)", q)
	assert.Equal(t, []interface{}{1, "2", 3}, a)

	s = Select().From("t")
	s.Where("c1").Or("c2").Eq(2).And("c3 = ?", 3)
	s.WhereNot("c4").And("c5 < ?", 5).Or("c6").Eq("6")
	q, a = s.Build()
	assert.Equal(t, "SELECT * FROM t WHERE (c1 OR (c2=$1 AND (c3 = $2))) AND (NOT c4 AND (c5 < $3 OR (c6=$4)))", q)
	assert.Equal(t, []interface{}{2, 3, 5, "6"}, a)
}

func TestSelectLimitOffset(t *testing.T) {
	q, _ := Select().From("t").Limit(1).Offset(10).Build()
	assert.Equal(t, "SELECT * FROM t LIMIT 1 OFFSET 10", q)
}
