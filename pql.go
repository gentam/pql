// Package pql helps build PostgreSQL queries.
package pql

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type Map map[string]interface{}

var pool *pgxpool.Pool

func Init(p *pgxpool.Pool) {
	pool = p
}

func (ss *SelectStmt) Query(ctx context.Context) (pgx.Rows, error) {
	query, args := ss.Build()
	return pool.Query(ctx, query, args...)
}

func (ss *SelectStmt) QueryRow(ctx context.Context) pgx.Row {
	query, args := ss.Build()
	return pool.QueryRow(ctx, query, args...)
}

func (ss *SelectStmt) QueryFunc(ctx context.Context, scans []interface{}, f func(pgx.QueryFuncRow) error) error {
	query, args := ss.Build()
	_, err := pool.QueryFunc(ctx, query, args, scans, f)
	return err
}

func (us *UpdateStmt) Exec(ctx context.Context) error {
	query, args := us.Build()
	_, err := pool.Exec(ctx, query, args...)
	return err
}

func (us *UpdateStmt) ExecRet(ctx context.Context) pgx.Row {
	query, args := us.Build()
	return pool.QueryRow(ctx, query, args...)
}

func (ds *DeleteStmt) Exec(ctx context.Context) error {
	query, args := ds.Build()
	_, err := pool.Exec(ctx, query, args...)
	return err
}

func (ds *DeleteStmt) ExecRet(ctx context.Context) pgx.Row {
	query, args := ds.Build()
	return pool.QueryRow(ctx, query, args...)
}

func (is *InsertStmt) Exec(ctx context.Context) error {
	query, args := is.Build()
	_, err := pool.Exec(ctx, query, args...)
	return err
}

func (is *InsertStmt) ExecRet(ctx context.Context) pgx.Row {
	query, args := is.Build()
	return pool.QueryRow(ctx, query, args...)
}

func buildReturning(sb *strings.Builder, returning []string) {
	sb.WriteString(" RETURNING ")
	for i, col := range returning {
		if i != 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(col)
	}
}
