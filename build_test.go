package pql

import (
	"reflect"
	"testing"
)

type buildResult struct {
	query string
	args  []any
}

func assertBuild(t *testing.T, b Builder, want buildResult) {
	t.Helper()

	gotQuery, gotArgs := b.Build()
	if gotQuery != want.query {
		t.Errorf("query mismatch\n got: %q\nwant: %q", gotQuery, want.query)
	}
	if !reflect.DeepEqual(gotArgs, want.args) {
		t.Errorf("args mismatch\n got: %#v\nwant: %#v", gotArgs, want.args)
	}
}

func assertBuildOneOf(t *testing.T, b Builder, wants ...buildResult) {
	t.Helper()

	gotQuery, gotArgs := b.Build()
	for _, want := range wants {
		if gotQuery == want.query && reflect.DeepEqual(gotArgs, want.args) {
			return
		}
	}

	t.Errorf("unexpected build result\n query: %q\n args: %#v\nwant one of: %#v", gotQuery, gotArgs, wants)
}
