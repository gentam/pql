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
}
