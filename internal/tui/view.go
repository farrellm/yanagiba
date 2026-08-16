package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Fixed vertical budget: header, filter bar, two rules, detail pane, footer.
const (
	headerHeight = 1
	filterHeight = 1
	footerHeight = 1
	ruleHeight   = 1
	detailHeight = 5
	editorHeight = 6
)

// layout distributes the terminal height across the panes.
func (m *Model) layout() {
	if m.width == 0 || m.height == 0 {
		return
	}

	chrome := headerHeight + filterHeight + footerHeight + 2*ruleHeight
	body := m.height - chrome

	detail := detailHeight
	if m.mode == modeEdit {
		detail = editorHeight
	}
	if body < detail+3 {
		// On a very short terminal, give the list what is left and let the
		// detail pane shrink rather than disappear entirely.
		detail = max(1, body-3)
	}

	listHeight := max(1, body-detail)

	m.list.SetSize(m.width, listHeight)
	m.detail.SetWidth(m.width)
	m.detail.SetHeight(max(1, detail))
	m.filterIn.SetWidth(max(10, m.width-20))
	m.editor.SetWidth(m.width)
	m.editor.SetHeight(max(1, detail))
	m.help.SetWidth(m.width)

	m.updateDetail()
}

// updateDetail refreshes the detail pane for the record under the cursor.
func (m *Model) updateDetail() {
	r, ok := m.selected()
	if !ok {
		m.detail.SetContent(m.styles.Dim.Render("no matching entries"))
		return
	}
	m.detail.SetContent(m.renderDetail(r, time.Now()))
}

func (m Model) renderDetail(r record, now time.Time) string {
	var b strings.Builder

	// Show the command with its real newlines restored, which is the only
	// place a multi-line command is legible.
	cmd := sanitize(r.Cmd)
	for line := range strings.SplitSeq(cmd, "\n") {
		b.WriteString(m.styles.DetailValue.Render(truncate(line, m.width)))
		b.WriteByte('\n')
	}

	label := func(s string) string { return m.styles.DetailLabel.Render(s) }

	b.WriteString(label("last  ") +
		m.styles.DetailValue.Render(r.When.Format("2006-01-02 15:04")) +
		m.styles.Dim.Render("  ("+humanizeSince(r.When, now)+")"))

	if !r.AddedWhen.Equal(r.When) {
		b.WriteString(label("   first ") +
			m.styles.DetailValue.Render(r.AddedWhen.Format("2006-01-02 15:04")))
	}
	b.WriteByte('\n')

	if len(r.Paths) == 0 {
		b.WriteString(label("paths ") + m.styles.Dim.Render("(none)"))
	} else {
		b.WriteString(label("paths ") +
			m.styles.Paths.Render(truncate(strings.Join(r.Paths, "  "), max(1, m.width-6))))
	}

	return b.String()
}

// View satisfies tea.Model.
func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	return v
}

func (m Model) render() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	rule := m.styles.Rule.Render(strings.Repeat("─", m.width))

	bottom := m.renderDetail0()
	if m.mode == modeEdit {
		bottom = m.editor.View()
	}

	sections := []string{
		m.renderHeader(),
		m.renderFilterBar(),
		rule,
		m.list.View(),
		rule,
		bottom,
		m.renderFooter(),
	}

	body := lipgloss.JoinVertical(lipgloss.Left, sections...)

	if overlay := m.renderOverlay(); overlay != "" {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay)
	}
	return body
}

func (m Model) renderDetail0() string { return m.detail.View() }

func (m Model) renderHeader() string {
	title := m.styles.Title.Render("yanagiba")

	shown := len(m.list.Items())
	counts := fmt.Sprintf("%d/%d", shown, m.store.len())
	if n := m.store.markCount(); n > 0 {
		counts += fmt.Sprintf("  %d marked", n)
	}
	if m.store.dirty {
		counts += "  unsaved"
	}

	right := m.styles.Counts.Render(counts)
	if m.store.dirty {
		right = m.styles.Warning.Render(counts)
	}

	gap := max(1, m.width-lipgloss.Width(title)-lipgloss.Width(right))
	return title + strings.Repeat(" ", gap) + right
}

func (m Model) renderFilterBar() string {
	var left string
	switch {
	case m.mode == modeFilter:
		left = m.styles.FilterCue.Render("/") + m.filterIn.View()
	case m.filter.pattern != "":
		left = m.styles.FilterCue.Render("/") + m.styles.Command.Render(m.filter.pattern)
	default:
		left = m.styles.Dim.Render("press / to filter by regular expression")
	}

	// A compile error displaces the flags: knowing why the pattern is rejected
	// matters more than being reminded that smart case is on.
	var right string
	if m.filter.err != nil {
		right = m.styles.FilterErr.Render("⚠ " + m.filter.errorText())
	} else {
		flags := []string{}
		if m.filter.smartCase {
			flags = append(flags, "smartcase")
		}
		if m.filter.matchPaths {
			flags = append(flags, "+paths")
		}
		right = m.styles.FilterFlag.Render(strings.Join(flags, " "))
	}

	// The text input pads itself out to its configured width; that trailing
	// blank space is not content, and measuring it would truncate the pattern
	// for no reason.
	left = strings.TrimRight(left, " ")

	// Truncate the pattern rather than the reason it was rejected.
	if room := m.width - lipgloss.Width(right) - 2; lipgloss.Width(left) > room {
		left = truncate(left, max(1, room))
	}

	gap := max(1, m.width-lipgloss.Width(left)-lipgloss.Width(right))
	return truncate(left+strings.Repeat(" ", gap)+right, m.width)
}

func (m Model) renderFooter() string {
	if m.status != "" {
		style := m.styles.Status
		if m.statusErr {
			style = m.styles.Danger
		}
		return truncate(style.Render(m.status), m.width)
	}

	switch m.mode {
	case modeFilter:
		return m.styles.Dim.Render("enter apply · esc cancel · alt+c smartcase · ctrl+p paths")
	case modeEdit:
		return m.styles.Dim.Render("ctrl+s save edit · esc cancel")
	default:
		return truncate(m.help.View(m.keys), m.width)
	}
}

// dialogInnerWidth is the widest a dialog's content may be, so a long history
// path cannot push the border off the side of the terminal.
func (m Model) dialogInnerWidth() int {
	return max(20, min(m.width-8, 64))
}

func (m Model) renderOverlay() string {
	switch m.mode {
	case modeHelp:
		return m.styles.Dialog.Render(m.help.FullHelpView(m.keys.FullHelp()) +
			"\n\n" + m.styles.Dim.Render("press any key to close"))

	case modeConfirmDelete:
		n := len(m.targetIDs())
		return m.styles.Dialog.Render(
			m.styles.Danger.Render(fmt.Sprintf("Delete %s?", entryCount(n))) +
				"\n" + m.styles.Dim.Render("Staged in memory only; u undoes it, w writes to disk.") +
				"\n\n" + m.styles.DetailValue.Render("y  delete") +
				"    " + m.styles.Dim.Render("any other key  cancel"))

	case modeConfirmSave:
		return m.styles.Dialog.Render(
			m.styles.Title.Render("Write "+entryCount(m.store.len())+" to disk?") +
				"\n" + m.styles.DetailLabel.Render(shortenPath(m.path, m.dialogInnerWidth())) +
				"\n\n" + m.styles.Warning.Render("Anything typed in a fish session since this file was") +
				"\n" + m.styles.Warning.Render("loaded is not in this snapshot and will be lost.") +
				"\n\n" + m.styles.Dim.Render("Other sessions pick these edits up on their own;") +
				"\n" + m.styles.Dim.Render("`history merge` makes it immediate.") +
				"\n\n" + m.styles.Dim.Render("A timestamped backup is written alongside the file.") +
				"\n\n" + m.styles.DetailValue.Render("y  write") +
				"    " + m.styles.Dim.Render("any other key  cancel"))

	case modeConfirmQuit:
		return m.styles.Dialog.Render(
			m.styles.Warning.Render("You have unsaved changes.") +
				"\n\n" + m.styles.DetailValue.Render("w  write and quit") +
				"\n" + m.styles.DetailValue.Render("y  discard and quit") +
				"\n" + m.styles.Dim.Render("any other key  keep editing"))

	default:
		return ""
	}
}
