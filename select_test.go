package pql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSelect(t *testing.T) {
	q1 := Select("a", "1", "now()").Build()
	q2 := Select().From("t").Build()
	q3 := Select().From("t").Where("true").Build()
	assert.Equal(t, "SELECT a,1,now()", q1)
	assert.Equal(t, "SELECT * FROM t", q2)
	assert.Equal(t, "SELECT * FROM t WHERE true", q3)
}
