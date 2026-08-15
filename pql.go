// Package pql helps build PostgreSQL queries.
package pql

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Map map[string]any

var pool *pgxpool.Pool

func Init(p *pgxpool.Pool) {
	pool = p
}

func (ss *SelectStmt) Query(ctx context.Context) (pgx.Rows, error) {
	query, args, err := ss.Build()
	if err != nil {
		return nil, err
	}
	return pool.Query(ctx, query, args...)
}

func (ss *SelectStmt) QueryRow(ctx context.Context) (pgx.Row, error) {
	query, args, err := ss.Build()
	if err != nil {
		return nil, err
	}
	return pool.QueryRow(ctx, query, args...), nil
}

func (us *UpdateStmt) Exec(ctx context.Context) error {
	query, args, err := us.Build()
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, query, args...)
	return err
}

func (us *UpdateStmt) ExecRet(ctx context.Context) (pgx.Row, error) {
	query, args, err := us.Build()
	if err != nil {
		return nil, err
	}
	return pool.QueryRow(ctx, query, args...), nil
}

func (ds *DeleteStmt) Exec(ctx context.Context) error {
	query, args, err := ds.Build()
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, query, args...)
	return err
}

func (ds *DeleteStmt) ExecRet(ctx context.Context) (pgx.Row, error) {
	query, args, err := ds.Build()
	if err != nil {
		return nil, err
	}
	return pool.QueryRow(ctx, query, args...), nil
}

func (is *InsertStmt) Exec(ctx context.Context) error {
	query, args, err := is.Build()
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, query, args...)
	return err
}

func (is *InsertStmt) ExecRet(ctx context.Context) (pgx.Row, error) {
	query, args, err := is.Build()
	if err != nil {
		return nil, err
	}
	return pool.QueryRow(ctx, query, args...), nil
}

func buildReturning(b *strings.Builder, returning []string) {
	b.WriteString(" RETURNING ")
	b.WriteString(strings.Join(returning, ","))
}
