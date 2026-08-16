// Command yanagiba is a terminal UI for browsing and editing fish shell
// history.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/farrellm/yanagiba/internal/fishhist"
	"github.com/farrellm/yanagiba/internal/tui"
)

// version is stamped at build time via -ldflags.
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "yanagiba:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		path        = flag.String("file", "", "history file to edit (default: the current fish session's)")
		check       = flag.Bool("check", false, "verify the file survives a parse/serialize round trip, then exit")
		showVersion = flag.Bool("version", false, "print the version and exit")
	)
	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		fmt.Println("yanagiba", version)
		return nil
	}

	resolved, err := resolvePath(*path)
	if err != nil {
		return err
	}

	if *check {
		return runCheck(resolved)
	}

	entries, err := fishhist.ParseFile(resolved)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return fmt.Errorf("no history entries found in %s", resolved)
	}

	return tui.Run(resolved, entries)
}

func resolvePath(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	return fishhist.DefaultPath()
}

// runCheck reports whether the codec is lossless for this file. It only reads.
//
// This is worth running against a real history file before trusting the editor
// with it: a lossless round trip means saving can only change the entries that
// were actually edited.
func runCheck(path string) error {
	result, err := fishhist.Check(path)
	if err != nil {
		return err
	}
	result.WriteReport(os.Stdout, path)
	if !result.OK() {
		return fmt.Errorf("round trip is not lossless; refusing to vouch for %s", path)
	}
	return nil
}

func usage() {
	out := flag.CommandLine.Output()
	fmt.Fprintf(out, `yanagiba %s -- browse and edit fish shell history

Usage:
  yanagiba [flags]

Flags:
`, version)
	flag.PrintDefaults()
	fmt.Fprintf(out, `
Keys:
  /            filter with a regular expression
  x            mark an entry        ctrl+a  mark all shown
  d            delete marked (or the current entry)
  e            edit the command     u       undo
  w            write changes to disk
  ?            all keybindings      q       quit

Changes are staged in memory; nothing is written until you press w, and a
timestamped backup is made alongside the history file when you do.
`)
}
