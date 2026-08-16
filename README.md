# yanagiba

A terminal UI for browsing and editing [fish shell](https://fishshell.com) history.

Filter ten years of history with a regular expression, see when each command ran
and which files it touched, and delete or edit entries — with changes staged in
memory until you explicitly write them, and a timestamped backup taken when you do.

```
┌ yanagiba ──────────────────────────────── 4/10333  2 marked ─┐
│ /^git (push|reset)                                 smartcase │
├──────────────────────────────────────────────────────────────┤
│ ✗ git reset --hard HEAD^                          2026-08-08 │
│ ✗ git push -f                                     2026-08-08 │
│ ▸ git restore web/vite.config.ts                  2026-08-08 │
│   git status                                      2019-05-03 │
├──────────────────────────────────────────────────────────────┤
│ git restore web/vite.config.ts                               │
│ last  2026-08-08 14:31  (8 days ago)                         │
│ paths web/vite.config.ts                                     │
├──────────────────────────────────────────────────────────────┤
│ / regex filter · x mark · d delete · e edit · w write · ? help│
└──────────────────────────────────────────────────────────────┘
```

## Install

```sh
make install                  # the binary
make install-fish-function    # optional wrapper that reloads history after editing
```

`make install` installs into `go env GOBIN` (or `$(go env GOPATH)/bin`); override
with `make install INSTALL_DIR=~/bin`. The wrapper is described under
[Safety](#safety) — it is worth having.

## Use

```sh
yanagiba                    # the current fish session's history
yanagiba --file PATH        # a specific history file
yanagiba --check            # verify the file round-trips losslessly, read-only
```

### Keys

| Key | Action |
|---|---|
| `/` | filter with a regular expression |
| `esc` | clear the filter |
| `alt+c` | toggle smart case |
| `ctrl+p` | also match the recorded file paths |
| `j`/`k`, arrows, `g`/`G`, `pgup`/`pgdn` | navigate |
| `s` | toggle newest-first / oldest-first |
| `x` | mark an entry |
| `ctrl+a` / `ctrl+x` | mark everything shown / clear marks |
| `d` | delete the marked entries, or the one under the cursor |
| `e` | edit the command (`ctrl+s` to accept, `esc` to cancel) |
| `u` | undo the last delete or edit |
| `y` | copy the command to the clipboard |
| `w` or `ctrl+s` | write changes to disk |
| `?` | full keybindings |
| `q` | quit (prompts if there are unsaved changes) |

**Smart case**: an all-lowercase pattern matches case-insensitively; typing an
uppercase letter makes the search case-sensitive. Escapes and inline flag groups
(`\S`, `(?i)`) don't count as uppercase.

## Safety

Editing a decade of shell history deserves some care, so:

- **Nothing is written until you press `w`.** Deletes and edits are staged in
  memory and `u` undoes them.
- **A timestamped backup** is written next to the history file on every save;
  the five most recent are kept.
- **Writes are atomic** — a sibling temporary file, fsynced, then `rename(2)` —
  and take the same `flock(2)` fish itself uses. File permissions are preserved.
- **`yanagiba --check`** parses the file, re-serializes it, and reports whether
  the result is byte-identical. It only reads. Run it before trusting the editor
  with a history you care about.

> [!IMPORTANT]
> yanagiba reads the whole history on start and writes a whole new one on save.
> **Commands typed in any fish session while it is open are not in that snapshot
> and are lost when you save.** Don't leave it open in the background.
>
> Measured against fish 4.8.1, the reverse worry is unfounded: a session picks up
> external edits on its own, and exiting does *not* resurrect deleted entries.

### The fish wrapper

```sh
make install-fish-function
```

Installs [`contrib/yanagiba.fish`](contrib/yanagiba.fish) into
`~/.config/fish/functions/`, wrapping the binary so the calling shell stays in
step:

- `history save` **before** — flushes commands this session has run but not yet
  written, so they are in the snapshot yanagiba reads and survive the save.
- `history merge` **after** — re-reads the file so the session reflects the edits
  immediately rather than eventually.

The wrapper passes arguments straight through and preserves the exit status.
It cannot help with commands typed in *other* sessions while the editor is open.

## About the file format

Fish's history is "nearly-valid YAML (but isn't quite)", in the words of a
comment in fish's own source. It is **not** YAML and a YAML parser will corrupt
it: values are unquoted, and newlines and backslashes carry a bespoke escaping
scheme (`\` → `\\`, newline → `\n`).

`internal/fishhist` reimplements fish 4.x's `src/history/yaml_backend.rs` and
`src/history/file.rs` byte for byte — including the quirks, such as decoding
stopping dead at an unrecognised escape. That fidelity is the point: an
untouched file must round-trip unchanged, so that saving can only alter the
entries you actually edited. The test suite pins this, and `--check` proves it
against your own history.

### There is no working directory

Fish records four fields per entry: `cmd`, `when` (last run), `added_when`
(first run), and `paths`. There is no `cwd` — fish simply doesn't store one.
The `paths` column shows the file arguments fish resolved for the command, which
is the closest thing the format offers.

## Development

```sh
make check     # fmt-check, vet, lint, test -- everything CI runs
make demo      # run against the bundled sample history
make help      # list all targets
```

Requires Go 1.25+. `make lint` installs a pinned `golangci-lint` into `bin/`.

## Name

A *yanagiba* (柳刃) is a long, single-bevel Japanese knife for slicing clean.

## License

BSD 3-Clause. See [LICENSE](LICENSE).
