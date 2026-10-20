# Keyset pagination

[main.go](main.go) fetches first, next, and previous pages from a temporary table
containing dates and NULLs. The table is removed when the connection closes.

Run from the repository root:

```sh
PQL_TEST_DATABASE_URL='postgres://localhost/my_dev_db?sslmode=disable' go run ./examples/pagination
```

```text
first: [1 2]
next: [3 4]
previous: [1 2]
```

## Page boundaries and ordering

The example orders by `due_date ASC NULLS LAST`, then by the unique `id`.
Use the same order definitions for sorting and for cursor conditions, with
one cursor value per order. Order expressions must be usable in WHERE;
SELECT aliases and output column numbers are unsuitable.

For the next page, apply `After` with the last row's cursor values and fetch
with the original ordering.

For the previous page, apply `Before` with the original orders and the first
row's cursor values. Fetch with `ReversedOrders(orders)` to get the nearest
preceding rows, then use `slices.Reverse` to restore the display order.
`ReversedOrders` returns a copy with each term's direction and NULL placement
reversed, preserving the priority of the keys.

`After` and `Before` exclude the boundary; `AtOrAfter` and `AtOrBefore` include it.
The boundary helpers only create conditions; configure the statement's ordering
and limit separately.

## Cursor types

Cursor values must be encodable by pgx for the corresponding SQL expression's
type. pql binds them without decoding or converting serialized cursor data.
For a `date` column, use `time.Time` or `pgtype.Date`. For a timestamp, retain its
precision when encoding and decoding the cursor.

The example scans `due_date` into `*time.Time` and passes it directly to the
boundary helper. A nil pointer represents SQL NULL; an explicit `nil` cursor
value also represents SQL NULL.

When adapting the example for a serialized cursor, decode a `YYYY-MM-DD` date
field before constructing the next-page statement with its `base` and `orders`:

```go
dueDate, err := time.Parse(time.DateOnly, cursorDate)
if err != nil {
    return err
}
next := base.Clone().SetOrders(orders...).
    Apply(pql.After(orders, dueDate, cursorID)).Limit(2)
```

For a NULL date field, pass `nil` instead of parsing a date. Keep cursor values
in the same order as the ordering terms, including the unique `id` tie-breaker.
