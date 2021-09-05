package pql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdate(t *testing.T) {
	q, a := Update("t").Set("c1", 1).Set("c2", 2).Build()
	assert.Contains(t, []string{"UPDATE t SET c1=$1,c2=$2", "UPDATE t SET c2=$1,c1=$2"}, q)
	assert.ElementsMatch(t, []interface{}{1, 2}, a)

	q, a = Update("t").Values(Map{"c1": 1, "c2": 2}).Build()
	assert.Contains(t, []string{"UPDATE t SET c1=$1,c2=$2", "UPDATE t SET c2=$1,c1=$2"}, q)
	assert.ElementsMatch(t, []interface{}{1, 2}, a)

	s := Update("t").Set("c1", 1)
	s.Where("c2").Eq(2)
	s.WhereNot("c3").Neq(3)
	s.Returning("c1")
	q, a = s.Build()
	assert.Equal(t, "UPDATE t SET c1=$1 WHERE (c2=$2) AND (NOT c3<>$3) RETURNING c1", q)
	assert.Equal(t, []interface{}{1, 2, 3}, a)

	s = Update("t").Set("c0", 0)
	w1 := Where("c1").Eq(1)
	w2 := Where("c2").Neq(2).Or("c3").IsNull()
	q, a = s.Apply(w1).Apply(w2).Build()
	assert.Equal(t, "UPDATE t SET c0=$1 WHERE (c1=$2) AND (c2<>$3 OR (c3 IS NULL))", q)
	assert.Equal(t, []interface{}{0, 1, 2}, a)
}
