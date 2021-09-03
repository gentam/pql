package pql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWhere(t *testing.T) {
	q, a := Where("c").Build()
	assert.Equal(t, "error: cannot build detatched WHERE clause", q)
	assert.Nil(t, a)
}
