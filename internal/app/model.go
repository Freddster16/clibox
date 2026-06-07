package app

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type viewMode int

const (
	inboxView viewMode = iota
	readerView
)

type model struct {
	messages []message
	cursor   int
	mode     viewMode
	showHelp bool
	status   string
	theme    int
	width    int
	height   int
}

type styles struct {
	row          lipgloss.Style
	title        lipgloss.Style
	subtitle     lipgloss.Style
	panelTitle   lipgloss.Style
	selected     lipgloss.Style
	unread       lipgloss.Style
	muted        lipgloss.Style
	footer       lipgloss.Style
	helpPanel    lipgloss.Style
	readerHeader lipgloss.Style
	themeBadge   lipgloss.Style
}

type appTheme struct {
	name   string
	styles styles
}

type palette struct {
	accent       string
	accentText   string
	accentSoft   string
	background   string
	surface      string
	text         string
	muted        string
	selected     string
	selectedText string
	unread       string
	border       string
}

var appThemes = []appTheme{
	newTheme("Nocturne", palette{
		accent:       "39",
		accentText:   "230",
		accentSoft:   "219",
		background:   "234",
		surface:      "236",
		text:         "252",
		muted:        "245",
		selected:     "24",
		selectedText: "230",
		unread:       "159",
		border:       "63",
	}),
	newTheme("Copper", palette{
		accent:       "166",
		accentText:   "230",
		accentSoft:   "222",
		background:   "234",
		surface:      "235",
		text:         "252",
		muted:        "246",
		selected:     "94",
		selectedText: "230",
		unread:       "209",
		border:       "130",
	}),
	newTheme("Lagoon", palette{
		accent:       "37",
		accentText:   "230",
		accentSoft:   "121",
		background:   "234",
		surface:      "236",
		text:         "252",
		muted:        "245",
		selected:     "29",
		selectedText: "230",
		unread:       "123",
		border:       "74",
	}),
}

func newTheme(name string, p palette) appTheme {
	return appTheme{
		name: name,
		styles: styles{
			row: lipgloss.NewStyle().
				Foreground(lipgloss.Color(p.text)),
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
				Foreground(lipgloss.Color(p.unread)),
			muted: lipgloss.NewStyle().
				Foreground(lipgloss.Color(p.muted)),
			footer: lipgloss.NewStyle().
				Foreground(lipgloss.Color(p.text)).
				Background(lipgloss.Color(p.surface)).
				Padding(0, 1),
			helpPanel: lipgloss.NewStyle().
				Foreground(lipgloss.Color(p.text)).
				Background(lipgloss.Color(p.background)).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(p.border)).
				Padding(1, 2),
			readerHeader: lipgloss.NewStyle().
				Foreground(lipgloss.Color(p.text)).
				Background(lipgloss.Color(p.surface)).
				Padding(0, 1),
			themeBadge: lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(p.accentSoft)).
				Background(lipgloss.Color(p.surface)).
				Padding(0, 1),
		},
	}
}

func New() model {
	return model{
		messages: fakeMessages(),
		theme:    themeIndexFromEnv(os.Getenv("CLIBOX_THEME")),
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		key := msg.String()

		if m.showHelp {
			switch key {
			case "?", "esc", "q", "enter":
				m.showHelp = false
			}
			return m, nil
		}

		switch key {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if m.mode == readerView {
				m.mode = inboxView
				return m, nil
			}
			return m, tea.Quit
		case "?":
			m.showHelp = true
		case "up", "k":
			if m.mode == inboxView && m.cursor > 0 {
				m.cursor--
				m.status = ""
			}
		case "down", "j":
			if m.mode == inboxView && m.cursor < len(m.messages)-1 {
				m.cursor++
				m.status = ""
			}
		case "enter":
			if m.mode == inboxView && len(m.messages) > 0 {
				m.mode = readerView
				m.messages[m.cursor].Unread = false
				m.status = ""
			}
		case "b", "esc":
			if m.mode == readerView {
				m.mode = inboxView
				m.status = ""
			}
		case "r":
			if m.mode == readerView {
				return m.withStatus("reply will open $EDITOR in Phase 4"), nil
			}
		case "c":
			return m.withStatus("compose will open $EDITOR in Phase 4"), nil
		case "a":
			return m.withStatus("archive will connect to Himalaya in Phase 5"), nil
		case "d":
			return m.withStatus("delete confirmation arrives in Phase 5"), nil
		case "/":
			return m.withStatus("search arrives in Phase 5"), nil
		case "R":
			return m.withStatus("refresh arrives when the backend adapter exists"), nil
		case "t":
			m.theme = (m.theme + 1) % len(appThemes)
			return m.withStatus("theme switched to " + m.activeTheme().name), nil
		}
	}

	return m, nil
}

func (m model) View() string {
	if m.width == 0 {
		return "Starting clibox..."
	}

	content := m.renderCurrentView()
	if m.showHelp {
		content = m.overlayHelp(content)
	}

	return content
}

func (m model) renderCurrentView() string {
	header := m.renderHeader()
	footer := m.renderFooter()
	bodyHeight := max(1, m.height-lipgloss.Height(header)-lipgloss.Height(footer))

	var body string
	if m.mode == readerView {
		body = m.renderReader(bodyHeight)
	} else {
		body = m.renderInbox(bodyHeight)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m model) renderHeader() string {
	theme := m.activeTheme()
	styles := theme.styles
	account := styles.subtitle.Render("personal@example.com")
	count := styles.subtitle.Render(fmt.Sprintf("%d emails", len(m.messages)))
	title := styles.title.Render("clibox")
	right := count
	if m.width >= 72 {
		right = styles.themeBadge.Render(theme.name) + " " + count
	}
	left := title + " " + account
	gap := max(1, m.width-lipgloss.Width(left)-lipgloss.Width(right))
	return left + strings.Repeat(" ", gap) + right
}

func (m model) renderInbox(height int) string {
	styles := m.activeTheme().styles
	if m.width >= 96 {
		return m.renderWideInbox(height)
	}

	lines := []string{styles.panelTitle.Render("Inbox")}
	lines = append(lines, m.renderRows(m.width, height-3)...)
	lines = append(lines, "")
	lines = append(lines, styles.muted.Render("Enter opens the selected message. Press ? for help."))
	return fitHeight(strings.Join(lines, "\n"), height)
}

func (m model) renderWideInbox(height int) string {
	styles := m.activeTheme().styles
	railWidth := 18
	listWidth := min(52, max(38, m.width/2))
	readerWidth := max(24, m.width-railWidth-listWidth-4)

	rail := m.renderMailboxRail(railWidth, height)
	list := strings.Join(append(
		[]string{styles.panelTitle.Render("Inbox")},
		m.renderRows(listWidth, height-1)...,
	), "\n")
	preview := m.renderPreview(readerWidth, height)

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Width(railWidth).Render(rail),
		"  ",
		lipgloss.NewStyle().Width(listWidth).Render(list),
		"  ",
		lipgloss.NewStyle().Width(readerWidth).Render(preview),
	)
}

func (m model) renderMailboxRail(width, height int) string {
	styles := m.activeTheme().styles
	unread := 0
	for _, msg := range m.messages {
		if msg.Unread {
			unread++
		}
	}

	lines := []string{
		styles.panelTitle.Render("Mailboxes"),
		styles.selected.Width(width).Render(fmt.Sprintf("> INBOX %7d", len(m.messages))),
		styles.muted.Render(fmt.Sprintf("  Unread %6d", unread)),
		"  Archive",
		"  Sent",
		"  Drafts",
		"",
		styles.panelTitle.Render("Accounts"),
		"  personal",
	}
	return fitHeight(strings.Join(lines, "\n"), height)
}

func (m model) renderRows(width, height int) []string {
	styles := m.activeTheme().styles
	rows := make([]string, 0, len(m.messages))
	visible := max(1, height)
	start := scrollStart(m.cursor, visible, len(m.messages))
	end := min(len(m.messages), start+visible)

	for i := start; i < end; i++ {
		msg := m.messages[i]
		prefix := " "
		if i == m.cursor {
			prefix = ">"
		}
		unread := " "
		if msg.Unread {
			unread = "*"
		}

		fromWidth := 12
		dateWidth := 10
		subjectWidth := max(8, width-fromWidth-dateWidth-8)
		line := fmt.Sprintf(
			"%s %s %-*s %-*s %*s",
			prefix,
			unread,
			fromWidth,
			truncate(msg.From, fromWidth),
			subjectWidth,
			truncate(msg.Subject, subjectWidth),
			dateWidth,
			truncate(msg.Date, dateWidth),
		)

		style := styles.row
		if msg.Unread {
			style = styles.unread
		}
		if i == m.cursor {
			style = styles.selected.Width(width)
		}
		rows = append(rows, style.Render(truncate(line, width)))
	}

	return rows
}

func (m model) renderPreview(width, height int) string {
	styles := m.activeTheme().styles
	msg := m.selectedMessage()
	lines := []string{
		styles.panelTitle.Render("Reader"),
		styles.readerHeader.Width(width).Render("From: " + msg.From + " <" + msg.Email + ">"),
		styles.readerHeader.Width(width).Render("Subject: " + msg.Subject),
		styles.readerHeader.Width(width).Render("Date: " + msg.Date),
		"",
	}
	lines = append(lines, wrapText(msg.Preview+"\n\n"+msg.Body, width)...)
	return fitHeight(strings.Join(lines, "\n"), height)
}

func (m model) renderReader(height int) string {
	styles := m.activeTheme().styles
	msg := m.selectedMessage()
	width := max(32, m.width)
	lines := []string{
		styles.panelTitle.Render("Reader"),
		styles.readerHeader.Width(width).Render("From: " + msg.From + " <" + msg.Email + ">"),
		styles.readerHeader.Width(width).Render("Subject: " + msg.Subject),
		styles.readerHeader.Width(width).Render("Date: " + msg.Date),
		"",
	}
	lines = append(lines, wrapText(msg.Body, width-2)...)
	return fitHeight(strings.Join(lines, "\n"), height)
}

func (m model) renderFooter() string {
	styles := m.activeTheme().styles
	hints := "j/k move  enter read  r reply  c compose  a archive  / search  t theme  ? help  q quit"
	if m.mode == readerView {
		hints = "b back  r reply  a archive  d delete  t theme  ? help  q back"
	}
	if m.status != "" {
		hints = m.status + "  |  " + hints
	}
	return styles.footer.Width(m.width).Render(truncate(hints, max(1, m.width-2)))
}

func (m model) overlayHelp(content string) string {
	styles := m.activeTheme().styles
	help := strings.Join([]string{
		styles.panelTitle.Render("Help"),
		"",
		"j / k      move down / up",
		"Enter      open selected email",
		"b / Esc    back to inbox",
		"r          reply in $EDITOR (planned)",
		"c          compose in $EDITOR (planned)",
		"a          archive selected email (planned)",
		"d          delete selected email (planned)",
		"/          search current mailbox (planned)",
		"R          refresh inbox (planned)",
		"t          cycle color theme",
		"?          close this help",
		"q          quit or close current view",
	}, "\n")

	panel := styles.helpPanel.Width(min(58, max(30, m.width-8))).Render(help)
	topPad := max(0, (m.height-lipgloss.Height(panel))/3)
	leftPad := max(0, (m.width-lipgloss.Width(panel))/2)
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

func (m model) selectedMessage() message {
	if len(m.messages) == 0 {
		return message{}
	}
	return m.messages[min(m.cursor, len(m.messages)-1)]
}

func (m model) withStatus(text string) model {
	m.status = text
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

func themeIndexFromEnv(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	for i, theme := range appThemes {
		if strings.EqualFold(theme.name, value) {
			return i
		}
	}
	return 0
}

func scrollStart(cursor, visible, total int) int {
	if total <= visible {
		return 0
	}
	half := visible / 2
	start := cursor - half
	if start < 0 {
		return 0
	}
	if start+visible > total {
		return total - visible
	}
	return start
}

func wrapText(text string, width int) []string {
	width = max(16, width)
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		line := words[0]
		for _, word := range words[1:] {
			if lipgloss.Width(line)+1+lipgloss.Width(word) > width {
				lines = append(lines, line)
				line = word
				continue
			}
			line += " " + word
		}
		lines = append(lines, line)
	}
	return lines
}

func fitHeight(value string, height int) string {
	lines := strings.Split(value, "\n")
	if len(lines) > height {
		return strings.Join(lines[:height], "\n")
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	if width <= 3 {
		runes := []rune(value)
		return string(runes[:min(width, len(runes))])
	}

	limit := max(0, width-3)
	var out strings.Builder
	for _, r := range value {
		next := out.String() + string(r)
		if lipgloss.Width(next) > limit {
			break
		}
		out.WriteRune(r)
	}
	return out.String() + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
