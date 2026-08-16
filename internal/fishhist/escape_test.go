package fishhist

import "testing"

func TestEscape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "git status", "git status"},
		{"empty", "", ""},
		{"backslash", `grep \d`, `grep \\d`},
		{"newline", "a\nb", `a\nb`},
		{"both, backslash first", "a\\\nb", `a\\\nb`},
		{"literal backslash n stays distinct", `printf \n`, `printf \\n`},
		{"trailing backslash", `foo \`, `foo \\`},
		{"invalid utf8 passes through", "\xff\xfe", "\xff\xfe"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := string(escape([]byte(tt.in))); got != tt.want {
				t.Errorf("escape(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestUnescape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "git status", "git status"},
		{"empty", "", ""},
		{"escaped backslash", `grep \\d`, `grep \d`},
		{"escaped newline", `a\nb`, "a\nb"},
		{"consecutive escapes", `a\n\nb`, "a\n\nb"},
		{"escape at start", `\na`, "\na"},
		{"escape at end", `a\n`, "a\n"},

		// Fish's decoder recognises only \\ and \n. Anything else ends
		// decoding and discards the remainder; we must match that or we would
		// rewrite the entry differently than fish would.
		{"unknown escape truncates", `a\tb`, "a"},
		{"trailing lone backslash truncates", `a\`, "a"},
		{"unknown escape discards tail", `keep\xdrop\nmore`, "keep"},

		{"invalid utf8 passes through", "\xff\xfe", "\xff\xfe"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := string(unescape([]byte(tt.in))); got != tt.want {
				t.Errorf("unescape(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestEscapeUnescapeRoundTrip(t *testing.T) {
	t.Parallel()

	// Any value fish could have produced survives a round trip, because escape
	// only ever emits the two sequences unescape understands.
	values := []string{
		"", "git status", `grep \d`, "a\nb", "a\\\nb",
		"multi\nline\ncommand", `C:\path\to\thing`, "\xff\xfe", "  leading",
	}
	for _, v := range values {
		if got := string(unescape(escape([]byte(v)))); got != v {
			t.Errorf("round trip of %q gave %q", v, got)
		}
	}
}
