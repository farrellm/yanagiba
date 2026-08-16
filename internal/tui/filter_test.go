package tui

import (
	"testing"

	"github.com/farrellm/yanagiba/internal/fishhist"
)

func TestFilterMatches(t *testing.T) {
	t.Parallel()

	entry := fishhist.Entry{Cmd: "git push --force", Paths: []string{"web/vite.config.ts"}}

	tests := []struct {
		name       string
		pattern    string
		smartCase  bool
		matchPaths bool
		want       bool
	}{
		{"empty matches everything", "", true, false, true},
		{"anchored prefix", "^git", true, false, true},
		{"anchored prefix that misses", "^push", true, false, false},
		{"alternation", "^(git|hg) push", true, false, true},
		{"substring", "force", true, false, true},

		{"smart case folds a lowercase pattern", "GIT", true, false, false},
		{"smart case leaves lowercase insensitive", "git", true, false, true},
		{"uppercase in pattern forces case sensitivity", "Git", true, false, false},
		{"case sensitive when smart case is off", "GIT", false, false, false},

		{"paths not searched by default", "vite", true, false, false},
		{"paths searched when enabled", "vite", true, true, true},
		{"command still matches when paths enabled", "^git", true, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := compileFilter(tt.pattern, tt.smartCase, tt.matchPaths)
			if f.err != nil {
				t.Fatalf("compileFilter(%q) errored: %v", tt.pattern, f.err)
			}
			if got := f.matches(entry); got != tt.want {
				t.Errorf("matches = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterSmartCaseIsCaseInsensitiveInPractice(t *testing.T) {
	t.Parallel()

	f := compileFilter("push", true, false)
	if !f.matches(fishhist.Entry{Cmd: "git PUSH"}) {
		t.Error("an all-lowercase pattern should match uppercase text under smart case")
	}
}

func TestFilterInvalidPatternIsReportedNotFatal(t *testing.T) {
	t.Parallel()

	f := compileFilter("[", true, false)
	if f.err == nil {
		t.Fatal("compileFilter(\"[\") should report an error")
	}
	// A half-typed pattern must not empty the list: an inactive filter matches
	// everything, so the previous results stay on screen.
	if f.active() {
		t.Error("a filter that failed to compile should not be active")
	}
	if !f.matches(fishhist.Entry{Cmd: "anything"}) {
		t.Error("a filter that failed to compile should match everything")
	}
}

func TestHasUpper(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want bool
	}{
		{"git push", false},
		{"Git", true},
		{"gitP", true},
		{"", false},
		{"^(a|b)$", false},

		// Escapes and inline flag groups must not be mistaken for the user
		// typing a capital letter, or smart case would switch off unexpectedly.
		{`\S+`, false},
		{`\d\w`, false},
		{"(?i)git", false},
		{"(?is:git)", false},
		{`\Sfoo`, false},
		{`\SFoo`, true},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			if got := hasUpper(tt.in); got != tt.want {
				t.Errorf("hasUpper(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
