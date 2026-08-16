package tui

import (
	"testing"
	"time"

	"github.com/farrellm/yanagiba/internal/fishhist"
)

func newTestModel(t *testing.T, cmds ...string) Model {
	t.Helper()
	m := New("/tmp/does-not-exist", testEntries(cmds...))
	// The program never runs, so supply the size the window-size message would.
	m.width, m.height = 80, 24
	m.layout()
	return m
}

func visibleCommands(m Model) []string {
	out := make([]string, 0, len(m.list.Items()))
	for _, it := range m.list.Items() {
		if ei, ok := it.(entryItem); ok {
			out = append(out, ei.rec.Cmd)
		}
	}
	return out
}

func TestModelDefaultsToNewestFirst(t *testing.T) {
	t.Parallel()

	// testEntries assigns increasing timestamps, so "c" is the most recent.
	m := newTestModel(t, "a", "b", "c")
	if got, want := visibleCommands(m), []string{"c", "b", "a"}; !equal(got, want) {
		t.Errorf("visible = %v, want %v", got, want)
	}
}

func TestModelSortToggleDoesNotReorderTheStore(t *testing.T) {
	t.Parallel()

	m := newTestModel(t, "a", "b", "c")
	m.order = sortOldest
	m.refreshItems()

	if got, want := visibleCommands(m), []string{"a", "b", "c"}; !equal(got, want) {
		t.Errorf("visible = %v, want %v", got, want)
	}
	// The order written to disk must be unaffected by how the list is sorted.
	if got, want := commands(m.store), []string{"a", "b", "c"}; !equal(got, want) {
		t.Errorf("store order = %v, want %v", got, want)
	}
}

func TestModelFilterNarrowsTheList(t *testing.T) {
	t.Parallel()

	m := newTestModel(t, "git push", "make build", "git status")
	m.filter = compileFilter("^git", true, false)
	m.refreshItems()

	if got, want := visibleCommands(m), []string{"git status", "git push"}; !equal(got, want) {
		t.Errorf("visible = %v, want %v", got, want)
	}
}

func TestModelRefreshKeepsCursorOnTheSameRecord(t *testing.T) {
	t.Parallel()

	m := newTestModel(t, "alpha", "beta", "gamma")
	m.list.Select(1) // "beta", since newest-first gives gamma, beta, alpha
	before, ok := m.selected()
	if !ok {
		t.Fatal("no selection")
	}

	// Narrowing the filter to something that still includes the selection must
	// not move the cursor onto a different command.
	m.filter = compileFilter("a", true, false)
	m.refreshItems()

	after, ok := m.selected()
	if !ok {
		t.Fatal("no selection after refresh")
	}
	if after.id != before.id {
		t.Errorf("cursor moved from %q to %q", before.Cmd, after.Cmd)
	}
}

func TestModelRefreshClampsCursorWhenListShrinks(t *testing.T) {
	t.Parallel()

	m := newTestModel(t, "a", "b", "c")
	m.list.Select(2)

	m.filter = compileFilter("^a$", true, false)
	m.refreshItems()

	if got := m.list.Index(); got != 0 {
		t.Errorf("index = %d, want 0 once only one entry matches", got)
	}
	if _, ok := m.selected(); !ok {
		t.Error("selection should remain valid after the list shrinks")
	}
}

func TestModelTargetIDsPrefersMarks(t *testing.T) {
	t.Parallel()

	m := newTestModel(t, "a", "b", "c")
	if got := len(m.targetIDs()); got != 1 {
		t.Errorf("with nothing marked, targetIDs should be the cursor alone, got %d", got)
	}

	m.store.mark([]int{m.store.records[0].id, m.store.records[2].id})
	if got := len(m.targetIDs()); got != 2 {
		t.Errorf("targetIDs = %d, want the 2 marked entries", got)
	}
}

func TestModelTargetIDsIncludesMarksHiddenByTheFilter(t *testing.T) {
	t.Parallel()

	// Marking an entry then narrowing the filter past it must not silently drop
	// it from a delete: the user marked it deliberately.
	m := newTestModel(t, "git push", "make build")
	m.store.mark([]int{m.store.records[1].id}) // "make build"

	m.filter = compileFilter("^git", true, false)
	m.refreshItems()

	if got, want := visibleCommands(m), []string{"git push"}; !equal(got, want) {
		t.Fatalf("visible = %v, want %v", got, want)
	}
	if got := m.targetIDs(); len(got) != 1 || got[0] != m.store.records[1].id {
		t.Errorf("targetIDs = %v, want the hidden marked entry", got)
	}
}

func TestModelEmptyResultHasNoSelection(t *testing.T) {
	t.Parallel()

	m := newTestModel(t, "a", "b")
	m.filter = compileFilter("^zzz", true, false)
	m.refreshItems()

	if _, ok := m.selected(); ok {
		t.Error("a filter matching nothing should leave no selection")
	}
	if got := m.targetIDs(); len(got) != 0 {
		t.Errorf("targetIDs = %v, want none", got)
	}
}

func TestModelRenderDoesNotPanicOnAwkwardEntries(t *testing.T) {
	t.Parallel()

	// Multi-line commands, invalid UTF-8 and wide runes all reach the renderer
	// straight from the file.
	entries := []fishhist.Entry{
		{Cmd: "for i in 1 2 3\n echo $i\nend", When: time.Unix(1000, 0), AddedWhen: time.Unix(900, 0)},
		{Cmd: "echo \xff\xfe", When: time.Unix(1001, 0), AddedWhen: time.Unix(1001, 0)},
		{Cmd: "echo 日本語のコマンド", When: time.Unix(1002, 0), AddedWhen: time.Unix(1002, 0), Paths: []string{"日本/語.txt"}},
		{Cmd: "", When: time.Unix(1003, 0), AddedWhen: time.Unix(1003, 0)},
	}

	m := New("/tmp/does-not-exist", entries)
	m.width, m.height = 40, 12
	m.layout()

	for i := range entries {
		m.list.Select(i)
		m.updateDetail()
		if got := m.render(); got == "" {
			t.Errorf("entry %d rendered nothing", i)
		}
	}
}

func TestModelRenderHandlesZeroSize(t *testing.T) {
	t.Parallel()

	// Bubble Tea renders once before the first window-size message arrives.
	m := New("/tmp/does-not-exist", testEntries("a"))
	if got := m.render(); got != "" {
		t.Errorf("render at zero size = %q, want empty", got)
	}
}
