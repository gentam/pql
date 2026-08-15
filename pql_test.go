package pql

import (
	"context"
	"testing"
)

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
