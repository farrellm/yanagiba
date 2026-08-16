# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

`yanagiba` is a Bubble Tea TUI for browsing and editing fish shell history.

## Commands

`make` is the single entry point; CI shells out to these same targets, so a green
`make check` locally means a green pipeline.

```sh
make check         # fmt-check, vet, lint, check-fish, test -- everything CI runs
make test          # go test -race -covermode=atomic ./...
make lint          # golangci-lint (installs pinned v2.12.2 into bin/ on demand)
make fmt           # gofumpt + goimports, driven through golangci-lint v2
make demo          # run the TUI against the bundled fixture, never real history
make help          # all targets
```

Single test:

```sh
go test ./internal/fishhist/ -run TestRoundTripSampleFile -v
```

Read-only verification against a real history file — parses, re-serializes, and
diffs. Run this before letting any change to the codec near real data:

```sh
make check-history                              # the current session's history
go run ./cmd/yanagiba --check --file PATH       # a specific file
```

Driving the TUI non-interactively (it takes over the terminal) needs tmux:

```sh
cp internal/fishhist/testdata/sample_history /tmp/demo_history
tmux new-session -d -s yana -x 100 -y 30 "$PWD/bin/yanagiba --file /tmp/demo_history"
tmux send-keys -t yana '/'; tmux send-keys -t yana '^git'
tmux capture-pane -t yana -p
```

Always point it at a **copy**. A save rewrites the file it was given.

## The history format is not YAML

Fish's own source calls it "nearly-valid YAML (but isn't quite)". Values are
unquoted and use a bespoke escaping scheme (`\` → `\\`, newline → `\n`). **A YAML
parser corrupts it** — real histories contain entries that are invalid YAML
outright.

`internal/fishhist` reimplements fish 4.x's `src/history/yaml_backend.rs` and
`src/history/file.rs` byte for byte. When changing anything there, consult that
Rust source, not intuition, and preserve the quirks it has:

- decoding **stops dead** at an unrecognised escape or a trailing lone backslash,
  discarding the rest of the value
- **all** leading whitespace after the `:` is trimmed, so a command typed with
  leading spaces loses them
- `added_when` is written only when it differs from `when`; `paths` only when
  non-empty
- an item starts at a column-0 line prefixed `- cmd` (fish checks the prefix, not
  `- cmd: `); `%`, `---`, `...`, `\0` lines and unknown keys are skipped
- entries are byte streams, not UTF-8 — parse over `[]byte`, sanitize only when
  rendering

### The round-trip invariant

**Parse → Marshal of an untouched file must be byte-identical.** That is what
makes a save alter only the entries the user actually edited, instead of silently
rewriting a decade of history. `TestRoundTripSampleFile` guards it; `--check`
proves it against real data.

Consequences: `internal/fishhist/testdata/sample_history` is a byte-exact fixture —
do not reformat it (`.editorconfig` exempts it), and only add entries that
round-trip cleanly. Lossy cases (truncating escapes, leading-space commands) are
tested inline in Go instead.

### Fish records no working directory

An entry has exactly four fields: `cmd`, `when` (last run), `added_when` (first
run), `paths`. There is no `cwd` and no way to derive one. The location column
shows `paths` — the file arguments fish resolved. Don't add a "directory" feature
expecting a working directory to exist.

### Never commit real history as test data

Real fish histories contain secrets — this repo's owner's history had live-looking
API keys in `curl` commands. Fixtures are synthetic; `--check` reads real files
without copying them anywhere.

## TUI architecture (`internal/tui`)

`store` holds the working copy and is the only mutable state; the Bubble Tea
`Model` wraps it. Two invariants matter:

- **`store.records` keep file order and are never reordered.** Fish writes history
  append-only, oldest first. Display sorting is a view concern — `Model.visible()`
  filters and sorts a derived slice. Sorting the store would rewrite the entire
  file on save.
- **Records carry a stable `id`.** Marks, edits and undo all reference IDs, never
  positions, because a delete shifts every position after it.

Edits are **staged**: `deleteIDs`/`editID` mutate memory and push onto an undo
stack; nothing touches disk until the user confirms a write. These are plain
methods on `store`, deliberately outside `Update`, so they are unit-testable
without a terminal — keep new mutations there too.

Marks are read from the store rather than the visible list, so an entry marked and
then hidden by a filter is still included in a delete.

## Charm v2

The v2 line moved to **`charm.land/*` vanity module paths**, not
`github.com/charmbracelet/*`. The API differs from v1 in ways that break silently:

- `View()` returns `tea.View`, not `string`; alt screen and mouse are declarative
  `View` fields, **not** `tea.WithAltScreen()` program options
- `tea.KeyPressMsg`, not `tea.KeyMsg` (now an interface); `msg.Code` / `msg.Text` /
  `msg.Mod`; space stringifies as `"space"`, not `" "`
- bubbles v2 uses setters: `textinput.SetWidth(n)`, `viewport.New(viewport.WithWidth(w))`,
  `help.SetWidth(n)`, `textinput.DefaultKeyMap()` is a function

The list component's built-in fuzzy filter is disabled (`SetFilteringEnabled(false)`)
so the regex filter in `filter.go` can own the semantics.

## Writing history safely

`fishhist.WriteFile` mirrors fish's own strategy: stat first (the flock open would
otherwise create the file and hide whether it existed), take `flock(2)` — the same
lock fish uses — back up to a timestamped sibling, then write a sibling temp file,
fsync, and `rename(2)`. The temp file must be a **sibling** so the rename stays on
one filesystem. Permissions are preserved; a history file can hold secrets.

The unavoidable hazard is the read-modify-write window: a command typed in any fish
session while the editor is open is not in the snapshot and is lost on save. Tested
against fish 4.8.1, the reverse worry is unfounded — sessions pick up external edits
on their own and exiting does not resurrect deleted entries. `contrib/yanagiba.fish`
runs `history save` before and `history merge` after to close the near end of that
window. Keep the warnings in the save dialog and README consistent with this.

## Linting

`.golangci.yml` uses the golangci-lint **v2** schema (`version: "2"`; formatters
live in their own top-level `formatters:` block). gocritic's `hugeParam` is
disabled deliberately: Bubble Tea is built on value receivers and passing models by
pointer would fight the framework. Unix-only — fish does not run on Windows and the
lock uses `flock(2)`, so CI covers ubuntu and macos only.
