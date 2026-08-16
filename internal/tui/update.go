package tui

import (
	"fmt"
	"path/filepath"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/farrellm/yanagiba/internal/fishhist"
)

// savedMsg reports the outcome of writing the history file.
type savedMsg struct {
	backup string
	count  int
	err    error
}

// Update satisfies tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		m.styles = newStyles(msg.IsDark())
		m.list.SetDelegate(entryDelegate{store: m.store, styles: &m.styles})
		return m, nil

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		return m, nil

	case savedMsg:
		return m.handleSaved(msg), nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleSaved(msg savedMsg) Model {
	if msg.err != nil {
		m.setErrStatus("save failed: " + msg.err.Error())
		return m
	}
	m.store.dirty = false
	m.store.undos = nil
	m.setStatus(fmt.Sprintf("wrote %s to %s (backup: %s)",
		entryCount(msg.count),
		filepath.Base(m.path), filepath.Base(msg.backup)))
	if m.quitting {
		m.quitting = false
	}
	return m
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeFilter:
		return m.handleFilterKey(msg)
	case modeEdit:
		return m.handleEditKey(msg)
	case modeConfirmDelete:
		return m.handleConfirmDelete(msg)
	case modeConfirmSave:
		return m.handleConfirmSave(msg)
	case modeConfirmQuit:
		return m.handleConfirmQuit(msg)
	case modeHelp:
		m.mode = modeBrowse
		return m, nil
	default:
		return m.handleBrowseKey(msg)
	}
}

func (m Model) handleBrowseKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	m.status = ""

	switch {
	case key.Matches(msg, m.keys.Quit):
		if m.store.dirty {
			m.mode = modeConfirmQuit
			return m, nil
		}
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		m.mode = modeHelp
		return m, nil

	case key.Matches(msg, m.keys.Filter):
		m.mode = modeFilter
		m.filterIn.SetValue(m.filter.pattern)
		m.filterIn.CursorEnd()
		return m, m.filterIn.Focus()

	case key.Matches(msg, m.keys.ClearFilter):
		if m.filter.active() || m.filter.pattern != "" {
			m.filter = compileFilter("", m.filter.smartCase, m.filter.matchPaths)
			m.filterIn.SetValue("")
			m.refreshItems()
		}
		return m, nil

	case key.Matches(msg, m.keys.SmartCase):
		m.filter = compileFilter(m.filter.pattern, !m.filter.smartCase, m.filter.matchPaths)
		m.refreshItems()
		m.setStatus("smart case " + onOff(m.filter.smartCase))
		return m, nil

	case key.Matches(msg, m.keys.MatchPaths):
		m.filter = compileFilter(m.filter.pattern, m.filter.smartCase, !m.filter.matchPaths)
		m.refreshItems()
		m.setStatus("match paths " + onOff(m.filter.matchPaths))
		return m, nil

	case key.Matches(msg, m.keys.Sort):
		if m.order == sortNewest {
			m.order = sortOldest
		} else {
			m.order = sortNewest
		}
		m.refreshItems()
		m.setStatus("sorted " + m.order.String())
		return m, nil

	case key.Matches(msg, m.keys.Mark):
		if r, ok := m.selected(); ok {
			m.store.toggleMark(r.id)
			m.list.CursorDown()
			m.updateDetail()
		}
		return m, nil

	case key.Matches(msg, m.keys.MarkAll):
		ids := make([]int, 0, len(m.list.Items()))
		for _, it := range m.list.Items() {
			if ei, ok := it.(entryItem); ok {
				ids = append(ids, ei.rec.id)
			}
		}
		m.store.mark(ids)
		m.setStatus(fmt.Sprintf("marked %s", entryCount(len(ids))))
		return m, nil

	case key.Matches(msg, m.keys.Unmark):
		m.store.clearMarks()
		m.setStatus("cleared marks")
		return m, nil

	case key.Matches(msg, m.keys.Delete):
		if len(m.targetIDs()) == 0 {
			return m, nil
		}
		m.mode = modeConfirmDelete
		return m, nil

	case key.Matches(msg, m.keys.Edit):
		r, ok := m.selected()
		if !ok {
			return m, nil
		}
		m.mode = modeEdit
		m.editingID = r.id
		m.editor.SetValue(r.Cmd)
		m.layout()
		return m, m.editor.Focus()

	case key.Matches(msg, m.keys.Undo):
		if what, ok := m.store.undo(); ok {
			m.refreshItems()
			m.setStatus(what)
		} else {
			m.setStatus("nothing to undo")
		}
		return m, nil

	case key.Matches(msg, m.keys.Yank):
		if r, ok := m.selected(); ok {
			m.setStatus("copied to clipboard")
			return m, tea.SetClipboard(r.Cmd)
		}
		return m, nil

	case key.Matches(msg, m.keys.Save):
		if !m.store.dirty {
			m.setStatus("no changes to write")
			return m, nil
		}
		m.mode = modeConfirmSave
		return m, nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	m.updateDetail()
	return m, cmd
}

func (m Model) handleFilterKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Abandon the in-progress pattern and restore what was applied before.
		m.mode = modeBrowse
		m.filterIn.Blur()
		m.filterIn.SetValue(m.filter.pattern)
		return m, nil
	case "enter":
		m.mode = modeBrowse
		m.filterIn.Blur()
		return m, nil
	case "ctrl+c":
		return m, tea.Quit

	// The filter bar advertises these, so they have to work while typing --
	// which is the only time anyone wants to reach for them.
	case "alt+c":
		m.filter = compileFilter(m.filterIn.Value(), !m.filter.smartCase, m.filter.matchPaths)
		m.refreshItems()
		return m, nil
	case "ctrl+p":
		m.filter = compileFilter(m.filterIn.Value(), m.filter.smartCase, !m.filter.matchPaths)
		m.refreshItems()
		return m, nil
	}

	var cmd tea.Cmd
	m.filterIn, cmd = m.filterIn.Update(msg)

	// Recompile on every keystroke. A linear regexp scan over ten thousand
	// commands is far below a frame budget, so there is no need to debounce.
	m.filter = compileFilter(m.filterIn.Value(), m.filter.smartCase, m.filter.matchPaths)
	if m.filter.err == nil {
		m.refreshItems()
	}
	return m, cmd
}

func (m Model) handleEditKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeBrowse
		m.editor.Blur()
		m.layout()
		m.setStatus("edit cancelled")
		return m, nil

	case "ctrl+s", "alt+enter":
		value := m.editor.Value()
		m.mode = modeBrowse
		m.editor.Blur()
		m.layout()
		if value == "" {
			m.setErrStatus("refusing to save an empty command; delete it instead")
			return m, nil
		}
		if m.store.editID(m.editingID, value) {
			m.refreshItems()
			m.setStatus("edited (unsaved)")
		} else {
			m.setStatus("unchanged")
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.editor, cmd = m.editor.Update(msg)
	return m, cmd
}

func (m Model) handleConfirmDelete(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		n := m.store.deleteIDs(m.targetIDs())
		m.mode = modeBrowse
		m.refreshItems()
		m.setStatus(fmt.Sprintf("deleted %s (unsaved -- press u to undo)",
			entryCount(n)))
		return m, nil
	default:
		m.mode = modeBrowse
		return m, nil
	}
}

func (m Model) handleConfirmSave(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		m.mode = modeBrowse
		return m, saveCmd(m.path, m.store.entries())
	default:
		m.mode = modeBrowse
		return m, nil
	}
}

func (m Model) handleConfirmQuit(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		return m, tea.Quit
	case "w", "W":
		m.mode = modeBrowse
		m.quitting = true
		return m, saveCmd(m.path, m.store.entries())
	default:
		m.mode = modeBrowse
		return m, nil
	}
}

func saveCmd(path string, entries []fishhist.Entry) tea.Cmd {
	return func() tea.Msg {
		backup, err := fishhist.WriteFile(path, entries)
		return savedMsg{backup: backup, count: len(entries), err: err}
	}
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}
