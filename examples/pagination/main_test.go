package main

import (
	"bytes"
	"os"
	"testing"
)

func TestPaginationPostgres(t *testing.T) {
	t.Parallel()
	url := os.Getenv("PQL_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set PQL_TEST_DATABASE_URL to run the pagination example")
	}
	var output bytes.Buffer
	if err := run(t.Context(), url, &output); err != nil {
		t.Fatal(err)
	}
	const want = "first: [1 2]\nnext: [3 4]\nprevious: [1 2]\n"
	if got := output.String(); got != want {
		t.Fatalf("pagination output = %q, want %q", got, want)
	}
}
