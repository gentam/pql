// Package pql builds PostgreSQL statements with bound parameters for pgx.
//
// Use [Select], [Insert], [Update], or [Delete] to create a statement. Its Build
// method returns SQL and positional arguments without connecting to a database.
// Execution methods use a supplied [Querier] or the pool configured with
// [SetDefaultPool]. Validation errors are reported by Build or execution methods;
// QueryRow reports errors through Scan.
//
// Table names, column names, and SQL expressions are inserted verbatim, without
// quoting or escaping, and must be trusted. Trailing line comments are terminated
// with a newline so they do not consume subsequent SQL. Values supplied to Set,
// Values, SetValues, or [Where] placeholders are bound parameters.
//
// Write complete WHERE conditions in SQL with unnumbered $ placeholders.
// Statement Where and WhereNot methods append a condition with AND and return
// the statement. WhereNot negates the entire supplied SQL condition.
// [Where] and [WhereNot] create independent conditions; [And], [Or], and [Not] combine
// whole expressions without modifying their inputs. Their method equivalents
// also return new expressions; use the result of each call. Apply adds expressions
// to a statement with AND; with no arguments it leaves conditions unchanged.
// SetWhere replaces all WHERE conditions, combining
// its inputs with AND; calling it without arguments removes the WHERE clause.
// WhereExpr values can be copied and reused without cloning. Their zero value
// is invalid and is rejected when the statement is built.
//
// Statements are mutable. Methods that configure them modify the receiver.
// Use [SelectStmt.Clone], [InsertStmt.Clone], [UpdateStmt.Clone], or
// [DeleteStmt.Clone] to derive an independent statement; immutable conditions
// are shared. Column, order, condition, and argument lists are copied
// when supplied. Bound values such as slices, maps, and pointers are not
// deep-copied.
//
// Concurrent builds, clones, and condition composition are safe while source
// statements and bound values are not being modified. Callers must synchronize
// edits with concurrent reads or edits of the same objects.
package pql
