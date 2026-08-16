package tui

import (
	"regexp"
	"strings"

	"github.com/farrellm/yanagiba/internal/fishhist"
)

// filter is a compiled regular expression filter over history entries.
//
// The zero value matches everything, which is what an empty filter bar means.
type filter struct {
	pattern string
	re      *regexp.Regexp
	err     error

	// smartCase makes an all-lowercase pattern case-insensitive, the same
	// convention vim and ripgrep use. Typing an uppercase letter opts back in
	// to a case-sensitive search.
	smartCase bool

	// matchPaths also tests the entry's recorded file paths.
	matchPaths bool
}

// compileFilter builds a filter from a pattern. An invalid pattern yields a
// filter carrying err; callers keep showing the previous results and surface
// the error rather than emptying the list mid-keystroke.
func compileFilter(pattern string, smartCase, matchPaths bool) filter {
	f := filter{pattern: pattern, smartCase: smartCase, matchPaths: matchPaths}
	if pattern == "" {
		return f
	}

	expr := pattern
	if smartCase && !hasUpper(pattern) {
		expr = "(?i)" + expr
	}

	re, err := regexp.Compile(expr)
	if err != nil {
		f.err = err
		return f
	}
	f.re = re
	return f
}

// active reports whether the filter narrows the list at all.
func (f filter) active() bool { return f.re != nil }

// errorText renders a compile failure compactly enough to fit on the filter
// bar. Go's message is "error parsing regexp: <reason>: `<pattern>`", which
// repeats the pattern the user can already see and pushes the reason -- the
// only useful part -- off the end of a narrow terminal.
func (f filter) errorText() string {
	if f.err == nil {
		return ""
	}

	msg := f.err.Error()
	msg = strings.TrimPrefix(msg, "error parsing regexp: ")
	if i := strings.LastIndex(msg, ": `"); i >= 0 {
		msg = msg[:i]
	}
	return msg
}

// matches reports whether an entry passes the filter. An entry always passes
// when the pattern is empty or failed to compile.
func (f filter) matches(e fishhist.Entry) bool {
	if f.re == nil {
		return true
	}
	if f.re.MatchString(e.Cmd) {
		return true
	}
	if f.matchPaths {
		for _, p := range e.Paths {
			if f.re.MatchString(p) {
				return true
			}
		}
	}
	return false
}

// hasUpper reports whether s contains an uppercase letter, ignoring the ones
// inside regexp escapes and inline flag groups. Without that exclusion,
// patterns like `\S` or `(?i)x` would silently disable smart case.
func hasUpper(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' {
			i++ // skip the escaped character, upper or not
			continue
		}
		if strings.HasPrefix(s[i:], "(?") {
			// Skip an inline flag group such as (?i) or (?is:...).
			if j := strings.IndexAny(s[i:], ":)"); j >= 0 {
				i += j
				continue
			}
		}
		if c >= 'A' && c <= 'Z' {
			return true
		}
	}
	return false
}
