package pql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInsert(t *testing.T) {
	q, a := Insert("t").Set("c1", 1).Set("c2", 2).Build()
	assert.Contains(t, []string{"INSERT INTO t (c1,c2) VALUES ($1,$2)", "INSERT INTO t (c2,c1) VALUES ($1,$2)"}, q)
	assert.ElementsMatch(t, []interface{}{1, 2}, a)
}
