package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type styles struct {
	screen       lipgloss.Style
	header       lipgloss.Style
	row          lipgloss.Style
	rowAlt       lipgloss.Style
	title        lipgloss.Style
	subtitle     lipgloss.Style
	panelTitle   lipgloss.Style
	selected     lipgloss.Style
	unread       lipgloss.Style
	muted        lipgloss.Style
	footer       lipgloss.Style
	helpPanel    lipgloss.Style
	readerHeader lipgloss.Style
	readerBody   lipgloss.Style
	themeBadge   lipgloss.Style
}

type appTheme struct {
	name        string
	description string
	palette     palette
	styles      styles
}

type palette struct {
	background     string
	header         string
	surface        string
	surfaceAlt     string
	accent         string
	accentText     string
	accentSoft     string
	text           string
	muted          string
	selected       string
	selectedText   string
	unread         string
	border         string
	footer         string
	footerText     string
	readerHeader   string
	readerHeaderFg string
}

var appThemes = []appTheme{
	newTheme("Nocturne", "violet header, cyan accents, dark blue surfaces", palette{
		background:     "#09090f",
		header:         "#312e81",
		surface:        "#111827",
		surfaceAlt:     "#0f172a",
		accent:         "#d946ef",
		accentText:     "#fdf4ff",
		accentSoft:     "#67e8f9",
		text:           "#f8fafc",
		muted:          "#94a3b8",
		selected:       "#7c3aed",
		selectedText:   "#ffffff",
		unread:         "#a7f3d0",
		border:         "#22d3ee",
		footer:         "#1e1b4b",
		footerText:     "#e0e7ff",
		readerHeader:   "#3730a3",
		readerHeaderFg: "#eef2ff",
	}),
	newTheme("Ember", "orange header, gold accents, warm dark surfaces", palette{
		background:     "#120b07",
		header:         "#7c2d12",
		surface:        "#26150d",
		surfaceAlt:     "#321a0f",
		accent:         "#fb923c",
		accentText:     "#1c0a00",
		accentSoft:     "#facc15",
		text:           "#fff7ed",
		muted:          "#d6a57e",
		selected:       "#f97316",
		selectedText:   "#1c0a00",
		unread:         "#fdba74",
		border:         "#ea580c",
		footer:         "#431407",
		footerText:     "#ffedd5",
		readerHeader:   "#9a3412",
		readerHeaderFg: "#fff7ed",
	}),
	newTheme("Lagoon", "teal header, seafoam accents, deep green surfaces", palette{
		background:     "#031b1f",
		header:         "#155e75",
		surface:        "#07343a",
		surfaceAlt:     "#092d33",
		accent:         "#2dd4bf",
		accentText:     "#042f2e",
		accentSoft:     "#a7f3d0",
		text:           "#ecfeff",
		muted:          "#8fc8d1",
		selected:       "#14b8a6",
		selectedText:   "#042f2e",
		unread:         "#5eead4",
		border:         "#06b6d4",
		footer:         "#083344",
		footerText:     "#cffafe",
		readerHeader:   "#0f766e",
		readerHeaderFg: "#ecfeff",
	}),
}

func newTheme(name, description string, p palette) appTheme {
	return appTheme{
		name:        name,
		description: description,
		palette:     p,
		styles: styles{
			screen: lipgloss.NewStyle().
				Foreground(lipgloss.Color(p.text)).
				Background(lipgloss.Color(p.background)),
			header: lipgloss.NewStyle().
				Foreground(lipgloss.Color(p.text)).
				Background(lipgloss.Color(p.header)),
			row: lipgloss.NewStyle().
				Foreground(lipgloss.Color(p.text)).
				Background(lipgloss.Color(p.surface)),
			rowAlt: lipgloss.NewStyle().
				Foreground(lipgloss.Color(p.text)).
				Background(lipgloss.Color(p.surfaceAlt)),
			title: lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(p.accentText)).
				Background(lipgloss.Color(p.accent)).
				Padding(0, 1),
			subtitle: lipgloss.NewStyle().
				Foreground(lipgloss.Color(p.muted)),
			panelTitle: lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(p.accentSoft)),
			selected: lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(p.selectedText)).
				Background(lipgloss.Color(p.selected)),
			unread: lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(p.unread)).
				Background(lipgloss.Color(p.surfaceAlt)),
			muted: lipgloss.NewStyle().
				Foreground(lipgloss.Color(p.muted)),
			footer: lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(p.footerText)).
				Background(lipgloss.Color(p.footer)).
				Padding(0, 1),
			helpPanel: lipgloss.NewStyle().
				Foreground(lipgloss.Color(p.text)).
				Background(lipgloss.Color(p.surface)).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(p.border)).
				Padding(1, 2),
			readerHeader: lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(p.readerHeaderFg)).
				Background(lipgloss.Color(p.readerHeader)).
				Padding(0, 1),
			readerBody: lipgloss.NewStyle().
				Foreground(lipgloss.Color(p.text)).
				Background(lipgloss.Color(p.surface)),
			themeBadge: lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(p.accentText)).
				Background(lipgloss.Color(p.accent)).
				Padding(0, 1),
		},
	}
}

func ThemeHelp() string {
	var lines []string
	for _, theme := range appThemes {
		lines = append(lines, fmt.Sprintf("  %-9s %s", strings.ToLower(theme.name), theme.description))
	}

	return fmt.Sprintf(`Available clibox themes:
%s

Start with a theme:
  clibox --theme lagoon

Inside clibox:
  press t to open the theme picker
`, strings.Join(lines, "\n"))
}

func (m model) previewTheme(index int) model {
	if len(appThemes) == 0 {
		return m
	}
	index %= len(appThemes)
	if index < 0 {
		index += len(appThemes)
	}
	m.themeCursor = index
	m.theme = index
	return m
}

func (m model) activeTheme() appTheme {
	if len(appThemes) == 0 {
		return appTheme{name: "Default"}
	}
	index := m.theme % len(appThemes)
	if index < 0 {
		index += len(appThemes)
	}
	return appThemes[index]
}

func themeIndex(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, true
	}
	if strings.EqualFold(value, "copper") {
		value = "Ember"
	}
	for i, theme := range appThemes {
		if strings.EqualFold(theme.name, value) {
			return i, true
		}
	}
	return 0, false
}

func renderThemeSwatches(theme appTheme, width int) string {
	swatches := []string{
		theme.styles.header.Render(" header "),
		theme.styles.themeBadge.Render(" accent "),
		theme.styles.selected.Render(" selected "),
		theme.styles.unread.Render(" unread "),
	}

	out := swatches[0]
	for _, swatch := range swatches[1:] {
		next := out + " " + swatch
		if lipgloss.Width(next) > width {
			break
		}
		out = next
	}
	return out
}
