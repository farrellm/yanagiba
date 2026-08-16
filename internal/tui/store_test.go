package tui

import (
	"testing"
	"time"

	"github.com/farrellm/yanagiba/internal/fishhist"
)

func testEntries(cmds ...string) []fishhist.Entry {
	out := make([]fishhist.Entry, len(cmds))
	for i, c := range cmds {
		at := time.Unix(int64(1000+i), 0)
		out[i] = fishhist.Entry{Cmd: c, When: at, AddedWhen: at}
	}
	return out
}

func commands(s *store) []string {
	out := make([]string, len(s.records))
	for i, r := range s.records {
		out[i] = r.Cmd
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestStoreDeleteAndUndoRestoresPositions(t *testing.T) {
	t.Parallel()

	s := newStore(testEntries("a", "b", "c", "d", "e"))

	// Delete a non-contiguous selection: undo must put each entry back where it
	// was, not merely append them.
	ids := []int{s.records[1].id, s.records[3].id}
	if got := s.deleteIDs(ids); got != 2 {
		t.Fatalf("deleteIDs removed %d, want 2", got)
	}
	if got, want := commands(s), []string{"a", "c", "e"}; !equal(got, want) {
		t.Fatalf("after delete = %v, want %v", got, want)
	}

	if _, ok := s.undo(); !ok {
		t.Fatal("undo reported nothing to do")
	}
	if got, want := commands(s), []string{"a", "b", "c", "d", "e"}; !equal(got, want) {
		t.Errorf("after undo = %v, want %v", got, want)
	}
}

func TestStoreDeleteFirstAndLast(t *testing.T) {
	t.Parallel()

	s := newStore(testEntries("a", "b", "c"))
	s.deleteIDs([]int{s.records[0].id, s.records[2].id})
	if got, want := commands(s), []string{"b"}; !equal(got, want) {
		t.Fatalf("after delete = %v, want %v", got, want)
	}

	s.undo()
	if got, want := commands(s), []string{"a", "b", "c"}; !equal(got, want) {
		t.Errorf("after undo = %v, want %v", got, want)
	}
}

func TestStoreDeleteClearsMarks(t *testing.T) {
	t.Parallel()

	s := newStore(testEntries("a", "b"))
	id := s.records[0].id
	s.toggleMark(id)
	s.deleteIDs([]int{id})

	if s.markCount() != 0 {
		t.Errorf("markCount = %d, want 0 -- a deleted entry must not stay marked", s.markCount())
	}
}

func TestStoreEditAndUndo(t *testing.T) {
	t.Parallel()

	s := newStore(testEntries("a", "b"))
	id := s.records[1].id

	if !s.editID(id, "b-edited") {
		t.Fatal("editID reported no change")
	}
	if got, want := commands(s), []string{"a", "b-edited"}; !equal(got, want) {
		t.Fatalf("after edit = %v, want %v", got, want)
	}

	s.undo()
	if got, want := commands(s), []string{"a", "b"}; !equal(got, want) {
		t.Errorf("after undo = %v, want %v", got, want)
	}
}

func TestStoreEditToSameValueIsNotAChange(t *testing.T) {
	t.Parallel()

	s := newStore(testEntries("a"))
	if s.editID(s.records[0].id, "a") {
		t.Error("editing to the identical value should report no change")
	}
	if s.dirty {
		t.Error("a no-op edit should not mark the store dirty")
	}
	if s.canUndo() {
		t.Error("a no-op edit should not push an undo entry")
	}
}

func TestStoreDirtyTracking(t *testing.T) {
	t.Parallel()

	s := newStore(testEntries("a", "b"))
	if s.dirty {
		t.Fatal("a freshly loaded store should be clean")
	}

	s.deleteIDs([]int{s.records[0].id})
	if !s.dirty {
		t.Fatal("a delete should mark the store dirty")
	}

	// Undoing the only change returns the store to its on-disk state.
	s.undo()
	if s.dirty {
		t.Error("undoing the only change should leave the store clean")
	}
}

func TestStoreMarks(t *testing.T) {
	t.Parallel()

	s := newStore(testEntries("a", "b", "c"))
	id := s.records[0].id

	s.toggleMark(id)
	if !s.isMarked(id) || s.markCount() != 1 {
		t.Fatal("toggleMark should mark an unmarked entry")
	}
	s.toggleMark(id)
	if s.isMarked(id) || s.markCount() != 0 {
		t.Fatal("toggleMark should unmark a marked entry")
	}

	s.mark([]int{s.records[0].id, s.records[2].id})
	if s.markCount() != 2 {
		t.Fatalf("markCount = %d, want 2", s.markCount())
	}
	s.clearMarks()
	if s.markCount() != 0 {
		t.Errorf("markCount = %d, want 0 after clearMarks", s.markCount())
	}
}

func TestStoreEntriesPreservesFileOrder(t *testing.T) {
	t.Parallel()

	// Display sorting must never reach the store: fish writes history
	// append-only, oldest first, and reordering it on save would rewrite the
	// whole file.
	s := newStore(testEntries("first", "second", "third"))
	entries := s.entries()

	if len(entries) != 3 {
		t.Fatalf("entries returned %d, want 3", len(entries))
	}
	for i, want := range []string{"first", "second", "third"} {
		if entries[i].Cmd != want {
			t.Errorf("entries[%d].Cmd = %q, want %q", i, entries[i].Cmd, want)
		}
	}
}

func TestStoreEntriesIsIndependentOfSource(t *testing.T) {
	t.Parallel()

	src := []fishhist.Entry{{Cmd: "a", Paths: []string{"p"}}}
	s := newStore(src)
	s.records[0].Paths[0] = "mutated"

	if src[0].Paths[0] != "p" {
		t.Error("newStore must deep-copy entries so edits cannot reach the parsed slice")
	}
}

func TestStoreUndoOnEmptyStack(t *testing.T) {
	t.Parallel()

	s := newStore(testEntries("a"))
	if _, ok := s.undo(); ok {
		t.Error("undo on a fresh store should report nothing to do")
	}
}

func TestStoreDeleteNothing(t *testing.T) {
	t.Parallel()

	s := newStore(testEntries("a"))
	if got := s.deleteIDs(nil); got != 0 {
		t.Errorf("deleteIDs(nil) = %d, want 0", got)
	}
	if got := s.deleteIDs([]int{9999}); got != 0 {
		t.Errorf("deleteIDs of an unknown id = %d, want 0", got)
	}
	if s.canUndo() {
		t.Error("a delete that removed nothing should not push an undo entry")
	}
}
