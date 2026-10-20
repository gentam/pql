# pql

`pql` is a small PostgreSQL query builder for Go. It combines trusted SQL
fragments with bound values and returns SQL plus positional arguments for
[pgx v5](https://github.com/jackc/pgx).

It was inspired by <https://github.com/jackc/pgsql>.

## Install

Requires Go 1.26 or later and pgx v5.

```sh
go get github.com/gentam/pql
```

## Build a query

Use `Select`, `Insert`, `Update`, or `Delete` to create a statement. `Build`
returns SQL, arguments, and a validation error without connecting to a database.

```go
s := pql.Select("id", "name").From("users")
s.Where("tenant_id = $", 42)
s.Desc("created_at").Limit(10)

query, args, err := s.Build()
if err != nil {
	return err
}

// query: SELECT id,name FROM users WHERE (tenant_id = $1) ORDER BY created_at DESC LIMIT 10
// args:  []any{42}
```

Write complete WHERE conditions in SQL with unnumbered `$` placeholders for
values. They become `$1`, `$2`, etc.; quoted text and comments are preserved.
PostgreSQL operators such as the JSONB `?` operator can be used directly.

Table names, column names, and SQL expressions must be trusted: they are not
quoted or escaped. Only values supplied through WHERE placeholders, `Set`,
`Values`, `SetValues`, or keyset helpers become bound parameters.

## Execute statements

Pass a connection, pool, or transaction implementing `pql.Querier` to execute a
statement. To omit that argument, configure `pql.SetDefaultPool(pool)` at startup,
before executing statements concurrently.

| Method | Statements | Returns |
| --- | --- | --- |
| `Query` | Select, Insert, Update, Delete | `(pgx.Rows, error)` |
| `QueryRow` | Select, Insert, Update, Delete | `pgx.Row` |
| `Exec` | Insert, Update, Delete | `(pgconn.CommandTag, error)` |
| `Exists` | Select | `(bool, error)` |

Use `Returning` on writes to specify result columns. `QueryRow` reads the first
row and reports all errors through `Scan`, including build errors and
`pgx.ErrNoRows` when no row is returned:

```go
var id int64
err := pql.Insert("users").Set("name", "Alice").Returning("id").
	QueryRow(ctx, conn).Scan(&id)
if err != nil {
	return err
}
```

Use `Query` for multiple rows. Close the rows and check `rows.Err()`, or use
`pgx.CollectRows`, which handles both:

```go
rows, err := pql.Delete("sessions").Where("expires_at < now()").
	Returning("id").Query(ctx, conn)
if err != nil {
	return err
}
ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
if err != nil {
	return err
}
```

Use `Exec` when no result rows are needed; its command tag exposes `RowsAffected()`:

```go
tag, err := pql.Update("users").Set("active", true).Where("id = $", 7).Exec(ctx, conn)
if err != nil {
	return err
}
```

INSERT and UPDATE also accept `Values(pql.Map{...})`; `pql.Map` is a shorter
spelling for map literals. `Set` and `Values` merge assignments, with later values
replacing earlier values for the same column. `SetValues(m)` copies and replaces
all assignments. Passing nil or an empty map clears them; `Build` requires at
least one assignment.

`SelectStmt.Exists` tests whether the query returns any rows, preserving its
LIMIT/OFFSET. Execution methods validate the statement before contacting the database.

## Compose conditions

Statement `Where`, `WhereNot`, and `Apply` append conditions with AND.
`Apply(exprs...)` accepts individual conditions or a slice; with no arguments
it leaves the WHERE clause unchanged.
`WhereNot` negates the entire supplied SQL condition. For reusable conditions,
combine `pql.Where` expressions with `And`, `Or`, and `Not`:

```go
filter := pql.And(
	pql.Where("tenant_id = $", 42),
	pql.Or(pql.Where("owner_id = $", 7), pql.Not(pql.Where("private"))),
)
stmt := pql.Select("id").From("items").Apply(filter)
// WHERE ((tenant_id = $1) AND ((owner_id = $2) OR (NOT (private))))
```

`WhereExpr` values are immutable and reusable. The methods `expr.And(...)`,
`expr.Or(...)`, and `expr.Not()` return new expressions; retain their results.
Finish composing a condition before applying it. `SetWhere(exprs...)` replaces
all conditions, and `SetWhere()` removes the WHERE clause.

Empty SQL and zero-valued `WhereExpr` inputs are build errors. For optional
filters, branch before adding them. A nil bound value represents SQL NULL;
use `IS NULL` when testing for NULL.

## Derive statements

All four statement types are mutable. Use `Clone` to derive independent queries:

```go
base := pql.Select("id", "title").From("items").Where("status = $", "OPEN")
list := base.Clone().Desc("id").Limit(20)
count := base.Clone().SetColumns("count(*)")
```

`SelectStmt.Select` and `Returning` append result expressions. `SetColumns`,
`SetOrders`, and `SetReturning` replace their lists; with no arguments they clear
the list. An empty SELECT list means `*`.
`OrderBy(terms...)` appends existing `OrderTerm` values; `Asc`, `Desc`, and `Order`
construct and append individual terms. Calling `OrderBy()` leaves the ordering
unchanged. For NULL placement, the last option wins; `NullsDefault` (0), also used
when omitted, leaves placement to PostgreSQL.
`ClearLimit` and `ClearOffset` remove pagination settings; an explicit `0` keeps
the corresponding clause.

Supplied lists are copied, and `Clone` copies mutable statement containers.
Bound values such as slices, maps, and pointers are not deep-copied. Do not
modify source statements or bound values during concurrent builds or clones.

## Keyset pagination

Use the same `OrderTerm` definitions for sorting and cursor conditions:

```go
orders := []pql.OrderTerm{
	pql.Desc("updated_at", pql.NullsLast),
	pql.Asc("id"),
}
page := pql.Select("id", "updated_at").From("items").
	SetOrders(orders...).
	Apply(pql.After(orders, lastUpdatedAt, lastID)).
	Limit(20)
```

Supply one cursor value per order, including a unique tie-breaker such as `id`.
Mixed directions and NULL placement are respected; pass `nil` for SQL NULL.
Use stable scalar expressions valid in WHERE and cursor values pgx can encode
for their SQL types.

`After` and `Before` exclude the boundary; `AtOrAfter` and `AtOrBefore` include it.
These helpers create conditions without changing the statement's ordering or limit.

The runnable [pagination example](examples/pagination/README.md) covers first,
next, and previous pages, reversed ordering, and date/NULL cursor handling.

## API reference and examples

For complete method contracts, validation rules, and placeholder syntax, run
`go doc -all github.com/gentam/pql`. Runnable examples are in
[example_test.go](./example_test.go).

## Tests

Run `go test ./...` for unit tests and examples. Set `PQL_TEST_DATABASE_URL` to a
PostgreSQL connection string to also run integration tests, which use temporary tables.
