package pql

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func numberPlaceholders(expr string, offset, numArgs int) (string, error) {
	sql, count, err := rewriteSQL(expr, offset, true)
	if err != nil {
		return "", err
	}
	if count != numArgs {
		return "", fmt.Errorf("pql: WHERE placeholder count %d does not match argument count %d", count, numArgs)
	}
	return sql, nil
}

// terminateLineComment keeps a fragment's final line comment from consuming
// SQL appended by the builder. Raw SQL is otherwise left unvalidated.
func terminateLineComment(expr string) string {
	if !strings.Contains(expr, "--") {
		return expr
	}
	sql, _, err := rewriteSQL(expr, 0, false)
	if err != nil {
		return expr
	}
	return sql
}

// rewriteSQL scans PostgreSQL tokens, optionally numbering placeholders, and
// terminates a trailing line comment with a newline. Ordinary strings follow
// standard_conforming_strings=on; only E strings treat backslashes as escapes.
// Untouched spans are copied verbatim.
func rewriteSQL(expr string, offset int, number bool) (string, int, error) {
	var b strings.Builder
	count, start := 0, 0
	for i := 0; i < len(expr); {
		switch c := expr[i]; {
		case (c == 'E' || c == 'e') && i+1 < len(expr) && expr[i+1] == '\'':
			i = quotedEnd(expr, i+1, '\'', true)
			if i < 0 {
				return "", 0, errors.New("pql: unterminated string in WHERE expression")
			}
		case c == '\'' || c == '"':
			i = quotedEnd(expr, i, c, false)
			if i < 0 {
				if c == '"' {
					return "", 0, errors.New("pql: unterminated quoted identifier in WHERE expression")
				}
				return "", 0, errors.New("pql: unterminated string in WHERE expression")
			}
		case c == '-' && i+1 < len(expr) && expr[i+1] == '-':
			i = lineCommentEnd(expr, i)
			if i == len(expr) {
				b.WriteString(expr[start:])
				b.WriteByte('\n')
				return b.String(), count, nil
			}
		case c == '/' && i+1 < len(expr) && expr[i+1] == '*':
			i = blockCommentEnd(expr, i)
			if i < 0 {
				return "", 0, errors.New("pql: unterminated block comment in WHERE expression")
			}
		case isSQLIdentStart(c):
			i++
			for i < len(expr) && isSQLIdentContinue(expr[i]) {
				i++
			}
		case c == '$':
			if end := dollarDelimiterEnd(expr, i); end >= 0 {
				delimiter := expr[i:end]
				closing := strings.Index(expr[end:], delimiter)
				if closing < 0 {
					return "", 0, errors.New("pql: unterminated dollar-quoted string in WHERE expression")
				}
				i = end + closing + len(delimiter)
				continue
			}
			if !number {
				i++
				continue
			}
			if i+1 < len(expr) && isSQLDigit(expr[i+1]) {
				return "", 0, errors.New("pql: numbered placeholders are not supported in WHERE expressions; use $")
			}
			if i+1 < len(expr) && isSQLIdentStart(expr[i+1]) {
				return "", 0, errors.New("pql: invalid WHERE placeholder; use $")
			}
			count++
			b.WriteString(expr[start:i])
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(offset + count))
			i++
			start = i
		default:
			i++
		}
	}
	if count == 0 {
		return expr, 0, nil
	}
	b.WriteString(expr[start:])
	return b.String(), count, nil
}

// quotedEnd returns the end of a quoted token, or -1 if it is unterminated.
func quotedEnd(expr string, start int, quote byte, escapes bool) int {
	for i := start + 1; i < len(expr); {
		switch {
		case escapes && expr[i] == '\\':
			i += 2
		case expr[i] == quote:
			if i+1 < len(expr) && expr[i+1] == quote {
				i += 2
				continue
			}
			if quote == '\'' {
				if next := stringContinuation(expr, i+1); next >= 0 {
					i = next + 1
					continue
				}
			}
			return i + 1
		default:
			i++
		}
	}
	return -1
}

// PostgreSQL continues strings across whitespace containing a newline, including
// line comments. Block comments do not participate in this lexical rule. The
// continued part of an E string keeps its backslash-escape behavior.
func stringContinuation(expr string, start int) int {
	newline := false
	for i := start; i < len(expr); {
		switch expr[i] {
		case ' ', '\t', '\f', '\v':
			i++
		case '\r', '\n':
			newline = true
			i++
		case '-':
			if i+1 >= len(expr) || expr[i+1] != '-' {
				return -1
			}
			i = lineCommentEnd(expr, i)
		case '\'':
			if newline {
				return i
			}
			return -1
		default:
			return -1
		}
	}
	return -1
}

func lineCommentEnd(expr string, start int) int {
	i := start + 2
	for i < len(expr) && expr[i] != '\r' && expr[i] != '\n' {
		i++
	}
	return i
}

func blockCommentEnd(expr string, start int) int {
	depth := 1
	for i := start + 2; i+1 < len(expr); {
		switch expr[i : i+2] {
		case "/*":
			depth++
			i += 2
		case "*/":
			depth--
			i += 2
			if depth == 0 {
				return i
			}
		default:
			i++
		}
	}
	return -1
}

func dollarDelimiterEnd(expr string, start int) int {
	i := start + 1
	if i < len(expr) && isSQLIdentStart(expr[i]) {
		i++
		for i < len(expr) && isSQLIdentContinue(expr[i]) && expr[i] != '$' {
			i++
		}
	}
	if i < len(expr) && expr[i] == '$' {
		return i + 1
	}
	return -1
}

// PostgreSQL's lexer accepts high-bit bytes in unquoted identifiers and tags.
func isSQLIdentStart(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || c >= 0x80
}

func isSQLIdentContinue(c byte) bool {
	return isSQLIdentStart(c) || isSQLDigit(c) || c == '$'
}

func isSQLDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
