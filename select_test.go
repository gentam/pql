package pql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSelect(t *testing.T) {
	q1, a := Select("a", "1", "now()").Build()
	q2, _ := Select().From("t").Build()
	assert.Equal(t, "SELECT a,1,now()", q1)
	assert.Equal(t, "SELECT * FROM t", q2)
	assert.Empty(t, a)
}

func TestSelectWhere(t *testing.T) {
	q1, a := Select().From("t").Where("v").Eq(1).Build()
	assert.Equal(t, "SELECT * FROM t WHERE v=$1", q1)
	assert.Equal(t, []interface{}{1}, a)
}
