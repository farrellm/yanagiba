package fishhist

import (
	"bytes"
	"os"
	"strconv"
	"time"
)

// Field keys used by the history format.
const (
	keyLastAdded  = "when"
	keyFirstAdded = "added_when"
	keyPaths      = "paths"
)

const itemPrefix = "- cmd"

// ParseFile reads and parses a history file.
func ParseFile(path string) ([]Entry, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the path is user-supplied by design
	if err != nil {
		return nil, err
	}
	return Parse(data), nil
}

// Parse decodes history entries from the raw contents of a history file.
//
// Malformed records are skipped rather than reported, matching fish, which
// logs and moves on. A history file accumulated over years will contain
// corruption from interrupted writes, and refusing to open it would be worse
// than dropping the damaged records.
func Parse(data []byte) []Entry {
	var entries []Entry

	lines := splitLines(data)
	for i := 0; i < len(lines); {
		line := lines[i]

		// Interior lines of an item are indented; a well-formed file never
		// reaches them here, but a corrupt one can.
		if len(line) > 0 && line[0] == ' ' {
			i++
			continue
		}
		// Fish tolerates a few real-YAML markers, and skips NUL-prefixed
		// lines left behind by interrupted writes.
		if hasAnyPrefix(line, "%", "---", "...", "\x00") {
			i++
			continue
		}
		if !bytes.HasPrefix(line, []byte(itemPrefix)) {
			i++
			continue
		}

		entry, next, ok := decodeItem(lines, i)
		if !ok {
			i++
			continue
		}
		entries = append(entries, entry)
		i = next
	}

	return entries
}

// decodeItem decodes the item beginning at lines[start], returning the index
// of the first line after it. It mirrors fish's decode_item_fish_2_0.
func decodeItem(lines [][]byte, start int) (Entry, int, bool) {
	_, value, ok := splitKeyValue(lines[start])
	if !ok {
		return Entry{}, start, false
	}

	entry := Entry{Cmd: string(unescape(value))}

	var (
		haveLastAdded  bool
		haveFirstAdded bool
		lastAdded      time.Time
		firstAdded     time.Time
		indent         = -1
	)

	i := start + 1
	for i < len(lines) {
		thisIndent, rest := trimLeadingSpaces(lines[i])
		if indent < 0 {
			indent = thisIndent
		}
		// The first indented line fixes the indent for the whole item; a line
		// at column zero, or at a different indent, ends it.
		if thisIndent == 0 || thisIndent != indent {
			break
		}

		key, value, ok := splitKeyValue(rest)
		if !ok {
			break
		}
		i++ // this line belongs to the item regardless of whether we use it

		switch string(key) {
		case keyLastAdded:
			if t, ok := parseTimestamp(value); ok {
				lastAdded = t
				haveLastAdded = true
				// Fish seeds first_added from the first timestamp it sees, so
				// an entry with only a "when" reports equal timestamps.
				if !haveFirstAdded {
					firstAdded = t
					haveFirstAdded = true
				}
			}
		case keyFirstAdded:
			if t, ok := parseTimestamp(value); ok {
				firstAdded = t
				haveFirstAdded = true
			}
		case keyPaths:
			i = decodePaths(lines, i, indent, &entry)
		default:
			// Unknown keys are consumed and ignored, as fish does.
		}
	}

	if haveLastAdded {
		entry.When = lastAdded
	} else {
		entry.When = time.Unix(0, 0)
	}
	if haveFirstAdded {
		entry.AddedWhen = firstAdded
	} else {
		entry.AddedWhen = time.Unix(0, 0)
	}

	return entry, i, true
}

// decodePaths consumes the "- " list items following a paths: key, returning
// the index of the first line that is not part of the list.
func decodePaths(lines [][]byte, i, indent int, entry *Entry) int {
	for i < len(lines) {
		leading, rest := trimLeadingSpaces(lines[i])
		if leading <= indent {
			break
		}
		if !bytes.HasPrefix(rest, []byte("- ")) {
			break
		}
		entry.Paths = append(entry.Paths, string(unescape(rest[2:])))
		i++
	}
	return i
}

// splitKeyValue splits a line at the first colon and trims leading whitespace
// from the value, mirroring fish's extract_prefix_and_unescape_yaml. The key is
// returned unescaped by fish too, but keys never contain escapes in practice.
func splitKeyValue(line []byte) (key, value []byte, ok bool) {
	idx := bytes.IndexByte(line, ':')
	if idx < 0 {
		return nil, nil, false
	}
	return line[:idx], trimLeadingASCIISpace(line[idx+1:]), true
}

func parseTimestamp(value []byte) (time.Time, bool) {
	secs, err := strconv.ParseInt(string(value), 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(secs, 0), true
}

// splitLines splits on newlines and drops the trailing element, so that a
// file ending in a newline does not yield a final empty line.
func splitLines(data []byte) [][]byte {
	if len(data) == 0 {
		return nil
	}
	lines := bytes.Split(data, []byte("\n"))
	return lines[:len(lines)-1]
}

func trimLeadingSpaces(s []byte) (count int, rest []byte) {
	for count < len(s) && s[count] == ' ' {
		count++
	}
	return count, s[count:]
}

func trimLeadingASCIISpace(s []byte) []byte {
	i := 0
	for i < len(s) && isASCIISpace(s[i]) {
		i++
	}
	return s[i:]
}

func isASCIISpace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	default:
		return false
	}
}

func hasAnyPrefix(s []byte, prefixes ...string) bool {
	for _, p := range prefixes {
		if bytes.HasPrefix(s, []byte(p)) {
			return true
		}
	}
	return false
}
