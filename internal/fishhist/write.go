package fishhist

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

// backupsKept is how many timestamped backups to retain alongside the history
// file. Older ones are pruned on each write.
const backupsKept = 5

// backupSuffix identifies backups this tool made, so pruning never touches
// anything else living in fish's data directory.
const backupSuffix = ".bak"

const backupInfix = ".yanagiba-"

// Marshal serializes entries in fish's history format.
//
// This mirrors HistoryItem::write_to: the added_when key is emitted only when
// it differs from when, and paths only when non-empty. Reproducing those
// conditions exactly is what lets an unmodified file round-trip byte for byte.
func Marshal(entries []Entry) []byte {
	var buf bytes.Buffer
	// Rough preallocation: most entries are one short command plus a timestamp.
	buf.Grow(len(entries) * 64)
	for _, e := range entries {
		appendEntry(&buf, e)
	}
	return buf.Bytes()
}

func appendEntry(buf *bytes.Buffer, e Entry) {
	buf.WriteString("- cmd: ")
	buf.Write(escape([]byte(e.Cmd)))
	buf.WriteByte('\n')

	buf.WriteString("  " + keyLastAdded + ": ")
	buf.WriteString(strconv.FormatInt(e.When.Unix(), 10))
	buf.WriteByte('\n')

	if !e.AddedWhen.Equal(e.When) {
		buf.WriteString("  " + keyFirstAdded + ": ")
		buf.WriteString(strconv.FormatInt(e.AddedWhen.Unix(), 10))
		buf.WriteByte('\n')
	}

	if len(e.Paths) > 0 {
		buf.WriteString("  " + keyPaths + ":\n")
		for _, p := range e.Paths {
			buf.WriteString("    - ")
			buf.Write(escape([]byte(p)))
			buf.WriteByte('\n')
		}
	}
}

// WriteFile atomically replaces the history file at path with entries,
// after backing up the existing file. It returns the path of the backup it
// created, or "" if there was no existing file to back up.
//
// The write mirrors fish's own strategy in rewrite_via_temporary_file: take an
// exclusive lock, write a sibling temporary file, fsync it, then rename it into
// place. Writing a sibling (rather than a file in TMPDIR) keeps the rename on
// one filesystem, which is what makes it atomic.
func WriteFile(path string, entries []Entry) (backup string, err error) {
	dir := filepath.Dir(path)

	// Stat before locking: lockFile opens with O_CREATE, so afterwards the
	// file always exists and we could no longer tell a fresh history from a
	// real one to back up.
	mode := os.FileMode(0o600)
	existed := false
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
		existed = true
	} else if !os.IsNotExist(statErr) {
		return "", statErr
	}

	// Lock against concurrently running fish shells, which use flock on this
	// same file. This does not stop a fish session from later overwriting us
	// from its in-memory copy -- nothing can -- but it does prevent a torn
	// read or write right now.
	unlock, err := lockFile(path)
	if err != nil {
		return "", err
	}
	defer unlock()

	if existed {
		backup, err = backupFile(path, mode)
		if err != nil {
			return "", fmt.Errorf("backing up history: %w", err)
		}
	}

	if err := writeAtomic(path, dir, mode, Marshal(entries)); err != nil {
		return backup, err
	}

	if err := pruneBackups(path); err != nil {
		// A failed prune leaves extra backups behind, which is harmless; the
		// history itself is already safely written.
		return backup, nil //nolint:nilerr // deliberate: the write succeeded
	}
	return backup, nil
}

func writeAtomic(path, dir string, mode os.FileMode, data []byte) error {
	tmp, err := os.CreateTemp(dir, ".yanagiba-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temporary file: %w", err)
	}
	tmpName := tmp.Name()

	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}

	if err := tmp.Chmod(mode); err != nil {
		cleanup()
		return fmt.Errorf("setting permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("writing temporary file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("syncing temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("closing temporary file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("replacing history file: %w", err)
	}
	return nil
}

// backupFile copies path to a timestamped sibling and returns its name.
func backupFile(path string, mode os.FileMode) (string, error) {
	src, err := os.Open(path) //nolint:gosec // the path is user-supplied by design
	if err != nil {
		return "", err
	}
	defer func() { _ = src.Close() }()

	name := backupName(path, time.Now())
	dst, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode) //nolint:gosec // sibling of a user-supplied path
	if err != nil {
		return "", err
	}

	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return "", err
	}
	if err := dst.Sync(); err != nil {
		_ = dst.Close()
		return "", err
	}
	if err := dst.Close(); err != nil {
		return "", err
	}
	return name, nil
}

func backupName(path string, at time.Time) string {
	// Colons in RFC3339 are legal in filenames but awkward to type, so use a
	// compact sortable stamp instead.
	return path + backupInfix + at.Format("20060102-150405") + backupSuffix
}

// pruneBackups deletes all but the newest backupsKept backups of path.
func pruneBackups(path string) error {
	matches, err := filepath.Glob(path + backupInfix + "*" + backupSuffix)
	if err != nil {
		return err
	}
	if len(matches) <= backupsKept {
		return nil
	}
	// The timestamp format is lexicographically sortable, so sorting by name
	// orders them by age.
	sort.Strings(matches)
	for _, old := range matches[:len(matches)-backupsKept] {
		if err := os.Remove(old); err != nil {
			return err
		}
	}
	return nil
}

// Backups lists existing backups of path, newest last.
func Backups(path string) ([]string, error) {
	matches, err := filepath.Glob(path + backupInfix + "*" + backupSuffix)
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	return matches, nil
}
