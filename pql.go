// Package pql helps build PostgreSQL queries.
package pql

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Map associates column expressions with values.
type Map map[string]any

var pool *pgxpool.Pool

var errPoolNotInitialized = errors.New("pql: pool is not initialized")

// Init sets the pool used by statement execution methods.
func Init(p *pgxpool.Pool) {
	pool = p
}

// Query executes the SELECT statement and returns its rows.
func (ss *SelectStmt) Query(ctx context.Context) (pgx.Rows, error) {
	query, args, err := ss.Build()
	if err != nil {
		return nil, err
	}
	p, err := initializedPool()
	if err != nil {
		return nil, err
	}
	return p.Query(ctx, query, args...)
}

// QueryRow executes the SELECT statement and returns one row.
func (ss *SelectStmt) QueryRow(ctx context.Context) (pgx.Row, error) {
	query, args, err := ss.Build()
	if err != nil {
		return nil, err
	}
	p, err := initializedPool()
	if err != nil {
		return nil, err
	}
	return p.QueryRow(ctx, query, args...), nil
}

// Exec executes the UPDATE statement.
func (us *UpdateStmt) Exec(ctx context.Context) error {
	query, args, err := us.Build()
	if err != nil {
		return err
	}
	p, err := initializedPool()
	if err != nil {
		return err
	}
	_, err = p.Exec(ctx, query, args...)
	return err
}

// ExecRet executes the UPDATE statement and returns one row.
func (us *UpdateStmt) ExecRet(ctx context.Context) (pgx.Row, error) {
	query, args, err := us.Build()
	if err != nil {
		return nil, err
	}
	p, err := initializedPool()
	if err != nil {
		return nil, err
	}
	return p.QueryRow(ctx, query, args...), nil
}

// Exec executes the DELETE statement.
func (ds *DeleteStmt) Exec(ctx context.Context) error {
	query, args, err := ds.Build()
	if err != nil {
		return err
	}
	p, err := initializedPool()
	if err != nil {
		return err
	}
	_, err = p.Exec(ctx, query, args...)
	return err
}

// ExecRet executes the DELETE statement and returns one row.
func (ds *DeleteStmt) ExecRet(ctx context.Context) (pgx.Row, error) {
	query, args, err := ds.Build()
	if err != nil {
		return nil, err
	}
	p, err := initializedPool()
	if err != nil {
		return nil, err
	}
	return p.QueryRow(ctx, query, args...), nil
}

// Exec executes the INSERT statement.
func (is *InsertStmt) Exec(ctx context.Context) error {
	query, args, err := is.Build()
	if err != nil {
		return err
	}
	p, err := initializedPool()
	if err != nil {
		return err
	}
	_, err = p.Exec(ctx, query, args...)
	return err
}

// ExecRet executes the INSERT statement and returns one row.
func (is *InsertStmt) ExecRet(ctx context.Context) (pgx.Row, error) {
	query, args, err := is.Build()
	if err != nil {
		return nil, err
	}
	p, err := initializedPool()
	if err != nil {
		return nil, err
	}
	return p.QueryRow(ctx, query, args...), nil
}

func initializedPool() (*pgxpool.Pool, error) {
	if pool == nil {
		return nil, errPoolNotInitialized
	}
	return pool, nil
}

func buildReturning(b *strings.Builder, returning []string) {
	b.WriteString(" RETURNING ")
	b.WriteString(strings.Join(returning, ","))
}
