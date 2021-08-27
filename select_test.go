package pql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSelect(t *testing.T) {
	q := Select("a", "1", "now()").From("t").Build()
	assert.Equal(t, "SELECT a,1,now() FROM t", q)
}
