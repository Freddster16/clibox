package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m model) overlayHelp(content string) string {
	styles := m.activeTheme().styles
	help := strings.Join([]string{
		styles.panelTitle.Render("Help"),
		"",
		"Theme      " + m.activeTheme().name,
		"",
		"Tab        focus mailboxes",
		"Tab        next mailbox when mailboxes are focused",
		"Enter      open selected mailbox/filter",
		"j / k      move in inbox; at bottom j loads older mail",
		"Enter      open selected email in full reader",
		"j / k      scroll in reader",
		"PgUp/PgDn  jump in reader",
		"b / Esc    back to inbox",
		"r          reply in clibox",
		"c          compose in clibox",
		"Tab        move between draft fields",
		"Ctrl+S     send draft",
		"Ctrl+O     open draft in external editor",
		"a          archive selected email",
		"d          delete selected email with confirmation",
		"/          search current mailbox",
		"Esc        clear active search/filter in inbox",
		"R          refresh inbox",
		"A          add or update an email account",
		"t          open theme picker",
		"?          close this help",
		"q          quit or close current view",
	}, "\n")

	panel := styles.helpPanel.Width(min(58, max(30, m.width-8))).Render(help)
	return placeOverlay(content, panel, m.width, m.height)
}

func (m model) overlayThemes(content string) string {
	styles := m.activeTheme().styles
	panelWidth := min(72, max(38, m.width-8))
	lines := []string{
		styles.panelTitle.Render("Themes"),
		"",
		styles.muted.Render("j/k previews colors. Enter applies. Esc cancels."),
		"",
	}

	for i, theme := range appThemes {
		prefix := " "
		if i == m.themeCursor {
			prefix = ">"
		}
		active := " "
		if i == m.themeBeforePicker {
			active = "*"
		}
		label := fmt.Sprintf("%s %s %d. %-9s %s", prefix, active, i+1, theme.name, theme.description)
		style := styles.row
		if i%2 == 1 {
			style = styles.rowAlt
		}
		if i == m.themeCursor {
			style = theme.styles.selected
		}
		lines = append(lines, style.Width(panelWidth).Render(truncate(label, panelWidth)))
		lines = append(lines, "  "+renderThemeSwatches(theme, max(10, panelWidth-2)))
	}

	lines = append(lines,
		"",
		styles.muted.Render("* original theme.  1-3 jumps directly."),
	)

	panel := styles.helpPanel.Width(panelWidth).Render(strings.Join(lines, "\n"))
	return placeOverlay(content, panel, m.width, m.height)
}

func placeOverlay(content, panel string, width, height int) string {
	topPad := max(0, (height-lipgloss.Height(panel))/3)
	leftPad := max(0, (width-lipgloss.Width(panel))/2)
	overlay := strings.Repeat("\n", topPad) + lipgloss.NewStyle().MarginLeft(leftPad).Render(panel)

	contentLines := strings.Split(content, "\n")
	overlayLines := strings.Split(overlay, "\n")
	for i, line := range overlayLines {
		if i >= len(contentLines) {
			break
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		contentLines[i] = line
	}

	return strings.Join(contentLines, "\n")
}
