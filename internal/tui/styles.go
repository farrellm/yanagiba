package tui

import "charm.land/lipgloss/v2"

// styles holds every lipgloss style the views use. They are rebuilt whenever
// the terminal reports its background colour, so the palette adapts to light
// and dark terminals.
type styles struct {
	Title      lipgloss.Style
	Counts     lipgloss.Style
	FilterCue  lipgloss.Style
	FilterFlag lipgloss.Style
	FilterErr  lipgloss.Style

	Cursor     lipgloss.Style
	Command    lipgloss.Style
	SelectedBg lipgloss.Style
	MarkOn     lipgloss.Style
	MarkOff    lipgloss.Style
	Date       lipgloss.Style
	Paths      lipgloss.Style
	Dim        lipgloss.Style

	DetailLabel lipgloss.Style
	DetailValue lipgloss.Style
	Rule        lipgloss.Style

	Status  lipgloss.Style
	Warning lipgloss.Style
	Danger  lipgloss.Style
	Dialog  lipgloss.Style
}

func newStyles(dark bool) styles {
	c := lipgloss.LightDark(dark)

	var (
		accent    = c(lipgloss.Color("#8250df"), lipgloss.Color("#c297ff"))
		secondary = c(lipgloss.Color("#0969da"), lipgloss.Color("#79c0ff"))
		muted     = c(lipgloss.Color("#6e7781"), lipgloss.Color("#8b949e"))
		faint     = c(lipgloss.Color("#8c959f"), lipgloss.Color("#6e7681"))
		text      = c(lipgloss.Color("#1f2328"), lipgloss.Color("#e6edf3"))
		warn      = c(lipgloss.Color("#9a6700"), lipgloss.Color("#e3b341"))
		danger    = c(lipgloss.Color("#cf222e"), lipgloss.Color("#ff7b72"))
		good      = c(lipgloss.Color("#1a7f37"), lipgloss.Color("#3fb950"))
	)

	return styles{
		Title:      lipgloss.NewStyle().Bold(true).Foreground(accent),
		Counts:     lipgloss.NewStyle().Foreground(muted),
		FilterCue:  lipgloss.NewStyle().Foreground(accent).Bold(true),
		FilterFlag: lipgloss.NewStyle().Foreground(faint),
		FilterErr:  lipgloss.NewStyle().Foreground(danger),

		Cursor:     lipgloss.NewStyle().Foreground(accent).Bold(true),
		Command:    lipgloss.NewStyle().Foreground(text),
		SelectedBg: lipgloss.NewStyle().Foreground(accent).Bold(true),
		MarkOn:     lipgloss.NewStyle().Foreground(danger).Bold(true),
		MarkOff:    lipgloss.NewStyle().Foreground(faint),
		Date:       lipgloss.NewStyle().Foreground(muted),
		Paths:      lipgloss.NewStyle().Foreground(secondary),
		Dim:        lipgloss.NewStyle().Foreground(faint),

		DetailLabel: lipgloss.NewStyle().Foreground(muted),
		DetailValue: lipgloss.NewStyle().Foreground(text),
		Rule:        lipgloss.NewStyle().Foreground(faint),

		Status:  lipgloss.NewStyle().Foreground(good),
		Warning: lipgloss.NewStyle().Foreground(warn),
		Danger:  lipgloss.NewStyle().Foreground(danger).Bold(true),
		Dialog: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(0, 2),
	}
}
