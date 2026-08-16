package tui

import (
	"fmt"

	"github.com/farrellm/yanagiba/internal/fishhist"
)

// record is a history entry plus a stable identity.
//
// Marks, edits and undo all refer to entries by ID rather than by position,
// because a delete shifts every position after it and would otherwise silently
// re-target a mark onto a neighbouring command.
type record struct {
	fishhist.Entry
	id int
}

// store holds the working copy of the history and the staged changes to it.
//
// Records are kept in the order they appeared in the file and are never
// reordered: fish writes history append-only, oldest first, and sorting the
// display must not rewrite that order on disk. Sorting is a view concern; see
// model.visible.
type store struct {
	records []record
	marked  map[int]struct{}
	undos   []undoOp
	nextID  int
	dirty   bool
}

func newStore(entries []fishhist.Entry) *store {
	s := &store{
		records: make([]record, 0, len(entries)),
		marked:  make(map[int]struct{}),
	}
	for _, e := range entries {
		s.records = append(s.records, record{Entry: e.Clone(), id: s.nextID})
		s.nextID++
	}
	return s
}

// entries returns the working copy in file order, ready to be written back.
func (s *store) entries() []fishhist.Entry {
	out := make([]fishhist.Entry, len(s.records))
	for i, r := range s.records {
		out[i] = r.Entry
	}
	return out
}

func (s *store) len() int { return len(s.records) }

func (s *store) isMarked(id int) bool {
	_, ok := s.marked[id]
	return ok
}

func (s *store) markCount() int { return len(s.marked) }

func (s *store) toggleMark(id int) {
	if s.isMarked(id) {
		delete(s.marked, id)
		return
	}
	s.marked[id] = struct{}{}
}

func (s *store) mark(ids []int) {
	for _, id := range ids {
		s.marked[id] = struct{}{}
	}
}

func (s *store) clearMarks() { clear(s.marked) }

// indexOf returns the position of the record with the given ID, or -1.
func (s *store) indexOf(id int) int {
	for i, r := range s.records {
		if r.id == id {
			return i
		}
	}
	return -1
}

// opKind distinguishes the staged operations that can be undone.
type opKind int

const (
	opDelete opKind = iota
	opEdit
)

type undoOp struct {
	kind opKind

	// deleted holds the removed records with the positions they occupied,
	// ascending, so undo can splice them back where they were.
	deleted []positionedRecord

	// editedID and previousCmd restore an edit.
	editedID    int
	previousCmd string
}

type positionedRecord struct {
	index  int
	record record
}

// deleteIDs removes the given records and stages an undo entry. It returns how
// many were removed.
func (s *store) deleteIDs(ids []int) int {
	if len(ids) == 0 {
		return 0
	}

	targets := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		targets[id] = struct{}{}
	}

	kept := make([]record, 0, len(s.records))
	var removed []positionedRecord
	for i, r := range s.records {
		if _, ok := targets[r.id]; ok {
			removed = append(removed, positionedRecord{index: i, record: r})
			continue
		}
		kept = append(kept, r)
	}
	if len(removed) == 0 {
		return 0
	}

	s.records = kept
	for _, pr := range removed {
		delete(s.marked, pr.record.id)
	}
	s.undos = append(s.undos, undoOp{kind: opDelete, deleted: removed})
	s.dirty = true
	return len(removed)
}

// editID replaces the command text of a record, staging an undo entry. It
// reports whether anything changed.
func (s *store) editID(id int, cmd string) bool {
	i := s.indexOf(id)
	if i < 0 || s.records[i].Cmd == cmd {
		return false
	}

	s.undos = append(s.undos, undoOp{
		kind:        opEdit,
		editedID:    id,
		previousCmd: s.records[i].Cmd,
	})
	s.records[i].Cmd = cmd
	s.dirty = true
	return true
}

// undo reverses the most recent staged operation, returning a description of
// what it did.
func (s *store) undo() (string, bool) {
	if len(s.undos) == 0 {
		return "", false
	}

	op := s.undos[len(s.undos)-1]
	s.undos = s.undos[:len(s.undos)-1]

	switch op.kind {
	case opDelete:
		// Reinsert ascending so each recorded index is correct by the time we
		// reach it: restoring the earliest first shifts the later ones into
		// exactly the positions they were recorded at.
		for _, pr := range op.deleted {
			at := min(pr.index, len(s.records))
			s.records = append(s.records, record{})
			copy(s.records[at+1:], s.records[at:])
			s.records[at] = pr.record
		}
		s.dirty = len(s.undos) > 0
		return fmt.Sprintf("restored %s", entryCount(len(op.deleted))), true

	case opEdit:
		if i := s.indexOf(op.editedID); i >= 0 {
			s.records[i].Cmd = op.previousCmd
		}
		s.dirty = len(s.undos) > 0
		return "reverted edit", true

	default:
		return "", false
	}
}

func (s *store) canUndo() bool { return len(s.undos) > 0 }

// entryCount renders a count of history entries with the right plural.
func entryCount(n int) string {
	if n == 1 {
		return "1 entry"
	}
	return fmt.Sprintf("%d entries", n)
}
