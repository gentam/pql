package pql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDelete(t *testing.T) {
	s := Delete("t")
	s.Where("c1").Eq(1)
	s.WhereNot("c2").IsNotNull()
	q, a := s.Build()
	assert.Equal(t, "DELETE FROM t WHERE (c1=$1) AND (NOT c2 IS NOT NULL)", q)
	assert.Equal(t, []interface{}{1}, a)

	w := Where("c0<=now()").Or("c0").IsNull()
	Delete("t").Apply(w).WhereNot("c1").Eq(1)
	q, a = w.Or("c2").Eq(2).Build()
	assert.Equal(t, "DELETE FROM t WHERE (c0<=now() OR (c0 IS NULL OR (c2=$1))) AND (NOT c1=$2)", q)
	assert.Equal(t, []interface{}{2, 1}, a)
}
