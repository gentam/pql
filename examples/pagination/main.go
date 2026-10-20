// Run with PQL_TEST_DATABASE_URL set: go run ./examples/pagination
// The example creates only a temporary table, removed when the connection closes.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"slices"
	"time"

	"github.com/gentam/pql"
	"github.com/jackc/pgx/v5"
)

type item struct {
	ID      int
	DueDate *time.Time `db:"due_date"`
}

func main() {
	url := os.Getenv("PQL_TEST_DATABASE_URL")
	if url == "" {
		log.Fatal("set PQL_TEST_DATABASE_URL to a PostgreSQL connection string")
	}
	if err := run(context.Background(), url, os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, url string, out io.Writer) error {
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, `
		CREATE TEMP TABLE pql_pagination_example (id integer PRIMARY KEY, due_date date);
		INSERT INTO pql_pagination_example VALUES
			(1, '2026-01-10'), (2, '2026-01-10'), (3, '2026-01-11'), (4, NULL)`)
	if err != nil {
		return err
	}

	// Keep the display order fixed when constructing both After and Before.
	orders := []pql.OrderTerm{pql.Asc("due_date", pql.NullsLast), pql.Asc("id")}
	base := pql.Select("id", "due_date").From("pql_pagination_example")
	fetch := func(query *pql.SelectStmt) ([]item, error) {
		rows, err := query.Query(ctx, conn)
		if err != nil {
			return nil, err
		}
		return pgx.CollectRows(rows, pgx.RowToStructByName[item])
	}

	first, err := fetch(base.Clone().SetOrders(orders...).Limit(2))
	if err != nil {
		return err
	}
	if len(first) == 0 {
		return nil
	}

	// Use values from the last row, in the same order as the OrderTerms.
	// DueDate is *time.Time: pgx encodes its date value, or SQL NULL when nil.
	last := first[len(first)-1]
	next, err := fetch(base.Clone().SetOrders(orders...).
		Apply(pql.After(orders, last.DueDate, last.ID)).Limit(2))
	if err != nil {
		return err
	}
	if len(next) == 0 {
		return nil
	}

	// Fetch the nearest preceding rows by reversing the database order.
	// The boundary is the first row of the current page.
	start := next[0]
	previous, err := fetch(base.Clone().SetOrders(pql.ReversedOrders(orders)...).
		Apply(pql.Before(orders, start.DueDate, start.ID)).Limit(2))
	if err != nil {
		return err
	}
	slices.Reverse(previous) // Return rows in the original display order.

	for _, page := range []struct {
		name string
		rows []item
	}{{"first", first}, {"next", next}, {"previous", previous}} {
		ids := make([]int, len(page.rows))
		for i, row := range page.rows {
			ids[i] = row.ID
		}
		if _, err := fmt.Fprintf(out, "%s: %v\n", page.name, ids); err != nil {
			return err
		}
	}
	return nil
}
