# Wrapper that keeps the calling shell in step with the history it just edited.
#
# yanagiba is a read-modify-write editor: it parses the history file on start
# and writes a whole new one on save. Two things follow, and this function
# handles both.
#
#   history save    Flushes commands this session has run but not yet written
#                   to disk. Anything missing from the snapshot yanagiba reads
#                   is absent from the file it writes -- that is how a
#                   just-typed command gets lost.
#
#   history merge   Re-reads the file afterwards so this session reflects the
#                   edits right away. fish picks external changes up on its own
#                   soon enough, but merge makes it immediate and explicit.
#
# Install with `make install-fish-function`, or copy this file to
# ~/.config/fish/functions/yanagiba.fish -- the name must match for autoloading.
#
# Note this cannot protect against commands typed in *other* fish sessions
# while the editor is open; those are not in the snapshot and are lost on save.
# Don't leave yanagiba sitting open in the background.

function yanagiba --description 'Edit fish history, then reload it'
    history save

    command yanagiba $argv
    set --local yanagiba_status $status

    history merge

    return $yanagiba_status
end
