package fishhist

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

// CheckResult reports whether a history file survives a parse/serialize round
// trip unchanged.
type CheckResult struct {
	Entries    int
	InputSize  int
	OutputSize int
	// Diffs holds a human-readable description of each line that differs,
	// capped at maxReportedDiffs.
	Diffs []string
}

// OK reports whether the round trip was lossless.
func (r CheckResult) OK() bool { return len(r.Diffs) == 0 }

const maxReportedDiffs = 20

// Check parses path, re-serializes it, and compares the result against the
// original bytes. It never writes anything.
//
// This is the safe way to gain confidence in the codec against a real history
// file before letting the editor near it: a lossless round trip means saving
// can only change the entries the user actually edited.
func Check(path string) (CheckResult, error) {
	original, err := os.ReadFile(path) //nolint:gosec // the path is user-supplied by design
	if err != nil {
		return CheckResult{}, err
	}

	entries := Parse(original)
	produced := Marshal(entries)

	result := CheckResult{
		Entries:    len(entries),
		InputSize:  len(original),
		OutputSize: len(produced),
	}

	if bytes.Equal(produced, original) {
		return result, nil
	}

	origLines := splitLines(original)
	newLines := splitLines(produced)
	for i := 0; i < len(origLines) || i < len(newLines); i++ {
		var a, b []byte
		if i < len(origLines) {
			a = origLines[i]
		}
		if i < len(newLines) {
			b = newLines[i]
		}
		if bytes.Equal(a, b) {
			continue
		}
		if len(result.Diffs) >= maxReportedDiffs {
			result.Diffs = append(result.Diffs, "... further differences omitted")
			break
		}
		result.Diffs = append(result.Diffs,
			fmt.Sprintf("line %d:\n  original: %q\n  rewritten: %q", i+1, a, b))
	}

	return result, nil
}

// WriteReport prints a human-readable summary of a check to w.
func (r CheckResult) WriteReport(w io.Writer, path string) {
	fmt.Fprintf(w, "%s\n", path)
	fmt.Fprintf(w, "  entries: %d\n", r.Entries)
	fmt.Fprintf(w, "  bytes:   %d in, %d out\n", r.InputSize, r.OutputSize)

	if r.OK() {
		fmt.Fprintf(w, "  round trip: lossless\n")
		return
	}

	fmt.Fprintf(w, "  round trip: MISMATCH (%d differing lines shown)\n", len(r.Diffs))
	for _, d := range r.Diffs {
		fmt.Fprintf(w, "  %s\n", d)
	}
}
