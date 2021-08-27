package pql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSelect(t *testing.T) {
	q := Select("a", "1", "now()").Build()
	assert.Equal(t, "SELECT a,1,now()", q)
}
