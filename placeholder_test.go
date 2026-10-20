package pql

import (
	"strings"
	"testing"
)

func TestNumberPlaceholders(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name   string
		expr   string
		want   string
		args   int
		offset int
	}{
		{"bare placeholder", `$`, `$1`, 1, 0},
		{"casts and punctuation", `coalesce($::int,$)+$`, `coalesce($1::int,$2)+$3`, 3, 0},
		{"array argument", `id = ANY($)`, `id = ANY($1)`, 1, 0},
		{"existing arguments", `a=$ OR b=$`, `a=$10 OR b=$11`, 2, 9},
		{"ordinary string", `body = '$' AND id = $`, `body = '$' AND id = $1`, 1, 0},
		{"doubled single quote", `body = 'it''s $' AND id = $`, `body = 'it''s $' AND id = $1`, 1, 0},
		{"empty string", `body = '' AND id = $`, `body = '' AND id = $1`, 1, 0},
		{"ordinary backslash", `body = '\' AND id = $`, `body = '\' AND id = $1`, 1, 0},
		{"escape string", `body = E'it\'s $' AND id = $`, `body = E'it\'s $' AND id = $1`, 1, 0},
		{"lowercase escape string", `body = e'it\'s $' AND id = $`, `body = e'it\'s $' AND id = $1`, 1, 0},
		{"escaped backslash", `body = E'\\' AND id = $`, `body = E'\\' AND id = $1`, 1, 0},
		{"escape string with doubled quote", `body = E'it''s $' AND id = $`, `body = E'it''s $' AND id = $1`, 1, 0},
		{"escape continuation", "body = E'a'\n" + `'\'$' AND id = $`, "body = E'a'\n" + `'\'$' AND id = $1`, 1, 0},
		{"escape continuation after comment", "body = E'a' -- $ ' \"\r\n" + `'\'$' AND id = $`, "body = E'a' -- $ ' \"\r\n" + `'\'$' AND id = $1`, 1, 0},
		{"multiple continuations", "body = E'a'\n'\\'b'\n'\\'$' AND id = $", "body = E'a'\n'\\'b'\n'\\'$' AND id = $1", 1, 0},
		{"block comment ends continuation", "E'a'\n/* $ */ " + `'\' || $`, "E'a'\n/* $ */ " + `'\' || $1`, 1, 0},
		{"escape behavior ends with token", `coalesce(E'a', '\') = $`, `coalesce(E'a', '\') = $1`, 1, 0},
		{"identifier ending with E", `typeE'\' = $`, `typeE'\' = $1`, 1, 0},
		{"unicode string", `U&'price\0024' = $`, `U&'price\0024' = $1`, 1, 0},
		{"custom unicode escape", `U&'price$0024' UESCAPE '$' = $`, `U&'price$0024' UESCAPE '$' = $1`, 1, 0},
		{"quoted identifier", `"price$" = $`, `"price$" = $1`, 1, 0},
		{"doubled double quote", `"price""$" = $`, `"price""$" = $1`, 1, 0},
		{"backslash in identifier", `"price\" = $`, `"price\" = $1`, 1, 0},
		{"unicode identifier", `U&"price\0024" = $`, `U&"price\0024" = $1`, 1, 0},
		{"unquoted identifier", `price$usd = $`, `price$usd = $1`, 1, 0},
		{"identifier ending with dollar", `price$=$`, `price$=$1`, 1, 0},
		{"identifier containing a number", `price$1 = $`, `price$1 = $1`, 1, 0},
		{"underscore identifier", `_price$_usd = $`, `_price$_usd = $1`, 1, 0},
		{"non-ASCII identifier", `価格$円 = $`, `価格$円 = $1`, 1, 0},
		{"dollar delimiter inside identifier", `price$tag$ = $`, `price$tag$ = $1`, 1, 0},
		{"dollar quote after punctuation", `coalesce(a,$t$$ inside$t$) = $`, `coalesce(a,$t$$ inside$t$) = $1`, 1, 0},
		{"untagged dollar quote", `body = $$ $ ' " /* $$ AND id=$`, `body = $$ $ ' " /* $$ AND id=$1`, 1, 0},
		{"empty dollar quote", `body = $$$$ AND id=$`, `body = $$$$ AND id=$1`, 1, 0},
		{"tagged dollar quote", `body = $tag$$other$ '$1' $tag$ AND id=$`, `body = $tag$$other$ '$1' $tag$ AND id=$1`, 1, 0},
		{"underscore and digit tag", `body = $_tag9$$$_tag9$ AND id=$`, `body = $_tag9$$$_tag9$ AND id=$1`, 1, 0},
		{"non-ASCII tag", `body = $引用$$内容$引用$ AND id=$`, `body = $引用$$内容$引用$ AND id=$1`, 1, 0},
		{"case-sensitive tag", `body = $tag$$TAG$$tag$ AND id=$`, `body = $tag$$TAG$$tag$ AND id=$1`, 1, 0},
		{"different tag with same prefix", `body = $x$$xy$$x$ AND id=$`, `body = $x$$xy$$x$ AND id=$1`, 1, 0},
		{"line comment", "a=$ -- $ ' \" /*\nb=$", "a=$1 -- $ ' \" /*\nb=$2", 2, 0},
		{"carriage return", "a=$ -- $\rb=$", "a=$1 -- $\rb=$2", 2, 0},
		{"CRLF", "a=$ -- $\r\nb=$", "a=$1 -- $\r\nb=$2", 2, 0},
		{"comment at end", `a=$ -- $`, "a=$1 -- $\n", 1, 0},
		{"comment at end without placeholders", `true -- $`, "true -- $\n", 0, 0},
		{"terminated comment at end", "a=$ -- $\n", "a=$1 -- $\n", 1, 0},
		{"block comment", `a=$ /* $ ' " -- */ AND b=$`, `a=$1 /* $ ' " -- */ AND b=$2`, 2, 0},
		{"nested block comment", `a=$ /* $ /* $ */ $ */ AND b=$`, `a=$1 /* $ /* $ */ $ */ AND b=$2`, 2, 0},
		{"adjacent empty comment", `a=/**/$`, `a=/**/$1`, 1, 0},
		{"only protected dollars", `"$" = '$' AND price$usd = $$ $ $$ /* $ */`, `"$" = '$' AND price$usd = $$ $ $$ /* $ */`, 0, 0},
		{"protected named placeholders", `"$foo" = '$foo' AND price$foo = $tag$$foo$tag$ /* $_bar */ AND id=$ -- $名前`, "\"$foo\" = '$foo' AND price$foo = $tag$$foo$tag$ /* $_bar */ AND id=$1 -- $名前\n", 1, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := numberPlaceholders(tt.expr, tt.offset, tt.args)
			if err != nil || got != tt.want {
				t.Fatalf("numberPlaceholders() = (%q, %v), want (%q, nil)", got, err, tt.want)
			}
		})
	}
}

func TestNumberPlaceholdersError(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		expr string
		args int
		want string
	}{
		{"missing argument", `a='$' AND b=$`, 0, "pql: WHERE placeholder count 1 does not match argument count 0"},
		{"extra argument", `a=$ /* $ */`, 2, "pql: WHERE placeholder count 1 does not match argument count 2"},
		{"extra argument with trailing comment", `a=$ -- $`, 2, "pql: WHERE placeholder count 1 does not match argument count 2"},
		{"only literal", `'$'`, 1, "pql: WHERE placeholder count 0 does not match argument count 1"},
		{"unterminated string", `'$`, 0, "pql: unterminated string in WHERE expression"},
		{"unterminated doubled quote", `'it''`, 0, "pql: unterminated string in WHERE expression"},
		{"unterminated escape string", `E'\'`, 0, "pql: unterminated string in WHERE expression"},
		{"escape at end", `E'\`, 0, "pql: unterminated string in WHERE expression"},
		{"unterminated continuation", "E'a'\n'\\", 0, "pql: unterminated string in WHERE expression"},
		{"unterminated identifier", `"$`, 0, "pql: unterminated quoted identifier in WHERE expression"},
		{"unterminated doubled identifier quote", `"a""`, 0, "pql: unterminated quoted identifier in WHERE expression"},
		{"unterminated dollar quote", `$$`, 0, "pql: unterminated dollar-quoted string in WHERE expression"},
		{"unterminated tag", `$tag$text`, 0, "pql: unterminated dollar-quoted string in WHERE expression"},
		{"mismatched tag case", `$tag$text$TAG$`, 0, "pql: unterminated dollar-quoted string in WHERE expression"},
		{"unterminated comment", `/* $`, 0, "pql: unterminated block comment in WHERE expression"},
		{"unterminated nested comment", `/* /* $ */`, 0, "pql: unterminated block comment in WHERE expression"},
		{"numbered parameter", `$1`, 1, "pql: numbered placeholders are not supported in WHERE expressions; use $"},
		{"zero numbered parameter", `$0`, 1, "pql: numbered placeholders are not supported in WHERE expressions; use $"},
		{"mixed numbered parameter", `$ + $12`, 2, "pql: numbered placeholders are not supported in WHERE expressions; use $"},
		{"named parameter", `$foo`, 1, "pql: invalid WHERE placeholder; use $"},
		{"underscore named parameter", `$_bar`, 1, "pql: invalid WHERE placeholder; use $"},
		{"non-ASCII named parameter", `$名前`, 1, "pql: invalid WHERE placeholder; use $"},
		{"incomplete dollar delimiter", `$tag...`, 1, "pql: invalid WHERE placeholder; use $"},
		{"mixed named parameter", `$ + $foo`, 2, "pql: invalid WHERE placeholder; use $"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := numberPlaceholders(tt.expr, 0, tt.args)
			if got != "" || err == nil || err.Error() != tt.want {
				t.Fatalf("numberPlaceholders() = (%q, %v), want (%q, %q)", got, err, "", tt.want)
			}
		})
	}
}

func TestTerminateLineComment(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		expr string
		want string
	}{
		{"id -- comment", "id -- comment\n"},
		{"id --", "id --\n"},
		{"id -- ' $tag$ /*", "id -- ' $tag$ /*\n"},
		{"id -- comment\n", "id -- comment\n"},
		{"id -- comment\r", "id -- comment\r"},
		{"id -- comment\r\n", "id -- comment\r\n"},
		{"id -- comment\n + 1", "id -- comment\n + 1"},
		{"id=$1 -- comment", "id=$1 -- comment\n"},
		{"id=$name -- comment", "id=$name -- comment\n"},
		{"id=$ -- comment", "id=$ -- comment\n"},
		{`'--'`, `'--'`},
		{`E'it\'s --'`, `E'it\'s --'`},
		{"E'a'\n'\\'--'", "E'a'\n'\\'--'"},
		{`"--"`, `"--"`},
		{`$$--$$`, `$$--$$`},
		{`$tag$--$tag$`, `$tag$--$tag$`},
		{`id /* -- /* -- */ */`, `id /* -- /* -- */ */`},
		{`'unterminated --`, `'unterminated --`},
		{`"unterminated --`, `"unterminated --`},
		{`$tag$unterminated --`, `$tag$unterminated --`},
		{`/* unterminated --`, `/* unterminated --`},
	} {
		t.Run(tt.expr, func(t *testing.T) {
			t.Parallel()
			if got := terminateLineComment(tt.expr); got != tt.want {
				t.Fatalf("terminateLineComment(%q) = %q, want %q", tt.expr, got, tt.want)
			}
		})
	}
}

func FuzzNumberPlaceholders(f *testing.F) {
	for _, value := range []string{"", "$", "'\"\\$ -- /* */", "\r\n", "$pql$", "価格$円"} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value string) {
		quoted := strings.ReplaceAll(value, "'", "''")
		tokens := []string{
			"'" + quoted + "'",
			"E'" + strings.ReplaceAll(quoted, `\`, `\\`) + "'",
			`"` + strings.ReplaceAll(value, `"`, `""`) + `"`,
		}
		if !strings.Contains(value, "$pql$") {
			tokens = append(tokens, "$pql$"+value+"$pql$")
		}
		for _, token := range tokens {
			for _, suffix := range []string{"", " -- comment"} {
				expr, want := token+suffix, token+suffix
				if suffix != "" {
					want += "\n"
				}
				if got := terminateLineComment(expr); got != want {
					t.Fatalf("terminateLineComment(%q) = %q, want %q", expr, got, want)
				}
			}
			expr := "$::text = " + token + " AND $::text = $::text"
			want := "$4::text = " + token + " AND $5::text = $6::text"
			got, err := numberPlaceholders(expr, 3, 3)
			if err != nil || got != want {
				t.Fatalf("numberPlaceholders(%q) = (%q, %v), want (%q, nil)", expr, got, err, want)
			}
		}
	})
}
