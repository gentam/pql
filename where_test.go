package pql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWhere(t *testing.T) {
	q, a := Where("c").Build()
	assert.Equal(t, "error: cannot build detatched WHERE clause", q)
	assert.Nil(t, a)

	s := Select()
	w := s.Where("c")
	assert.Equal(t, s, w.SS())
	assert.Nil(t, w.US())
	u := Update("t")
	w = u.Where("c")
	assert.Equal(t, u, w.US())
	assert.Nil(t, w.DS())
	d := Delete("t")
	w = d.Where("c")
	assert.Equal(t, d, w.DS())
	assert.Nil(t, w.SS())
}
