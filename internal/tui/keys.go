package tui

import "charm.land/bubbles/v2/key"

// keyMap holds every binding the browse view responds to. Editing, filtering
// and confirmation modes have their own small sets of keys handled inline.
type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Home     key.Binding
	End      key.Binding

	Filter      key.Binding
	ClearFilter key.Binding
	SmartCase   key.Binding
	MatchPaths  key.Binding

	Mark    key.Binding
	MarkAll key.Binding
	Unmark  key.Binding

	Delete key.Binding
	Edit   key.Binding
	Undo   key.Binding
	Yank   key.Binding
	Sort   key.Binding

	Save key.Binding
	Help key.Binding
	Quit key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+u"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+d"),
			key.WithHelp("pgdn", "page down"),
		),
		Home: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("g", "top"),
		),
		End: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("G", "bottom"),
		),

		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "regex filter"),
		),
		ClearFilter: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "clear filter"),
		),
		// Deliberately not ctrl+s: in a tool whose consequential action is
		// writing the file, ctrl+s must mean save everywhere or nowhere.
		SmartCase: key.NewBinding(
			key.WithKeys("alt+c"),
			key.WithHelp("alt+c", "smart case"),
		),
		MatchPaths: key.NewBinding(
			key.WithKeys("ctrl+p"),
			key.WithHelp("ctrl+p", "match paths"),
		),

		Mark: key.NewBinding(
			key.WithKeys("x", " "),
			key.WithHelp("x", "mark"),
		),
		MarkAll: key.NewBinding(
			key.WithKeys("ctrl+a"),
			key.WithHelp("ctrl+a", "mark all shown"),
		),
		Unmark: key.NewBinding(
			key.WithKeys("ctrl+x"),
			key.WithHelp("ctrl+x", "clear marks"),
		),

		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e", "enter"),
			key.WithHelp("e", "edit"),
		),
		Undo: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "undo"),
		),
		Yank: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "copy"),
		),
		Sort: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "sort"),
		),

		Save: key.NewBinding(
			key.WithKeys("w", "ctrl+s"),
			key.WithHelp("w", "write"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

// ShortHelp is the one-line footer.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Filter, k.Mark, k.Delete, k.Edit, k.Save, k.Help, k.Quit}
}

// FullHelp is the expanded help overlay.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageUp, k.PageDown, k.Home, k.End},
		{k.Filter, k.ClearFilter, k.SmartCase, k.MatchPaths, k.Sort},
		{k.Mark, k.MarkAll, k.Unmark, k.Yank},
		{k.Delete, k.Edit, k.Undo, k.Save},
		{k.Help, k.Quit},
	}
}
