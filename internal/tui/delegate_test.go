package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
)

func TestTruncate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		in    string
		width int
		want  string
	}{
		{"fits", "git push", 20, "git push"},
		{"exact fit", "git push", 8, "git push"},
		{"truncated", "git push --force", 8, "git pus…"},
		{"width one", "git push", 1, "…"},
		{"width zero", "git push", 0, ""},
		{"negative width", "git push", -3, ""},
		{"empty input", "", 10, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := truncate(tt.in, tt.width); got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.in, tt.width, got, tt.want)
			}
		})
	}
}

func TestTruncateNeverExceedsWidth(t *testing.T) {
	t.Parallel()

	// Wide runes occupy two cells, so counting runes instead of cells would
	// overflow the column and break the layout.
	inputs := []string{
		"日本語のコマンドをたくさん",
		"ascii text that is quite long indeed",
		"mixed 日本 text 語 here",
		"emoji 🎏 in the middle",
	}
	for _, in := range inputs {
		for width := 1; width <= 20; width++ {
			got := truncate(in, width)
			if w := lipgloss.Width(got); w > width {
				t.Errorf("truncate(%q, %d) = %q, which is %d cells wide", in, width, got, w)
			}
		}
	}
}

func TestOneLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want string
	}{
		{"git push", "git push"},
		{"a\nb", "a ⏎ b"},
		{"a\tb", "a b"},
		{"for i in 1 2\necho $i\nend", "for i in 1 2 ⏎ echo $i ⏎ end"},
	}
	for _, tt := range tests {
		if got := oneLine(tt.in); got != tt.want {
			t.Errorf("oneLine(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestOneLineProducesASingleLine(t *testing.T) {
	t.Parallel()

	// A stray newline in a list row would push every following row down.
	if got := oneLine("a\nb\nc"); strings.Contains(got, "\n") {
		t.Errorf("oneLine left a newline in %q", got)
	}
}

func TestSanitize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain text is untouched", "git push", "git push"},
		{"newlines and tabs survive", "a\nb\tc", "a\nb\tc"},
		{"unicode survives", "echo café 日本", "echo café 日本"},
		{"control characters are replaced", "a\x01b", "a·b"},
		{"delete is replaced", "a\x7fb", "a·b"},
		{"escape is replaced", "a\x1b[31mb", "a·[31mb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := sanitize(tt.in); got != tt.want {
				t.Errorf("sanitize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSanitizeReplacesInvalidUTF8(t *testing.T) {
	t.Parallel()

	// History files are byte streams; writing raw invalid UTF-8 to the terminal
	// scrambles the display.
	got := sanitize("echo \xff\xfe")
	if !utf8.ValidString(got) {
		t.Errorf("sanitize left invalid UTF-8 in %q", got)
	}
	if !strings.HasPrefix(got, "echo ") {
		t.Errorf("sanitize mangled the valid prefix: %q", got)
	}
}

func TestSanitizeStripsANSIEscapes(t *testing.T) {
	t.Parallel()

	// A command containing an escape sequence must not be able to recolour or
	// reposition the UI when it is rendered.
	if got := sanitize("\x1b[2J\x1b[H"); strings.Contains(got, "\x1b") {
		t.Errorf("sanitize left an escape character in %q", got)
	}
}

func TestHumanizeSince(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		at   time.Time
		want string
	}{
		{"seconds", now.Add(-30 * time.Second), "just now"},
		{"minutes", now.Add(-5 * time.Minute), "5 min ago"},
		{"hours", now.Add(-3 * time.Hour), "3 hr ago"},
		{"days", now.Add(-3 * 24 * time.Hour), "3 days ago"},
		{"months", now.Add(-90 * 24 * time.Hour), "3 months ago"},
		{"years", now.Add(-400 * 24 * time.Hour), "1.1 years ago"},
		{"future", now.Add(time.Hour), "in the future"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := humanizeSince(tt.at, now); got != tt.want {
				t.Errorf("humanizeSince = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTruncateLeftKeepsTheTail(t *testing.T) {
	t.Parallel()

	// A path is identified by its filename, so the front is what gives way.
	if got, want := truncateLeft("/a/very/long/path/history", 12), "…ath/history"; got != want {
		t.Errorf("truncateLeft = %q, want %q", got, want)
	}
	if got, want := truncateLeft("short", 20), "short"; got != want {
		t.Errorf("truncateLeft = %q, want %q", got, want)
	}
	if got, want := truncateLeft("short", 0), ""; got != want {
		t.Errorf("truncateLeft = %q, want %q", got, want)
	}
	if got, want := truncateLeft("short", 1), "…"; got != want {
		t.Errorf("truncateLeft = %q, want %q", got, want)
	}
}

func TestTruncateLeftNeverExceedsWidth(t *testing.T) {
	t.Parallel()

	for _, in := range []string{"/home/user/日本語/fish_history", "/a/b/c/d/e/f/g"} {
		for width := 1; width <= 20; width++ {
			if w := lipgloss.Width(truncateLeft(in, width)); w > width {
				t.Errorf("truncateLeft(%q, %d) is %d cells wide", in, width, w)
			}
		}
	}
}

func TestShortenPathCollapsesHome(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home directory")
	}

	got := shortenPath(filepath.Join(home, ".local", "share", "fish", "fish_history"), 60)
	if !strings.HasPrefix(got, "~/") {
		t.Errorf("shortenPath = %q, want it to start with ~/", got)
	}
	if !strings.HasSuffix(got, "fish_history") {
		t.Errorf("shortenPath = %q, want it to keep the filename", got)
	}
}
