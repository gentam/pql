package pql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdate(t *testing.T) {
	q, a := Update("t").Set("c1", 1).Set("c2", 2).Build()
	assert.Contains(t, []string{"UPDATE t SET c1=$1,c2=$2", "UPDATE t SET c2=$1,c1=$2"}, q)
	assert.ElementsMatch(t, []interface{}{1, 2}, a)

	s := Update("t").Set("c1", 1)
	s.Where("c2").Eq(2)
	s.WhereNot("c3").Neq(3)
	q, a = s.Build()
	assert.Equal(t, "UPDATE t SET c1=$1 WHERE (c2=$2) AND (NOT c3<>$3)", q)
	assert.Equal(t, []interface{}{1, 2, 3}, a)
}
