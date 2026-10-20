package pql_test

import (
	"fmt"

	"github.com/gentam/pql"
)

func ExampleSelect() {
	s := pql.Select("id", "name").From("users")
	s.Where("tenant_id=$", 42)
	s.Desc("created_at").Limit(10)
	query, args, err := s.Build()
	if err != nil {
		panic(err)
	}
	fmt.Println(query)
	fmt.Println(args)

	// Output:
	// SELECT id,name FROM users WHERE (tenant_id=$1) ORDER BY created_at DESC LIMIT 10
	// [42]
}

func ExampleInsert() {
	i := pql.Insert("users").Returning("id")
	i.Set("status", "ACTIVE")
	i.Set("tenant_id", 42)
	query, args, err := i.Build()
	if err != nil {
		panic(err)
	}
	fmt.Println(query)
	fmt.Println(args)

	// Output:
	// INSERT INTO users (status,tenant_id) VALUES ($1,$2) RETURNING id
	// [ACTIVE 42]
}

func ExampleWhere() {
	filter := pql.Where("tenant_id=$ AND role in ($,$)", 42, "ADMIN", "MANAGER")

	query, args, err := pql.Select("count(*)").From("users").
		Apply(filter).Build()
	if err != nil {
		panic(err)
	}
	fmt.Println(query)
	fmt.Println(args)

	m := pql.Map{"role": "STAFF", "status": "INACTIVE"}
	query, args, err = pql.Update("users").Values(m).
		Apply(filter).Build()
	if err != nil {
		panic(err)
	}
	fmt.Println(query)
	fmt.Println(args)

	// Output:
	// SELECT count(*) FROM users WHERE (tenant_id=$1 AND role in ($2,$3))
	// [42 ADMIN MANAGER]
	// UPDATE users SET role=$1,status=$2 WHERE (tenant_id=$3 AND role in ($4,$5))
	// [STAFF INACTIVE 42 ADMIN MANAGER]
}

func ExampleDelete() {
	d := pql.Delete("sessions")
	d.Where("expires_at < (now() - interval '12h')")
	d.Returning("id")

	query, _, err := d.Build()
	if err != nil {
		panic(err)
	}
	fmt.Println(query)

	// Output:
	// DELETE FROM sessions WHERE (expires_at < (now() - interval '12h')) RETURNING id
}

func ExampleOrder_Reversed() {
	order := pql.Desc("updated_at", pql.NullsLast)

	query, _, err := pql.Select().From("events").
		SetOrders(order).
		Build()
	if err != nil {
		panic(err)
	}

	reversedQuery, _, err := pql.Select().From("events").
		SetOrders(order.Reversed()).
		Build()
	if err != nil {
		panic(err)
	}

	fmt.Println(query)
	fmt.Println(reversedQuery)

	// Output:
	// SELECT * FROM events ORDER BY updated_at DESC NULLS LAST
	// SELECT * FROM events ORDER BY updated_at ASC NULLS FIRST
}

func ExampleSelectStmt_Where() {
	query, args, err := pql.Select("id").From("users").
		Where("tenant_id=$", 42).Where("active").Desc("id").Limit(20).Build()
	if err != nil {
		panic(err)
	}
	fmt.Println(query)
	fmt.Println(args)

	// Output:
	// SELECT id FROM users WHERE (tenant_id=$1) AND (active) ORDER BY id DESC LIMIT 20
	// [42]
}

func ExampleSelectStmt_WhereNot() {
	query, args, err := pql.Select("id").From("users").
		WhereNot("status = $ OR hidden", "ARCHIVED").Where("active").Build()
	if err != nil {
		panic(err)
	}
	fmt.Println(query)
	fmt.Println(args)

	// Output:
	// SELECT id FROM users WHERE (NOT (status = $1 OR hidden)) AND (active)
	// [ARCHIVED]
}

func ExampleWhereExpr() {
	base := pql.Where("tenant_id=$", 42)
	var copied pql.WhereExpr = base
	base = base.And(pql.Where("active"))
	for _, expr := range []pql.WhereExpr{copied, base} {
		query, args, err := pql.Select("id").From("users").Apply(expr).Build()
		if err != nil {
			panic(err)
		}
		fmt.Println(query)
		fmt.Println(args)
	}
	// Output:
	// SELECT id FROM users WHERE (tenant_id=$1)
	// [42]
	// SELECT id FROM users WHERE ((tenant_id=$1) AND (active))
	// [42]
}

func ExampleSelectStmt_Apply() {
	filters := []pql.WhereExpr{
		pql.Where("active"),
		pql.WhereNot("private"),
	}
	query, args, err := pql.Select("id").From("users").
		Where("tenant_id=$", 42).Apply(filters...).Build()
	if err != nil {
		panic(err)
	}
	fmt.Println(query)
	fmt.Println(args)

	// Output:
	// SELECT id FROM users WHERE (tenant_id=$1) AND (active) AND (NOT (private))
	// [42]
}

func ExampleWhere_jsonb() {
	filter := pql.Where("payload ? $", "request_id")
	query, args, err := pql.Select("id").From("events").Apply(filter).Build()
	if err != nil {
		panic(err)
	}
	fmt.Println(query)
	fmt.Println(args)

	// Output:
	// SELECT id FROM events WHERE (payload ? $1)
	// [request_id]
}

func ExampleWhereNot() {
	visible := pql.WhereNot("payload ? $", "deleted")
	query, args, err := pql.Select("id").From("events").Apply(visible).Build()
	if err != nil {
		panic(err)
	}
	fmt.Println(query)
	fmt.Println(args)
	// Output:
	// SELECT id FROM events WHERE (NOT (payload ? $1))
	// [deleted]
}

func ExampleAnd() {
	base := pql.Where("tenant_id=$", 42)
	visible := pql.And(base, pql.Or(
		pql.Where("owner_id=$", 7),
		pql.Not(pql.Where("private")),
	))

	for _, filter := range []pql.WhereExpr{base, visible} {
		query, args, err := pql.Select("id").From("items").Apply(filter).Build()
		if err != nil {
			panic(err)
		}
		fmt.Println(query)
		fmt.Println(args)
	}

	// Output:
	// SELECT id FROM items WHERE (tenant_id=$1)
	// [42]
	// SELECT id FROM items WHERE ((tenant_id=$1) AND ((owner_id=$2) OR (NOT (private))))
	// [42 7]
}

func ExampleWhereExpr_And() {
	base := pql.Where("tenant_id = $", 42)
	visible := base.And(
		pql.Where("owner_id = $", 7).Or(pql.Where("private").Not()),
	)
	for _, filter := range []pql.WhereExpr{base, visible} {
		query, args, err := pql.Select("id").From("items").Apply(filter).Build()
		if err != nil {
			panic(err)
		}
		fmt.Println(query)
		fmt.Println(args)
	}

	// Output:
	// SELECT id FROM items WHERE (tenant_id = $1)
	// [42]
	// SELECT id FROM items WHERE ((tenant_id = $1) AND ((owner_id = $2) OR (NOT (private))))
	// [42 7]
}
