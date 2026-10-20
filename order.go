package pql

import (
	"errors"
	"fmt"
	"strings"
)

// NullsOrder specifies where NULL values appear in an ORDER BY expression.
type NullsOrder uint8

const (
	// NullsFirst emits NULLS FIRST.
	NullsFirst NullsOrder = iota + 1
	// NullsLast emits NULLS LAST.
	NullsLast
)

// Order represents an ORDER BY expression.
type Order struct {
	col   string
	desc  bool
	nulls []NullsOrder
}

// Asc creates an ascending order for col.
func Asc(col string, nulls ...NullsOrder) Order {
	return newOrder(col, false, nulls...)
}

// Desc creates a descending order for col.
func Desc(col string, nulls ...NullsOrder) Order {
	return newOrder(col, true, nulls...)
}

func newOrder(col string, desc bool, nulls ...NullsOrder) Order {
	return Order{col: col, desc: desc, nulls: append([]NullsOrder(nil), nulls...)}
}

// Column returns the ORDER BY expression.
func (o Order) Column() string {
	return o.col
}

// IsDescending reports whether the order is descending.
func (o Order) IsDescending() bool {
	return o.desc
}

// NullsLast reports the effective NULL ordering, including PostgreSQL's
// default of NULLS LAST for ascending and NULLS FIRST for descending order.
func (o Order) NullsLast() bool {
	if len(o.nulls) == 1 {
		return o.nulls[0] == NullsLast
	}
	return !o.desc
}

// Reversed returns the exact inverse ordering without modifying o.
func (o Order) Reversed() Order {
	reversed := newOrder(o.col, !o.desc, o.nulls...)
	if len(reversed.nulls) == 1 {
		switch reversed.nulls[0] {
		case NullsFirst:
			reversed.nulls[0] = NullsLast
		case NullsLast:
			reversed.nulls[0] = NullsFirst
		}
	}
	return reversed
}

func buildOrder(b *strings.Builder, orders []Order) error {
	b.WriteString(" ORDER BY ")
	for i, order := range orders {
		if err := order.validate(); err != nil {
			return err
		}
		if i != 0 {
			b.WriteByte(',')
		}
		b.WriteString(terminateLineComment(order.col))
		if order.desc {
			b.WriteString(" DESC")
		} else {
			b.WriteString(" ASC")
		}

		if len(order.nulls) == 1 {
			if order.nulls[0] == NullsFirst {
				b.WriteString(" NULLS FIRST")
			} else {
				b.WriteString(" NULLS LAST")
			}
		}
	}
	return nil
}

func (o Order) validate() error {
	if strings.TrimSpace(o.col) == "" {
		return errors.New("pql: ORDER BY expression is required")
	}
	switch len(o.nulls) {
	case 0:
	case 1:
		if o.nulls[0] != NullsFirst && o.nulls[0] != NullsLast {
			return fmt.Errorf("pql: invalid NULLS order %d", o.nulls[0])
		}
	default:
		return fmt.Errorf("pql: ORDER BY accepts at most 1 NULLS option, got %d", len(o.nulls))
	}
	return nil
}
