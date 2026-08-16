package tui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// entryItem adapts a record to the list component.
type entryItem struct {
	rec record
}

// FilterValue satisfies list.Item. The list's own fuzzy filtering is disabled
// in favour of the regex filter, so this is never consulted.
func (i entryItem) FilterValue() string { return i.rec.Cmd }

// dateColumnWidth is the rendered width of "2026-08-16", plus a leading gap.
const dateColumnWidth = 11

// entryDelegate renders one entry per row: a mark column, the command, and the
// date right-aligned.
//
// It holds a pointer to the store because marks live there and change
// independently of the item values the list holds.
type entryDelegate struct {
	store  *store
	styles *styles
}

func (d entryDelegate) Height() int  { return 1 }
func (d entryDelegate) Spacing() int { return 0 }

func (d entryDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d entryDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(entryItem)
	if !ok {
		return
	}

	selected := index == m.Index()

	cursor := "  "
	if selected {
		cursor = d.styles.Cursor.Render("▸ ")
	}

	markGlyph := "  "
	if d.store.isMarked(it.rec.id) {
		markGlyph = d.styles.MarkOn.Render("✗ ")
	}

	date := d.styles.Date.Render(it.rec.When.Format("2006-01-02"))

	// Reserve room for the fixed columns so the command never collides with
	// the date.
	cmdWidth := m.Width() - lipgloss.Width(cursor) - lipgloss.Width(markGlyph) - dateColumnWidth
	if cmdWidth < 8 {
		cmdWidth = 8
	}

	cmd := truncate(oneLine(it.rec.Cmd), cmdWidth)
	if selected {
		cmd = d.styles.SelectedBg.Render(cmd)
	} else {
		cmd = d.styles.Command.Render(cmd)
	}
	cmd = lipgloss.NewStyle().Width(cmdWidth).MaxWidth(cmdWidth).Render(cmd)

	fmt.Fprint(w, cursor+markGlyph+cmd+" "+date)
}

// oneLine collapses a multi-line command for display in a single row, marking
// the joins so the user can tell a wrapped command from a one-liner.
func oneLine(s string) string {
	if !strings.ContainsAny(s, "\n\t") {
		return s
	}
	s = strings.ReplaceAll(s, "\t", " ")
	return strings.ReplaceAll(s, "\n", " ⏎ ")
}

// truncate shortens s to at most width display cells, appending an ellipsis.
func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}

	var b strings.Builder
	used := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if used+rw > width-1 {
			break
		}
		b.WriteRune(r)
		used += rw
	}
	return b.String() + "…"
}

// truncateLeft shortens s from the front. Paths are far more identifiable by
// their tail than their head, so a long one loses its leading directories
// rather than its filename.
func truncateLeft(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}

	runes := []rune(s)
	used := 0
	i := len(runes)
	for i > 0 {
		rw := lipgloss.Width(string(runes[i-1]))
		if used+rw > width-1 {
			break
		}
		used += rw
		i--
	}
	return "…" + string(runes[i:])
}

// shortenPath makes a path fit, collapsing the home directory to ~ first.
func shortenPath(path string, width int) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if rest, ok := strings.CutPrefix(path, home+string(os.PathSeparator)); ok {
			path = filepath.Join("~", rest)
		}
	}
	return truncateLeft(path, width)
}

// sanitize replaces bytes that would corrupt the display. History files are
// byte streams and can hold invalid UTF-8 and control characters; writing
// those straight to the terminal would scramble the layout.
func sanitize(s string) string {
	needsWork := !utf8.ValidString(s)
	if !needsWork {
		for i := 0; i < len(s); i++ {
			if isDisplayControl(s[i]) {
				needsWork = true
				break
			}
		}
	}
	if !needsWork {
		return s
	}

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\n' || r == '\t':
			b.WriteRune(r)
		case r == utf8.RuneError:
			b.WriteRune('�')
		case r < 0x20 || r == 0x7f:
			b.WriteRune('·')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// isDisplayControl reports whether a byte would disturb the terminal if it were
// rendered verbatim. Newlines and tabs are kept: the detail pane lays them out.
func isDisplayControl(b byte) bool {
	return (b < 0x20 && b != '\n' && b != '\t') || b == 0x7f
}

// humanizeSince renders a coarse "how long ago" for the detail pane.
func humanizeSince(t, now time.Time) string {
	d := now.Sub(t)
	switch {
	case d < 0:
		return "in the future"
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hr ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%d months ago", int(d.Hours()/24/30))
	default:
		return fmt.Sprintf("%.1f years ago", d.Hours()/24/365)
	}
}
