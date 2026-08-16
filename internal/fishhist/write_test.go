package fishhist

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteFileReplacesAndBacksUp(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "fish_history")
	original := "- cmd: original\n  when: 100\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	at := time.Unix(200, 0)
	backup, err := WriteFile(path, []Entry{{Cmd: "replacement", When: at, AddedWhen: at}})
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "- cmd: replacement\n  when: 200\n"; string(got) != want {
		t.Errorf("history = %q, want %q", got, want)
	}

	if backup == "" {
		t.Fatal("WriteFile returned no backup path for an existing file")
	}
	saved, err := os.ReadFile(backup) //nolint:gosec // path produced by the code under test
	if err != nil {
		t.Fatalf("reading backup: %v", err)
	}
	if string(saved) != original {
		t.Errorf("backup = %q, want the original %q", saved, original)
	}
}

func TestWriteFilePreservesPermissions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "fish_history")
	if err := os.WriteFile(path, []byte("- cmd: x\n  when: 1\n"), 0o640); err != nil {
		t.Fatal(err)
	}

	if _, err := WriteFile(path, []Entry{{Cmd: "y", When: time.Unix(1, 0), AddedWhen: time.Unix(1, 0)}}); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// A history file can contain secrets, so widening its permissions during a
	// rewrite would be a real leak.
	if got, want := info.Mode().Perm(), os.FileMode(0o640); got != want {
		t.Errorf("mode = %v, want %v", got, want)
	}
}

func TestWriteFileWithoutExistingFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "fish_history")
	backup, err := WriteFile(path, []Entry{{Cmd: "new", When: time.Unix(1, 0), AddedWhen: time.Unix(1, 0)}})
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if backup != "" {
		t.Errorf("backup = %q, want none for a file that did not exist", backup)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("history file not created: %v", err)
	}
}

func TestWriteFileLeavesNoTempFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "fish_history")
	at := time.Unix(1, 0)
	if _, err := WriteFile(path, []Entry{{Cmd: "a", When: at, AddedWhen: at}}); err != nil {
		t.Fatal(err)
	}

	names, err := filepath.Glob(filepath.Join(dir, "*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 0 {
		t.Errorf("left temporary files behind: %v", names)
	}
}

func TestPruneBackupsKeepsNewest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "fish_history")
	at := time.Unix(1, 0)

	// Each write backs up the previous contents, so more writes than
	// backupsKept must still leave only backupsKept backups.
	for i := range backupsKept + 3 {
		if _, err := WriteFile(path, []Entry{{Cmd: "x", When: at, AddedWhen: at}}); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
		// The backup name has one-second resolution, so distinct names require
		// distinct seconds. Fake it by renaming rather than sleeping.
		if err := renameNewestBackup(path, i); err != nil {
			t.Fatalf("staging backup %d: %v", i, err)
		}
	}

	backups, err := Backups(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) > backupsKept {
		t.Errorf("kept %d backups, want at most %d: %v", len(backups), backupsKept, backups)
	}
}

// renameNewestBackup gives the most recent backup a distinct, older timestamp
// so the test does not have to sleep for real time to pass.
func renameNewestBackup(path string, i int) error {
	backups, err := Backups(path)
	if err != nil {
		return err
	}
	if len(backups) == 0 {
		return nil
	}
	newest := backups[len(backups)-1]
	want := backupName(path, time.Date(2020, 1, 1, 0, 0, i, 0, time.UTC))
	if newest == want {
		return nil
	}
	return os.Rename(newest, want)
}

func TestBackupNameIsSortableByAge(t *testing.T) {
	t.Parallel()

	// pruneBackups relies on lexical order matching chronological order.
	older := backupName("/tmp/fish_history", time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	newer := backupName("/tmp/fish_history", time.Date(2026, 1, 2, 3, 4, 6, 0, time.UTC))
	if older >= newer {
		t.Errorf("%q should sort before %q", older, newer)
	}
}

func TestWriteFileRoundTripsThroughParse(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "fish_history")
	want := []Entry{
		{Cmd: "multi\nline", When: time.Unix(100, 0), AddedWhen: time.Unix(50, 0)},
		{Cmd: `back\slash`, When: time.Unix(200, 0), AddedWhen: time.Unix(200, 0), Paths: []string{"a b", "c"}},
	}
	if _, err := WriteFile(path, want); err != nil {
		t.Fatal(err)
	}

	got, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("parsed %d entries, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Cmd != want[i].Cmd {
			t.Errorf("entry %d Cmd = %q, want %q", i, got[i].Cmd, want[i].Cmd)
		}
		if !got[i].When.Equal(want[i].When) || !got[i].AddedWhen.Equal(want[i].AddedWhen) {
			t.Errorf("entry %d timestamps = %v/%v, want %v/%v",
				i, got[i].When, got[i].AddedWhen, want[i].When, want[i].AddedWhen)
		}
	}
}
