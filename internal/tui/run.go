package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/farrellm/yanagiba/internal/fishhist"
)

// Run starts the interactive editor over the history file at path.
func Run(path string, entries []fishhist.Entry) error {
	_, err := tea.NewProgram(New(path, entries)).Run()
	return err
}
