package pql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSelect(t *testing.T) {
	q1 := Select().From("t").Build()
	q2 := Select("a", "1", "now()").From("t").Build()
	assert.Equal(t, "SELECT * FROM t", q1)
	assert.Equal(t, "SELECT a,1,now() FROM t", q2)
}
