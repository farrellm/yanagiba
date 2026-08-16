//go:build unix

package fishhist

import (
	"os"

	"golang.org/x/sys/unix"
)

// lockFile takes an exclusive advisory lock on path, creating it if needed,
// and returns a function that releases the lock and closes the file.
//
// Fish locks the history file with flock(2) as well, so this genuinely
// interlocks with running shells rather than only with other yanagiba
// processes.
func lockFile(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0o600) //nolint:gosec // the path is user-supplied by design
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, err
	}
	return func() {
		_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
		_ = f.Close()
	}, nil
}
