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
s.Where("tenant_id").Eq(42)
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

## WHERE expressions

Use operator methods for values:

```go
w := pql.Where("tenant_id").Eq(42).
	And("role in (?,?)", "ADMIN", "MANAGER")

query, args, err := pql.Select("count(*)").From("users").
	Apply(w).
	Build()
if err != nil {
	return err
}

// query: SELECT count(*) FROM users WHERE (tenant_id=$1 AND (role in ($2,$3)))
// args:  []any{42, "ADMIN", "MANAGER"}
```

`And` and `Or` group the remaining expression on their right. For example,
`pql.Where("a").And("b").Or("c")` builds the equivalent of
`a AND (b OR c)`.

`Apply` accepts a detached `WhereExpr`, allowing the expression to be applied
to `Select`, `Update`, or `Delete` statements. Passing `nil` is a no-op.
`WhereNot` prefixes an expression with `NOT`.

Available operators include:

| Method | SQL |
| --- | --- |
| `Eq`, `Neq` | `=`, `<>` |
| `Lt`, `Gt`, `Le`, `Ge` | `<`, `>`, `<=`, `>=` |
| `Like`, `Ilike` | `LIKE`, `ILIKE` |
| `Contains`, `ContainedBy` | `@>`, `<@` |
| `IsNull`, `IsNotNull` | `IS NULL`, `IS NOT NULL` |
| `And`, `Or` | boolean grouping |

Raw expressions may contain `?` placeholders. They are replaced with
PostgreSQL positional placeholders and appended to the statement arguments.

```go
s.Where("created_at BETWEEN ? AND ?", from, to)
```

When arguments are supplied, every `?` in the expression is treated as a
placeholder. With no arguments, the expression is left unchanged, so a trusted
PostgreSQL JSONB expression such as `event_payload ? 'request_id'` can be used
directly.

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
- empty WHERE conditions
- mismatched raw-expression placeholders and arguments
- invalid NULL ordering options

Always check the returned error before executing the SQL.

## SQL safety

Table names, column names, and raw SQL expressions are written into SQL as
provided. They are not quoted or escaped. Only values passed through operator
methods, `Set`, `Values`, or raw-expression `?` placeholders become pgx
arguments.

Do not place untrusted input in identifiers or raw SQL fragments.

Runnable examples are available in [example_test.go](./example_test.go).
