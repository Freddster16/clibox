package app

import (
	"context"
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

type Options struct {
	Theme    string
	Account  string
	Mailbox  string
	Himalaya string
	PageSize int
	backend  inboxBackend
}

type inboxLoadedMsg struct {
	messages []message
	err      error
}

type model struct {
	messages          []message
	backend           inboxBackend
	cursor            int
	mode              viewMode
	showHelp          bool
	showThemes        bool
	loading           bool
	status            string
	account           string
	mailbox           string
	theme             int
	themeCursor       int
	themeBeforePicker int
	width             int
	height            int
}

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

func New() model {
	return NewWithOptions(Options{})
}

func NewWithTheme(name string) model {
	return NewWithOptions(Options{Theme: name})
}

func NewWithOptions(options Options) model {
	selected := strings.TrimSpace(options.Theme)
	if selected == "" {
		selected = os.Getenv("CLIBOX_THEME")
	}

	index, ok := themeIndex(selected)
	status := fmt.Sprintf("theme %s active; press t to choose", appThemes[index].name)
	if strings.TrimSpace(selected) != "" && !ok {
		status = fmt.Sprintf("unknown theme %q; using %s", selected, appThemes[index].name)
	}

	backend := options.backend
	if backend == nil {
		himalaya := newHimalayaBackend(options)
		backend = himalaya
		if options.Mailbox == "" {
			options.Mailbox = himalaya.mailbox
		}
	}

	return model{
		backend:           backend,
		loading:           true,
		status:            status,
		account:           strings.TrimSpace(options.Account),
		mailbox:           strings.TrimSpace(options.Mailbox),
		theme:             index,
		themeCursor:       index,
		themeBeforePicker: index,
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

func (m model) Init() tea.Cmd {
	return m.loadInbox()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case inboxLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.status = msg.err.Error()
			m.messages = nil
			m.cursor = 0
			return m, nil
		}
		m.messages = msg.messages
		if len(m.messages) == 0 {
			m.cursor = 0
			return m.withStatus("Himalaya returned no messages for " + m.mailboxLabel()), nil
		}
		if m.cursor >= len(m.messages) {
			m.cursor = len(m.messages) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		return m.withStatus(fmt.Sprintf("loaded %d emails from %s", len(m.messages), m.mailboxLabel())), nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		key := msg.String()

		if m.showThemes {
			switch key {
			case "ctrl+c":
				return m, tea.Quit
			case "up", "k":
				return m.previewTheme(m.themeCursor - 1), nil
			case "down", "j":
				return m.previewTheme(m.themeCursor + 1), nil
			case "enter", "t":
				m.theme = m.themeCursor
				m.themeBeforePicker = m.theme
				m.showThemes = false
				return m.withStatus("theme " + m.activeTheme().name + " applied"), nil
			case "esc", "q":
				m.theme = m.themeBeforePicker
				m.themeCursor = m.theme
				m.showThemes = false
				return m.withStatus("theme " + m.activeTheme().name + " kept"), nil
			case "1", "2", "3":
				index := int([]rune(key)[0] - '1')
				if index >= 0 && index < len(appThemes) {
					m.themeCursor = index
					m.theme = index
					m.themeBeforePicker = index
					m.showThemes = false
					return m.withStatus("theme " + m.activeTheme().name + " applied"), nil
				}
			}
			return m, nil
		}

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
			m.loading = true
			m.status = "refreshing " + m.mailboxLabel() + " from Himalaya..."
			return m, m.loadInbox()
		case "t":
			m.showThemes = true
			m.themeCursor = m.theme
			m.themeBeforePicker = m.theme
			m.status = ""
		}
	}

	return m, nil
}

func (m model) loadInbox() tea.Cmd {
	if m.backend == nil {
		return nil
	}

	backend := m.backend
	return func() tea.Msg {
		messages, err := backend.ListEnvelopes(context.Background())
		return inboxLoadedMsg{messages: messages, err: err}
	}
}

func (m model) View() string {
	if m.width == 0 {
		return "Starting clibox..."
	}

	content := m.renderCurrentView()
	if m.showHelp {
		content = m.overlayHelp(content)
	}
	if m.showThemes {
		content = m.overlayThemes(content)
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

	content := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
	return m.activeTheme().styles.screen.Width(m.width).Height(max(1, m.height)).Render(content)
}

func (m model) renderHeader() string {
	theme := m.activeTheme()
	styles := theme.styles
	if m.width < 46 {
		return styles.header.Width(m.width).Render(truncate(fmt.Sprintf("clibox theme:%s", theme.name), m.width))
	}

	account := styles.subtitle.Render(m.accountLabel())
	count := styles.subtitle.Render(fmt.Sprintf("%d emails", len(m.messages)))
	title := styles.title.Render("clibox")
	badge := styles.themeBadge.Render("theme " + theme.name)
	left := title + " " + badge
	right := count
	if m.width >= 78 {
		left += " " + account
	}
	gap := max(1, m.width-lipgloss.Width(left)-lipgloss.Width(right))
	return styles.header.Width(m.width).Render(left + strings.Repeat(" ", gap) + right)
}

func (m model) renderInbox(height int) string {
	styles := m.activeTheme().styles
	if m.width >= 96 {
		return m.renderWideInbox(height)
	}

	lines := []string{styles.panelTitle.Render("Inbox")}
	lines = append(lines, m.renderRows(m.width, height-3)...)
	lines = append(lines, "")
	lines = append(lines, styles.muted.Render(fmt.Sprintf("Theme %s. Press t to choose, ? for help.", m.activeTheme().name)))
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
		styles.selected.Width(width).Render(fmt.Sprintf("> %-8s %4d", truncate(m.mailboxLabel(), 8), len(m.messages))),
		styles.rowAlt.Width(width).Render(fmt.Sprintf("  Unread %6d", unread)),
		styles.row.Width(width).Render("  Archive"),
		styles.rowAlt.Width(width).Render("  Sent"),
		styles.row.Width(width).Render("  Drafts"),
		styles.screen.Width(width).Render(""),
		styles.panelTitle.Render("Theme"),
		styles.themeBadge.Width(width).Render(m.activeTheme().name),
		styles.screen.Width(width).Render(""),
		styles.panelTitle.Render("Accounts"),
		styles.row.Width(width).Render("  " + truncate(m.accountLabel(), max(1, width-2))),
	}
	return fitHeight(strings.Join(lines, "\n"), height)
}

func (m model) renderRows(width, height int) []string {
	styles := m.activeTheme().styles
	if m.loading {
		return []string{styles.row.Width(width).Render(truncate("  Loading inbox from Himalaya...", width))}
	}
	if len(m.messages) == 0 {
		return []string{styles.row.Width(width).Render(truncate("  No messages loaded. Press R to retry.", width))}
	}

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
		if i%2 == 1 {
			style = styles.rowAlt
		}
		if msg.Unread {
			style = styles.unread
		}
		if i == m.cursor {
			style = styles.selected.Width(width)
		}
		rows = append(rows, style.Width(width).Render(truncate(line, width)))
	}

	return rows
}

func (m model) renderPreview(width, height int) string {
	return m.renderMessage(width, height, true)
}

func (m model) renderReader(height int) string {
	return m.renderMessage(max(32, m.width), height, false)
}

func (m model) renderMessage(width, height int, includePreview bool) string {
	styles := m.activeTheme().styles
	if len(m.messages) == 0 {
		return fitHeight(strings.Join([]string{
			styles.panelTitle.Render("Reader"),
			styles.readerBody.Width(width).Render("No message selected."),
			styles.readerBody.Width(width).Render("Configure Himalaya, then press R to load your inbox."),
		}, "\n"), height)
	}

	msg := m.selectedMessage()
	body := msg.Body
	if includePreview && strings.TrimSpace(msg.Preview) != "" {
		body = msg.Preview + "\n\n" + body
	}
	lines := []string{
		styles.panelTitle.Render("Reader"),
		styles.readerHeader.Width(width).Render("From: " + msg.From + " <" + msg.Email + ">"),
		styles.readerHeader.Width(width).Render("Subject: " + msg.Subject),
		styles.readerHeader.Width(width).Render("Date: " + msg.Date),
		styles.readerBody.Width(width).Render(""),
	}
	lines = append(lines, styledLines(wrapText(body, width-2), styles.readerBody, width)...)
	return fitHeight(strings.Join(lines, "\n"), height)
}

func (m model) renderFooter() string {
	styles := m.activeTheme().styles
	themeHint := fmt.Sprintf("theme %s: t themes", m.activeTheme().name)
	hints := themeHint + "  |  j/k move  enter read  R refresh  r reply  c compose  a archive  / search  ? help  q quit"
	if m.mode == readerView {
		hints = themeHint + "  |  b back  r reply  a archive  d delete  ? help  q back"
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
		"Theme      " + m.activeTheme().name,
		"",
		"j / k      move down / up",
		"Enter      open selected email",
		"b / Esc    back to inbox",
		"r          reply in $EDITOR (planned)",
		"c          compose in $EDITOR (planned)",
		"a          archive selected email (planned)",
		"d          delete selected email (planned)",
		"/          search current mailbox (planned)",
		"R          refresh inbox from Himalaya",
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

func (m model) accountLabel() string {
	if strings.TrimSpace(m.account) != "" {
		return m.account
	}
	return "default account"
}

func (m model) mailboxLabel() string {
	if strings.TrimSpace(m.mailbox) != "" {
		return m.mailbox
	}
	return "INBOX"
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

func styledLines(lines []string, style lipgloss.Style, width int) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, style.Width(width).Render(truncate(line, width)))
	}
	return out
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
