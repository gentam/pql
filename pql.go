// Package pql helps build PostgreSQL queries.
package pql

import "strings"

type Map map[string]interface{}

func buildReturning(sb *strings.Builder, returning []string) {
	sb.WriteString(" RETURNING ")
	for i, col := range returning {
		if i != 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(col)
	}
}
