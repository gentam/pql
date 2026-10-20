package pql

import (
	"context"
	"os"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestWherePlaceholdersPostgres(t *testing.T) {
	t.Parallel()
	url := os.Getenv("PQL_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set PQL_TEST_DATABASE_URL to test placeholders against PostgreSQL")
	}
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["standard_conforming_strings"] = "on"
	db, err := pgxpool.NewWithConfig(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	for _, tt := range []struct {
		name string
		from string
		expr string
		arg  any
	}{
		{"ordinary string", "", `'$' = $`, "$"},
		{"ordinary backslash", "", `'\' = $`, `\`},
		{"escaped quote", "", `E'it\'s $' = $`, "it's $"},
		{"escaped backslash", "", `E'\\$' = $`, `\$`},
		{"escape continuation", "", "E'a'\n" + `'\'$' = $`, "a'$"},
		{"escape continuation after comment", "", "E'a' -- $\r\n" + `'\'$' = $`, "a'$"},
		{"unicode string", "", `U&'price\0024' = $`, "price$"},
		{"custom unicode escape", "", `U&'price$0024' UESCAPE '$' = $`, "price$"},
		{"dollar quote", "", `$$price $1$$ = $`, "price $1"},
		{"tagged dollar quote", "", `$tag$price $$ $tag$ = $`, "price $$ "},
		{"non-ASCII tag", "", `$引用$価格$引用$ = $`, "価格"},
		{"quoted identifier", `(VALUES (42)) AS t("price$")`, `"price$" = $`, 42},
		{"unquoted identifier", `(VALUES (42)) AS t(price$usd)`, `price$usd = $`, 42},
		{"unicode identifier", `(VALUES (42)) AS t("price$")`, `U&"price\0024" = $`, 42},
		{"non-ASCII identifier", `(VALUES (42)) AS t(価格$円)`, `価格$円 = $`, 42},
		{"comments", "", "/* $ /* $ */ $ */ $::boolean -- $", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stmt := Select("true").Apply(Where(tt.expr, tt.arg))
			if tt.from != "" {
				stmt.From(tt.from)
			}
			var matched bool
			row, err := stmt.QueryRow(t.Context(), db)
			if err != nil {
				t.Fatal(err)
			}
			if err := row.Scan(&matched); err != nil || !matched {
				t.Fatalf("Scan() = %v, matched = %v; want true", err, matched)
			}
		})
	}
}

func TestWhereCompositionPostgres(t *testing.T) {
	t.Parallel()
	url := os.Getenv("PQL_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set PQL_TEST_DATABASE_URL to test condition composition against PostgreSQL")
	}
	conn, err := pgx.Connect(t.Context(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close(context.Background()) })
	// Rows distinguish both grouping choices and SQL's three-valued logic.
	const from = `(VALUES
		(1, true, false, true),
		(2, false, true, true),
		(3, true, true, false),
		(4, NULL::boolean, false, true),
		(5, true, NULL::boolean, false),
		(6, false, false, false)
	) AS t(id,a,b,c)`
	a, b, c := Where("a"), Where("b"), Where("c")
	for _, tt := range []struct {
		name string
		w    WhereExpr
		want []int
	}{
		{"and with or", And(a, Or(b, c)), []int{1, 3}},
		{"or with and", Or(And(a, b), c), []int{1, 2, 3, 4}},
		{"not entire or", Not(Or(a, b)), []int{6}},
		{"not entire and", Not(And(a, b)), []int{1, 2, 4, 6}},
		{"not null remains unknown", Not(Where("$::boolean", nil)), nil},
		{"empty and", And(), []int{1, 2, 3, 4, 5, 6}},
		{"empty or", Or(), nil},
		{"negated empty or", Not(Or()), []int{1, 2, 3, 4, 5, 6}},
		{"bound values and comments", And(Where("id>$ -- lower", 1), Not(Where("id>$ -- upper", 3))), []int{2, 3}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rows, err := Select("id").From(from).Apply(tt.w).Asc("id").Query(t.Context(), conn)
			if err != nil {
				t.Fatal(err)
			}
			got, err := pgx.CollectRows(rows, pgx.RowTo[int])
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("matched %v, want %v", got, tt.want)
			}
		})
	}
}
