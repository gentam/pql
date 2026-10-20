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

func ExampleInsertStmt_Values() {
	values := pql.Map{"name": "Alice", "tenant_id": 42}
	query, args, err := pql.Insert("users").Values(values).Build()
	if err != nil {
		panic(err)
	}
	fmt.Println(query)
	fmt.Println(args)
	// Output:
	// INSERT INTO users (name,tenant_id) VALUES ($1,$2)
	// [Alice 42]
}

func ExampleInsertStmt_SetValues() {
	base := pql.Insert("users").Values(pql.Map{"name": "Alice", "status": "DRAFT"}).Returning("id")
	query, args, err := base.Clone().SetValues(pql.Map{"name": "Bob"}).Build()
	if err != nil {
		panic(err)
	}
	fmt.Println(query)
	fmt.Println(args)
	// Output:
	// INSERT INTO users (name) VALUES ($1) RETURNING id
	// [Bob]
}

func ExampleUpdateStmt_Values() {
	values := pql.Map{"active": true, "name": "Alice"}
	query, args, err := pql.Update("users").Values(values).Where("id=$", 7).Build()
	if err != nil {
		panic(err)
	}
	fmt.Println(query)
	fmt.Println(args)
	// Output:
	// UPDATE users SET active=$1,name=$2 WHERE (id=$3)
	// [true Alice 7]
}

func ExampleUpdateStmt_SetValues() {
	base := pql.Update("users").Set("active", true).Where("id=$", 7).Returning("id")
	query, args, err := base.Clone().SetValues(pql.Map{"name": "Alice"}).Build()
	if err != nil {
		panic(err)
	}
	fmt.Println(query)
	fmt.Println(args)
	// Output:
	// UPDATE users SET name=$1 WHERE (id=$2) RETURNING id
	// [Alice 7]
}

func ExampleSelectStmt_Clone() {
	base := pql.Select("id", "title").From("items")
	base.Where("status=$", "OPEN")

	list := base.Clone().Desc("id").Limit(20)
	list.Where("id<$", 100)
	count := base.Clone().SetColumns("count(*)")
	exists := base.Clone().SetColumns("1").Limit(1)

	for _, stmt := range []*pql.SelectStmt{list, count, exists} {
		query, args, err := stmt.Build()
		if err != nil {
			panic(err)
		}
		fmt.Println(query)
		fmt.Println(args)
	}

	// Output:
	// SELECT id,title FROM items WHERE (status=$1) AND (id<$2) ORDER BY id DESC LIMIT 20
	// [OPEN 100]
	// SELECT count(*) FROM items WHERE (status=$1)
	// [OPEN]
	// SELECT 1 FROM items WHERE (status=$1) LIMIT 1
	// [OPEN]
}

func ExampleSelectStmt_SetColumns() {
	list := pql.Select("id", "title").From("items").Desc("id").Limit(20).Offset(40)
	list.Where("status=$", "OPEN")
	count := list.Clone().SetColumns("count(*)").SetOrders().ClearLimit().ClearOffset()

	for _, stmt := range []*pql.SelectStmt{list, count} {
		query, args, err := stmt.Build()
		if err != nil {
			panic(err)
		}
		fmt.Println(query)
		fmt.Println(args)
	}

	// Output:
	// SELECT id,title FROM items WHERE (status=$1) ORDER BY id DESC LIMIT 20 OFFSET 40
	// [OPEN]
	// SELECT count(*) FROM items WHERE (status=$1)
	// [OPEN]
}

func ExampleSelectStmt_Columns() {
	s := pql.Select("id", "name")
	cols := s.Columns()
	cols[1] = "display_name"
	fmt.Println(s.Columns())
	fmt.Println(s.Clone().SetColumns(cols...).Columns())
	fmt.Println(len(pql.Select().Columns()))
	fmt.Println(pql.Select("*").Columns())
	// Output:
	// [id name]
	// [id display_name]
	// 0
	// [*]
}

func ExampleSelectStmt_LimitValue() {
	s := pql.Select()
	fmt.Println(s.LimitValue())
	s.Limit(0)
	fmt.Println(s.LimitValue())
	s.ClearLimit()
	fmt.Println(s.LimitValue())
	// Output:
	// 0 false
	// 0 true
	// 0 false
}

func ExampleSelectStmt_OffsetValue() {
	s := pql.Select().Offset(20)
	fmt.Println(s.OffsetValue())
	s.ClearOffset()
	fmt.Println(s.OffsetValue())
	// Output:
	// 20 true
	// 0 false
}

func ExampleInsertStmt_Clone() {
	base := pql.Insert("users").Set("tenant_id", 42).Returning("id")
	for _, name := range []string{"Alice", "Bob"} {
		query, args, err := base.Clone().Set("name", name).Build()
		if err != nil {
			panic(err)
		}
		fmt.Println(query)
		fmt.Println(args)
	}
	// Output:
	// INSERT INTO users (name,tenant_id) VALUES ($1,$2) RETURNING id
	// [Alice 42]
	// INSERT INTO users (name,tenant_id) VALUES ($1,$2) RETURNING id
	// [Bob 42]
}

func ExampleUpdateStmt_Clone() {
	base := pql.Update("users").Where("tenant_id=$", 42).Returning("id")
	for _, active := range []bool{true, false} {
		query, args, err := base.Clone().Set("active", active).Build()
		if err != nil {
			panic(err)
		}
		fmt.Println(query)
		fmt.Println(args)
	}
	// Output:
	// UPDATE users SET active=$1 WHERE (tenant_id=$2) RETURNING id
	// [true 42]
	// UPDATE users SET active=$1 WHERE (tenant_id=$2) RETURNING id
	// [false 42]
}

func ExampleDeleteStmt_Clone() {
	base := pql.Delete("sessions").Where("tenant_id=$", 42).Returning("id")
	for _, userID := range []int{1, 2} {
		query, args, err := base.Clone().Where("user_id=$", userID).Build()
		if err != nil {
			panic(err)
		}
		fmt.Println(query)
		fmt.Println(args)
	}
	// Output:
	// DELETE FROM sessions WHERE (tenant_id=$1) AND (user_id=$2) RETURNING id
	// [42 1]
	// DELETE FROM sessions WHERE (tenant_id=$1) AND (user_id=$2) RETURNING id
	// [42 2]
}

func ExampleInsertStmt_SetReturning() {
	base := pql.Insert("users").Set("active", true).Returning("id")
	for _, stmt := range []*pql.InsertStmt{
		base.Clone().SetReturning("id", "name"),
		base.Clone().SetReturning(),
		base,
	} {
		query, _, err := stmt.Build()
		if err != nil {
			panic(err)
		}
		fmt.Println(query)
	}
	// Output:
	// INSERT INTO users (active) VALUES ($1) RETURNING id,name
	// INSERT INTO users (active) VALUES ($1)
	// INSERT INTO users (active) VALUES ($1) RETURNING id
}

func ExampleUpdateStmt_SetReturning() {
	base := pql.Update("users").Set("active", true).Where("id=$", 7).Returning("id")
	for _, stmt := range []*pql.UpdateStmt{
		base.Clone().SetReturning("id", "name"),
		base.Clone().SetReturning(),
		base,
	} {
		query, _, err := stmt.Build()
		if err != nil {
			panic(err)
		}
		fmt.Println(query)
	}
	// Output:
	// UPDATE users SET active=$1 WHERE (id=$2) RETURNING id,name
	// UPDATE users SET active=$1 WHERE (id=$2)
	// UPDATE users SET active=$1 WHERE (id=$2) RETURNING id
}

func ExampleDeleteStmt_SetReturning() {
	base := pql.Delete("users").Where("id=$", 7).Returning("id")
	for _, stmt := range []*pql.DeleteStmt{
		base.Clone().SetReturning("id", "name"),
		base.Clone().SetReturning(),
		base,
	} {
		query, _, err := stmt.Build()
		if err != nil {
			panic(err)
		}
		fmt.Println(query)
	}
	// Output:
	// DELETE FROM users WHERE (id=$1) RETURNING id,name
	// DELETE FROM users WHERE (id=$1)
	// DELETE FROM users WHERE (id=$1) RETURNING id
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

func ExampleSelectStmt_SetWhere() {
	base := pql.Select("id").From("users").Where("status=$", "OPEN")
	for _, stmt := range []*pql.SelectStmt{
		base.Clone().SetWhere(pql.Where("status=$", "ARCHIVED"), pql.Where("tenant_id=$", 42)),
		base.Clone().SetWhere(),
		base,
	} {
		query, args, err := stmt.Build()
		if err != nil {
			panic(err)
		}
		fmt.Println(query)
		fmt.Println(args)
	}
	// Output:
	// SELECT id FROM users WHERE (status=$1) AND (tenant_id=$2)
	// [ARCHIVED 42]
	// SELECT id FROM users
	// []
	// SELECT id FROM users WHERE (status=$1)
	// [OPEN]
}

func ExampleUpdateStmt_SetWhere() {
	base := pql.Update("users").Set("active", true).Where("status=$", "OPEN")
	for _, stmt := range []*pql.UpdateStmt{
		base.Clone().SetWhere(pql.Where("status=$", "ARCHIVED"), pql.Where("tenant_id=$", 42)),
		base.Clone().SetWhere(),
		base,
	} {
		query, args, err := stmt.Build()
		if err != nil {
			panic(err)
		}
		fmt.Println(query)
		fmt.Println(args)
	}
	// Output:
	// UPDATE users SET active=$1 WHERE (status=$2) AND (tenant_id=$3)
	// [true ARCHIVED 42]
	// UPDATE users SET active=$1
	// [true]
	// UPDATE users SET active=$1 WHERE (status=$2)
	// [true OPEN]
}

func ExampleDeleteStmt_SetWhere() {
	base := pql.Delete("users").Where("status=$", "OPEN")
	for _, stmt := range []*pql.DeleteStmt{
		base.Clone().SetWhere(pql.Where("status=$", "ARCHIVED"), pql.Where("tenant_id=$", 42)),
		base.Clone().SetWhere(),
		base,
	} {
		query, args, err := stmt.Build()
		if err != nil {
			panic(err)
		}
		fmt.Println(query)
		fmt.Println(args)
	}
	// Output:
	// DELETE FROM users WHERE (status=$1) AND (tenant_id=$2)
	// [ARCHIVED 42]
	// DELETE FROM users
	// []
	// DELETE FROM users WHERE (status=$1)
	// [OPEN]
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
