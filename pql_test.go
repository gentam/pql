package pql

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ Querier = (*pgxpool.Pool)(nil)
var _ Querier = (*pgxpool.Conn)(nil)
var _ Querier = (*pgx.Conn)(nil)
var _ Querier = (pgx.Tx)(nil)

func TestExecutionWithoutInit(t *testing.T) {
	Init(nil)
	ctx := context.Background()
	tests := []struct {
		name string
		exec func() error
	}{
		{
			name: "select query",
			exec: func() error {
				_, err := Select().Query(ctx)
				return err
			},
		},
		{
			name: "select query row",
			exec: func() error {
				_, err := Select().QueryRow(ctx)
				return err
			},
		},
		{
			name: "update exec",
			exec: func() error {
				return Update("t").Set("c", 1).Exec(ctx)
			},
		},
		{
			name: "update exec returning",
			exec: func() error {
				_, err := Update("t").Set("c", 1).ExecRet(ctx)
				return err
			},
		},
		{
			name: "delete exec",
			exec: func() error {
				return Delete("t").Exec(ctx)
			},
		},
		{
			name: "delete exec returning",
			exec: func() error {
				_, err := Delete("t").ExecRet(ctx)
				return err
			},
		},
		{
			name: "insert exec",
			exec: func() error {
				return Insert("t").Set("c", 1).Exec(ctx)
			},
		},
		{
			name: "insert exec returning",
			exec: func() error {
				_, err := Insert("t").Set("c", 1).ExecRet(ctx)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.exec(); err != errPoolNotInitialized {
				t.Fatalf("error = %v, want %v", err, errPoolNotInitialized)
			}
		})
	}
}

func TestResolveQuerier(t *testing.T) {
	defaultPool := &pgxpool.Pool{}
	explicitPool := &pgxpool.Pool{}
	var nilPool *pgxpool.Pool

	t.Cleanup(func() { Init(nil) })
	tests := []struct {
		name    string
		init    *pgxpool.Pool
		conn    []Querier
		want    Querier
		wantErr string
	}{
		{
			name: "initialized pool",
			init: defaultPool,
			want: defaultPool,
		},
		{
			name: "explicit connection without initialized pool",
			conn: []Querier{explicitPool},
			want: explicitPool,
		},
		{
			name: "explicit connection overrides initialized pool",
			init: defaultPool,
			conn: []Querier{explicitPool},
			want: explicitPool,
		},
		{
			name:    "uninitialized pool",
			wantErr: "pql: pool is not initialized",
		},
		{
			name:    "nil connection",
			conn:    []Querier{nil},
			wantErr: "pql: connection is nil",
		},
		{
			name:    "typed nil connection",
			conn:    []Querier{nilPool},
			wantErr: "pql: connection is nil",
		},
		{
			name:    "multiple connections",
			conn:    []Querier{defaultPool, explicitPool},
			wantErr: "pql: expected at most one connection, got 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Init(tt.init)
			got, err := resolveQuerier(tt.conn)
			if err != nil {
				if err.Error() != tt.wantErr {
					t.Fatalf("error = %q, want %q", err, tt.wantErr)
				}
				return
			}
			if tt.wantErr != "" {
				t.Fatalf("error = nil, want %q", tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("connection = %v, want %v", got, tt.want)
			}
		})
	}
}
