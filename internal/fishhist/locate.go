package fishhist

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultPath returns the path of the history file for the current fish
// session.
//
// Fish stores history per named session: the $fish_history variable holds the
// session name and the file is <name>_history, defaulting to "fish". The
// variable is exported into the environment of child processes, so honouring it
// means yanagiba edits the same history the invoking shell is using.
func DefaultPath() (string, error) {
	dir, err := dataDir()
	if err != nil {
		return "", err
	}

	session := os.Getenv("fish_history")
	if session == "" {
		session = "fish"
	}
	return filepath.Join(dir, session+"_history"), nil
}

func dataDir() (string, error) {
	if dir := os.Getenv("__fish_user_data_dir"); dir != "" {
		return dir, nil
	}
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "fish"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locating home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", "fish"), nil
}
