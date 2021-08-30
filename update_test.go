package pql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdate(t *testing.T) {
	q, a := Update("t").Set("c1", 1).Set("c2", 2).Build()
	assert.Contains(t, []string{"UPDATE t SET c1=$1,c2=$2", "UPDATE t SET c2=$1,c1=$2"}, q)
	assert.ElementsMatch(t, []interface{}{1, 2}, a)

	q, a = Update("t").Set("c1", 1).Where("c2").Eq(2).Build()
	assert.Equal(t, "UPDATE t SET c1=$1 WHERE (c2=$2)", q)
	assert.Equal(t, []interface{}{1, 2}, a)
}
