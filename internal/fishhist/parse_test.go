package fishhist

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestParseSampleFile(t *testing.T) {
	t.Parallel()

	entries, err := ParseFile(filepath.Join("testdata", "sample_history"))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	if got, want := len(entries), 14; got != want {
		t.Fatalf("parsed %d entries, want %d", got, want)
	}

	if got, want := entries[0].Cmd, "git status"; got != want {
		t.Errorf("entries[0].Cmd = %q, want %q", got, want)
	}
	// An entry without added_when reports it equal to when, as fish does.
	if !entries[0].AddedWhen.Equal(entries[0].When) {
		t.Errorf("entries[0] added_when = %v, want it to equal when %v",
			entries[0].AddedWhen, entries[0].When)
	}

	if got, want := entries[1].AddedWhen, time.Unix(1556856000, 0); !got.Equal(want) {
		t.Errorf("entries[1].AddedWhen = %v, want %v", got, want)
	}

	if got, want := entries[3].Paths, []string{"a.sh", "b.sh"}; !reflect.DeepEqual(got, want) {
		t.Errorf("entries[3].Paths = %q, want %q", got, want)
	}

	// Escapes are decoded: \\ becomes one backslash, \n becomes a newline.
	if got, want := entries[4].Cmd, `grep \d access.log`; got != want {
		t.Errorf("entries[4].Cmd = %q, want %q", got, want)
	}
	if got, want := entries[5].Cmd, "for i in 1 2 3\necho $i\nend"; got != want {
		t.Errorf("entries[5].Cmd = %q, want %q", got, want)
	}

	// A colon in the command must not be mistaken for a key separator: only
	// the first colon on the line delimits the key.
	if got, want := entries[6].Cmd, `echo "quotes: colons - and dashes"`; got != want {
		t.Errorf("entries[6].Cmd = %q, want %q", got, want)
	}

	if got, want := entries[7].Cmd, "echo café 日本語"; got != want {
		t.Errorf("entries[7].Cmd = %q, want %q", got, want)
	}

	// A line continuation: an escaped backslash followed by an escaped newline.
	if got, want := entries[12].Cmd, "curl -sL https://example.com/x.json \\\n  -H 'Accept: application/json'"; got != want {
		t.Errorf("entries[12].Cmd = %q, want %q", got, want)
	}
}

// TestRoundTripSampleFile is the load-bearing test: an untouched history file
// must survive parse -> marshal unchanged, byte for byte. If this fails, every
// save silently rewrites the user's history.
func TestRoundTripSampleFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join("testdata", "sample_history")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	got := Marshal(Parse(original))
	if !bytes.Equal(got, original) {
		t.Errorf("round trip changed the file.\n--- got ---\n%s\n--- want ---\n%s", got, original)
	}
}

func TestParseSkipsNonEntryLines(t *testing.T) {
	t.Parallel()

	// Fish tolerates real-YAML markers and skips records corrupted by an
	// interrupted write, rather than refusing to open the file.
	input := "%YAML 1.1\n" +
		"---\n" +
		"- cmd: first\n" +
		"  when: 100\n" +
		"...\n" +
		"\x00garbage\n" +
		"not an entry at all\n" +
		"- cmd: second\n" +
		"  when: 200\n"

	entries := Parse([]byte(input))
	if got, want := len(entries), 2; got != want {
		t.Fatalf("parsed %d entries, want %d: %+v", got, want, entries)
	}
	if entries[0].Cmd != "first" || entries[1].Cmd != "second" {
		t.Errorf("got commands %q and %q", entries[0].Cmd, entries[1].Cmd)
	}
}

func TestParseIgnoresUnknownKeys(t *testing.T) {
	t.Parallel()

	// Fish consumes and ignores keys it does not know. We do too -- which also
	// means we drop them on save, so this is worth pinning deliberately.
	entries := Parse([]byte("- cmd: hello\n  when: 100\n  frobnicate: 12\n  paths:\n    - x\n"))
	if got, want := len(entries), 1; got != want {
		t.Fatalf("parsed %d entries, want %d", got, want)
	}
	if got, want := entries[0].Paths, []string{"x"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Paths = %q, want %q -- keys after an unknown one must still parse", got, want)
	}
}

func TestParseTrimsLeadingWhitespaceFromCommands(t *testing.T) {
	t.Parallel()

	// Fish trims all leading whitespace after the colon, so a command that was
	// typed with leading spaces comes back without them. This is lossy, but
	// matching fish matters more than preserving the input.
	entries := Parse([]byte("- cmd:     indented command\n  when: 100\n"))
	if got, want := entries[0].Cmd, "indented command"; got != want {
		t.Errorf("Cmd = %q, want %q", got, want)
	}
}

func TestParseTruncatesOnUnknownEscape(t *testing.T) {
	t.Parallel()

	// See unescape: fish stops decoding at an unrecognised escape.
	entries := Parse([]byte("- cmd: keep\\tdrop\n  when: 100\n"))
	if got, want := entries[0].Cmd, "keep"; got != want {
		t.Errorf("Cmd = %q, want %q", got, want)
	}
}

func TestParseEmptyAndUnterminated(t *testing.T) {
	t.Parallel()

	if got := Parse(nil); len(got) != 0 {
		t.Errorf("Parse(nil) = %+v, want empty", got)
	}
	if got := Parse([]byte("")); len(got) != 0 {
		t.Errorf("Parse(\"\") = %+v, want empty", got)
	}
	// A final line with no trailing newline is incomplete and is dropped,
	// matching fish's complete_lines.
	got := Parse([]byte("- cmd: done\n  when: 100\n- cmd: torn"))
	if len(got) != 1 {
		t.Fatalf("parsed %d entries, want 1: %+v", len(got), got)
	}
}

func TestMarshalOmitsRedundantFields(t *testing.T) {
	t.Parallel()

	// added_when is written only when it differs from when, and paths only
	// when non-empty -- exactly the conditions in fish's writer.
	at := time.Unix(100, 0)
	got := string(Marshal([]Entry{{Cmd: "x", When: at, AddedWhen: at}}))
	if want := "- cmd: x\n  when: 100\n"; got != want {
		t.Errorf("Marshal = %q, want %q", got, want)
	}

	got = string(Marshal([]Entry{{
		Cmd: "y", When: at, AddedWhen: time.Unix(50, 0), Paths: []string{"a", "b"},
	}}))
	want := "- cmd: y\n  when: 100\n  added_when: 50\n  paths:\n    - a\n    - b\n"
	if got != want {
		t.Errorf("Marshal = %q, want %q", got, want)
	}
}

func TestMarshalEscapesCommandsAndPaths(t *testing.T) {
	t.Parallel()

	at := time.Unix(100, 0)
	got := string(Marshal([]Entry{{
		Cmd: "a\nb", When: at, AddedWhen: at, Paths: []string{`p\q`},
	}}))
	want := "- cmd: a\\nb\n  when: 100\n  paths:\n    - p\\\\q\n"
	if got != want {
		t.Errorf("Marshal = %q, want %q", got, want)
	}
}

func TestParseHandlesInvalidUTF8(t *testing.T) {
	t.Parallel()

	// History files are byte streams; a command can contain any bytes at all.
	raw := []byte("- cmd: echo \xff\xfe\n  when: 100\n")
	entries := Parse(raw)
	if got, want := entries[0].Cmd, "echo \xff\xfe"; got != want {
		t.Errorf("Cmd = %q, want %q", got, want)
	}
	if got := Marshal(entries); !bytes.Equal(got, raw) {
		t.Errorf("round trip of invalid UTF-8 gave %q, want %q", got, raw)
	}
}
