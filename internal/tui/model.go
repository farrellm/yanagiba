package tui

import (
	"sort"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/farrellm/yanagiba/internal/fishhist"
)

// mode is which input the keyboard is currently driving.
type mode int

const (
	modeBrowse mode = iota
	modeFilter
	modeEdit
	modeConfirmDelete
	modeConfirmSave
	modeConfirmQuit
	modeHelp
)

// sortOrder controls the display order only; the file is always written in its
// original order.
type sortOrder int

const (
	sortNewest sortOrder = iota
	sortOldest
)

func (o sortOrder) String() string {
	if o == sortOldest {
		return "oldest first"
	}
	return "newest first"
}

// Model is the root Bubble Tea model.
type Model struct {
	path  string
	store *store

	list     list.Model
	filterIn textinput.Model
	editor   textarea.Model
	detail   viewport.Model
	help     help.Model
	keys     keyMap
	styles   styles

	mode   mode
	filter filter
	order  sortOrder

	// editingID is the record being edited in modeEdit.
	editingID int

	width  int
	height int

	status    string
	statusErr bool

	quitting bool
}

// New builds a model over the entries loaded from path.
func New(path string, entries []fishhist.Entry) Model {
	st := newStore(entries)
	sty := newStyles(true)

	m := Model{
		path:   path,
		store:  st,
		keys:   defaultKeyMap(),
		styles: sty,
		filter: compileFilter("", true, false),
		order:  sortNewest,
		help:   help.New(),
	}

	m.filterIn = textinput.New()
	m.filterIn.Prompt = ""
	m.filterIn.Placeholder = "regular expression"

	m.editor = textarea.New()
	m.editor.Prompt = ""
	m.editor.ShowLineNumbers = false
	m.editor.CharLimit = 0

	m.detail = viewport.New()

	delegate := entryDelegate{store: st, styles: &m.styles}
	m.list = list.New(nil, delegate, 0, 0)
	m.list.SetShowTitle(false)
	m.list.SetShowStatusBar(false)
	m.list.SetShowHelp(false)
	m.list.SetShowFilter(false)
	m.list.SetShowPagination(false)
	// The list's own fuzzy filter is replaced by the regex filter, and its quit
	// bindings would bypass the unsaved-changes prompt.
	m.list.SetFilteringEnabled(false)
	m.list.DisableQuitKeybindings()

	m.refreshItems()
	return m
}

// Init satisfies tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.RequestBackgroundColor
}

// visible returns the records passing the current filter, in display order.
func (m Model) visible() []record {
	out := make([]record, 0, len(m.store.records))
	for _, r := range m.store.records {
		if m.filter.matches(r.Entry) {
			out = append(out, r)
		}
	}

	// The store keeps file order (oldest first); newest-first is just the
	// reverse, so a stable sort by timestamp is only needed to keep entries
	// with equal timestamps in a predictable order.
	sort.SliceStable(out, func(i, j int) bool {
		if m.order == sortOldest {
			return out[i].When.Before(out[j].When)
		}
		return out[j].When.Before(out[i].When)
	})
	return out
}

// refreshItems rebuilds the list contents from the store and filter, keeping
// the cursor on the same record where possible.
func (m *Model) refreshItems() {
	selectedID := -1
	if r, ok := m.selected(); ok {
		selectedID = r.id
	}

	records := m.visible()
	items := make([]list.Item, len(records))
	for i, r := range records {
		items[i] = entryItem{rec: r}
	}
	m.list.SetItems(items)

	if selectedID >= 0 {
		for i, r := range records {
			if r.id == selectedID {
				m.list.Select(i)
				break
			}
		}
	}
	if m.list.Index() >= len(items) {
		m.list.Select(max(0, len(items)-1))
	}

	m.updateDetail()
}

// selected returns the record under the cursor.
func (m Model) selected() (record, bool) {
	it, ok := m.list.SelectedItem().(entryItem)
	if !ok {
		return record{}, false
	}
	return it.rec, true
}

// targetIDs returns the records an action applies to: the marked ones if any
// are marked, otherwise the one under the cursor. Marks that the filter is
// currently hiding still count, which is why this reads the store rather than
// the visible list.
func (m Model) targetIDs() []int {
	if m.store.markCount() > 0 {
		ids := make([]int, 0, m.store.markCount())
		for _, r := range m.store.records {
			if m.store.isMarked(r.id) {
				ids = append(ids, r.id)
			}
		}
		return ids
	}
	if r, ok := m.selected(); ok {
		return []int{r.id}
	}
	return nil
}

func (m *Model) setStatus(s string)    { m.status, m.statusErr = s, false }
func (m *Model) setErrStatus(s string) { m.status, m.statusErr = s, true }
