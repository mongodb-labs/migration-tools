package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShellQuoteArg(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want string
	}{
		{
			name: "plain flag needs no quoting",
			arg:  "--name",
			want: "--name",
		},
		{
			name: "alphanumeric value needs no quoting",
			arg:  "mise-and-go",
			want: "mise-and-go",
		},
		{
			name: "file path needs no quoting",
			arg:  "src/mise.toml",
			want: "src/mise.toml",
		},
		{
			name: "value with spaces is single-quoted",
			arg:  "mise and go executables",
			want: "'mise and go executables'",
		},
		{
			name: "dollar sign is single-quoted to prevent expansion",
			arg:  "foo$bar",
			want: "'foo$bar'",
		},
		{
			name: "backtick is single-quoted to prevent command substitution",
			arg:  "foo`bar`baz",
			want: "'foo`bar`baz'",
		},
		{
			name: "backslash is single-quoted",
			arg:  `foo\bar`,
			want: `'foo\bar'`,
		},
		{
			name: "embedded single quote uses the '\\'' escape",
			arg:  "it's",
			want: `'it'\''s'`,
		},
		{
			name: "multiple single quotes are each escaped",
			arg:  "a'b'c",
			want: `'a'\''b'\''c'`,
		},
		{
			name: "exclamation mark is single-quoted",
			arg:  "foo!bar",
			want: "'foo!bar'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, shellQuoteArg(tt.arg),
				"shellQuoteArg(%q) should produce a safely shell-quoted string", tt.arg)
		})
	}
}
