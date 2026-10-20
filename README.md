# pql

`pql` is a small PostgreSQL query builder for Go. It combines trusted SQL
fragments with bound values and returns SQL plus positional arguments for
[pgx v5](https://github.com/jackc/pgx).

It was inspired by <https://github.com/jackc/pgsql>.

## Requirements

- Go 1.26 or later
- pgx v5

```sh
go get github.com/gentam/pql
```

## Build a query

Statements return SQL, arguments, and a build error.

```go
s := pql.Select("id", "name").From("users")
s.Where("tenant_id=$", 42)
s.Desc("created_at").Limit(10)

query, args, err := s.Build()
if err != nil {
	return err
}

// query: SELECT id,name FROM users WHERE (tenant_id=$1) ORDER BY created_at DESC LIMIT 10
// args:  []any{42}
```

`Insert`, `Update`, and `Delete` use the same pattern.

```go
i := pql.Insert("users").Returning("id")
i.Set("status", "ACTIVE")
i.Set("tenant_id", 42)

query, args, err := i.Build()
if err != nil {
	return err
}
// query: INSERT INTO users (status,tenant_id) VALUES ($1,$2) RETURNING id
// args:  []any{"ACTIVE", 42}
```

Both `Insert` and `Update` accept values individually with `Set`, or as a
`pql.Map` with `Values`. Calls to either method are cumulative and may be mixed
in any order; later values replace earlier values for the same column.

## Execute statements

`Query`, `QueryRow`, `Exec`, and `ExecRet` use the pool registered by `Init`
when no connection is supplied. An explicitly supplied `Querier`, such as a
`pgx.Tx` or another `*pgxpool.Pool`, takes precedence for that call.

## Compose conditions

Write complete WHERE conditions in SQL with unnumbered `$` placeholders for
values. They become `$1`, `$2`, etc.; quoted text and comments are preserved.
PostgreSQL operators such as the JSONB `?` operator can be used directly.

Statement `Where`, `WhereNot`, and `Apply` append conditions with AND.
`Apply(exprs...)` accepts individual conditions or a slice; with no arguments
it leaves the WHERE clause unchanged.
`WhereNot` negates the entire supplied SQL condition. For reusable conditions,
combine `pql.Where` expressions with `And`, `Or`, and `Not`:

```go
filter := pql.And(
	pql.Where("tenant_id=$", 42),
	pql.Or(pql.Where("owner_id=$", 7), pql.Not(pql.Where("private"))),
)
stmt := pql.Select("id").From("items").Apply(filter)
// WHERE ((tenant_id=$1) AND ((owner_id=$2) OR (NOT (private))))
```

`WhereExpr` values are immutable and reusable. The methods `expr.And(...)`,
`expr.Or(...)`, and `expr.Not()` return new expressions; retain their results.
Finish composing a condition before applying it. `SetWhere(exprs...)` replaces
all conditions, and `SetWhere()` removes the WHERE clause.

Empty SQL and zero-valued `WhereExpr` inputs are build errors. For optional
filters, branch before adding them. A nil bound value represents SQL NULL;
use `IS NULL` when testing for NULL.

## Ordering

`Asc` and `Desc` can specify PostgreSQL NULL placement:

```go
order := pql.Desc("updated_at", pql.NullsLast)
s := pql.Select().From("events").SetOrders(order)
```

An `Order` is a reusable value. `Reversed` returns the exact inverse without
modifying the original order, including an explicit NULL placement.

```go
reversed := order.Reversed()
// updated_at ASC NULLS FIRST
```

Without an explicit option, PostgreSQL's defaults apply: ascending order puts
NULLs last and descending order puts NULLs first.

## Errors

`Build` validates structural errors such as:

- missing tables for `Insert`, `Update`, and `Delete`
- missing values for `Insert` and `Update`
- empty or whitespace-only column, WHERE, RETURNING, and ORDER BY expressions
- non-empty, whitespace-only FROM expressions
- mismatched placeholders and arguments, numbered placeholders, and unclosed SQL text
- invalid NULL ordering options
- negative `Limit` or `Offset` values

Always check the returned error before executing the SQL.

## SQL safety

Table names, column names, and raw SQL expressions are written into SQL as
provided. They are not quoted or escaped. Only values passed through WHERE
placeholders, `Set`, or `Values` become pgx arguments.

Do not place untrusted input in identifiers or raw SQL fragments.

Runnable examples are available in [example_test.go](./example_test.go).
