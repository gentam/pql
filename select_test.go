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

func TestSelectWhere(t *testing.T) {
	q, a := Select().From("t").Where("true").Build()
	assert.Equal(t, "SELECT * FROM t WHERE (true)", q)
	assert.Empty(t, a)

	q, a = Select().From("t").WhereNot("true").Build()
	assert.Equal(t, "SELECT * FROM t WHERE (NOT true)", q)
	assert.Empty(t, a)

	q, a = Select().From("t").Where("v").Eq(1).Build()
	assert.Equal(t, "SELECT * FROM t WHERE (v=$1)", q)
	assert.Equal(t, []interface{}{1}, a)

	s := Select().From("t")
	s.Where("c1").Eq(1)
	s.Where("c2").Eq("2")
	s.WhereNot("c3").Eq(3)
	q, a = s.Build()
	assert.Equal(t, "SELECT * FROM t WHERE (c1=$1) AND (c2=$2) AND (NOT c3=$3)", q)
	assert.Equal(t, []interface{}{1, "2", 3}, a)

	s = Select().From("t")
	s.Where("c1").Or("c2").And("c3")
	s.WhereNot("c4").And("c5").Or("c6")
	q, _ = s.Build()
	assert.Equal(t, "SELECT * FROM t WHERE (c1 OR (c2 AND (c3))) AND (NOT c4 AND (c5 OR (c6)))", q)
}
