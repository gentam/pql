package pql

import (
	"reflect"
	"testing"
)

type buildResult struct {
	query string
	args  []any
	err   string
}

func assertBuild(t *testing.T, b Builder, want buildResult) {
	t.Helper()

	gotQuery, gotArgs, gotErr := b.Build()
	if gotQuery != want.query {
		t.Errorf("query mismatch\n got: %q\nwant: %q", gotQuery, want.query)
	}
	if !reflect.DeepEqual(gotArgs, want.args) {
		t.Errorf("args mismatch\n got: %#v\nwant: %#v", gotArgs, want.args)
	}
	if gotErr == nil {
		if want.err != "" {
			t.Errorf("error mismatch\n got: nil\nwant: %q", want.err)
		}
	} else if gotErr.Error() != want.err {
		t.Errorf("error mismatch\n got: %q\nwant: %q", gotErr, want.err)
	}
}

func TestBuildError(t *testing.T) {
	tests := []struct {
		name  string
		build func() Builder
		want  string
	}{
		{
			name:  "insert without table",
			build: func() Builder { return Insert("").Set("c", 1) },
			want:  "pql: INSERT table is required",
		},
		{
			name:  "insert without values",
			build: func() Builder { return Insert("t") },
			want:  "pql: INSERT values are required",
		},
		{
			name:  "update without table",
			build: func() Builder { return Update("").Set("c", 1) },
			want:  "pql: UPDATE table is required",
		},
		{
			name:  "update without values",
			build: func() Builder { return Update("t") },
			want:  "pql: UPDATE values are required",
		},
		{
			name:  "delete without table",
			build: func() Builder { return Delete("") },
			want:  "pql: DELETE table is required",
		},
		{
			name:  "empty condition",
			build: func() Builder { return Select().From("t").Where("") },
			want:  "pql: WHERE condition is required",
		},
		{
			name: "too few placeholders",
			build: func() Builder {
				return Select().From("t").Where("a=?", 1, 2)
			},
			want: "pql: WHERE placeholder count 1 does not match argument count 2",
		},
		{
			name: "too many placeholders",
			build: func() Builder {
				return Select().From("t").Where("a=? AND b=?", 1)
			},
			want: "pql: WHERE placeholder count 2 does not match argument count 1",
		},
		{
			name: "binary operator with extra argument",
			build: func() Builder {
				return Select().From("t").Where("a", 1).Eq(2)
			},
			want: `pql: WHERE operator "=" requires 1 argument, got 2`,
		},
		{
			name: "postfix operator with argument",
			build: func() Builder {
				return Select().From("t").Where("a", 1).IsNull()
			},
			want: `pql: WHERE operator "IS NULL" requires 0 arguments, got 1`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertBuild(t, tt.build(), buildResult{err: tt.want})
		})
	}
}
