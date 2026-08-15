package pql_test

import (
	"fmt"

	"github.com/gentam/pql"
)

func ExampleSelect() {
	s := pql.Select("id", "name").From("users")
	s.Where("tenant_id").Eq(42)
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
	filter := pql.Where("tenant_id").Eq(42).
		And("role in (?,?)", "ADMIN", "MANAGER")

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
	// SELECT count(*) FROM users WHERE (tenant_id=$1 AND (role in ($2,$3)))
	// [42 ADMIN MANAGER]
	// UPDATE users SET role=$1,status=$2 WHERE (tenant_id=$3 AND (role in ($4,$5)))
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
