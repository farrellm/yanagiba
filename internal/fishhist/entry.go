// Package fishhist reads and writes the fish shell history file.
//
// The format is documented in fish's own source as "nearly-valid YAML (but
// isn't quite)". It is not YAML and must not be parsed as such: values are
// unquoted, and newlines and backslashes are escaped with a bespoke scheme.
// Every routine here mirrors fish 4.x's src/history/yaml_backend.rs and
// src/history/file.rs byte for byte, including its quirks, so that a file
// round-tripped through this package is identical to what fish itself
// would have written.
package fishhist

import "time"

// Entry is a single history record.
//
// A record has exactly four fields in fish's format. Notably there is no
// working directory: fish does not record one. Paths holds the file arguments
// fish resolved for the command, which is the closest thing available.
type Entry struct {
	// Cmd is the command line, unescaped. It may contain newlines and is not
	// guaranteed to be valid UTF-8.
	Cmd string

	// When is the last time the command was run. Fish deduplicates history and
	// bumps this timestamp on each repeat.
	When time.Time

	// AddedWhen is the first time the command was run. Fish only writes this
	// key when it differs from When; for entries without it, parsing sets it
	// equal to When.
	AddedWhen time.Time

	// Paths holds the file paths fish resolved as arguments to the command.
	Paths []string
}

// Clone returns a deep copy of e, so that mutating the copy's Paths does not
// affect the original.
func (e Entry) Clone() Entry {
	if e.Paths != nil {
		e.Paths = append([]string(nil), e.Paths...)
	}
	return e
}
