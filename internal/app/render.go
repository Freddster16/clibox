package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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
	if m.mode == setupView {
		body = m.renderSetup(bodyHeight)
	} else if m.mode == draftReviewView {
		body = m.renderDraftReview(bodyHeight)
	} else if m.mode == readerView {
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
	countLabel := fmt.Sprintf("%d emails", len(m.messages))
	if strings.TrimSpace(m.searchQuery) != "" {
		countLabel = fmt.Sprintf("%d results", len(m.messages))
	} else if m.mailboxFilter == unreadMailFilter {
		countLabel = fmt.Sprintf("%d unread", len(m.messages))
	}
	count := styles.subtitle.Render(countLabel)
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
	if m.mailboxFocused {
		return m.renderMailboxRail(max(32, m.width), height)
	}

	lines := []string{styles.panelTitle.Render(m.inboxTitle())}
	lines = append(lines, m.renderRows(m.width, height-3)...)
	lines = append(lines, "")
	lines = append(lines, styles.muted.Render(fmt.Sprintf("Theme %s. Press t to choose, ? for help.", m.activeTheme().name)))
	return fitHeight(strings.Join(lines, "\n"), height)
}

func (m model) renderSetup(height int) string {
	styles := m.activeTheme().styles
	width := max(32, m.width)
	if m.configuring {
		lines := []string{
			styles.panelTitle.Render("Email setup"),
			"",
			styles.readerBody.Width(width).Render("clibox is saving your account settings and password to your OS credential store."),
			styles.readerBody.Width(width).Render("Your inbox will reload automatically when setup finishes."),
		}
		return fitHeight(strings.Join(lines, "\n"), height)
	}

	switch m.setupStep {
	case setupReviewStep:
		return m.renderSetupReview(width, height)
	case setupSecretStep:
		return m.renderSetupSecret(width, height)
	case setupAccountStep:
		return m.renderSetupAccount(width, height)
	default:
		return m.renderSetupEmail(width, height)
	}
}

func (m model) renderSetupEmail(width, height int) string {
	styles := m.activeTheme().styles
	email := m.setupEmail + "_"
	lines := []string{
		styles.panelTitle.Render("Add email account"),
		"",
		styles.readerBody.Width(width).Render("Start with your email address. clibox will detect the provider, choose the mail servers, and set up your account without sending you through another wizard."),
		"",
		styles.readerHeader.Width(width).Render("Email address"),
		styles.selected.Width(min(width, max(30, lipgloss.Width(email)+2))).Render(" " + email),
		"",
		styles.readerBody.Width(width).Render("Examples: you@gmail.com, you@icloud.com, work@company.com"),
		"",
		styles.readerBody.Width(width).Render("Enter continues. q quits."),
	}
	return fitHeight(strings.Join(lines, "\n"), height)
}

func (m model) renderSetupReview(width, height int) string {
	styles := m.activeTheme().styles
	provider := m.setupProvider
	if provider.Name == "" {
		provider = detectProvider(m.setupEmail)
	}

	lines := []string{
		styles.panelTitle.Render("Review account setup"),
		"",
		styles.readerHeader.Width(width).Render("Email: " + m.setupEmail),
		styles.readerHeader.Width(width).Render("Provider: " + provider.Name),
		styles.readerHeader.Width(width).Render("Account name: " + m.setupAccount),
		"",
	}
	lines = append(lines, styledLines(wrapText(provider.AuthSummary, width-2), styles.readerBody, width)...)
	if provider.ManualWarning != "" {
		lines = append(lines, "")
		lines = append(lines, styledLines(wrapText(provider.ManualWarning, width-2), styles.unread, width)...)
	}
	lines = append(lines, "")
	for _, instruction := range provider.Instructions {
		lines = append(lines, styledLines(wrapText("- "+instruction, width-2), styles.readerBody, width)...)
	}
	lines = append(lines, "")
	if provider.HelpURL != "" {
		lines = append(lines,
			styles.readerHeader.Width(width).Render(provider.HelpLabel),
			styles.readerBody.Width(width).Render(provider.HelpURL),
			styles.readerBody.Width(width).Render("Click the link if your terminal supports it, or press o to open it."),
		)
	}
	if _, ok := m.backend.(oauthAccountSetupBackend); ok && providerNeedsOAuth(provider) {
		lines = append(lines, styles.readerBody.Width(width).Render("Enter opens browser login. e edits email. n edits account name."))
	} else if provider.canAutoConfigure() {
		lines = append(lines, styles.readerBody.Width(width).Render("Enter continues to password setup. e edits email. n edits account name."))
	} else {
		lines = append(lines, styles.readerBody.Width(width).Render("Automatic setup for this provider needs manual server settings first. e edits email. n edits account name."))
	}
	return fitHeight(strings.Join(lines, "\n"), height)
}

func (m model) renderSetupSecret(width, height int) string {
	styles := m.activeTheme().styles
	provider := m.setupProvider
	if provider.Name == "" {
		provider = detectProvider(m.setupEmail)
	}
	mask := strings.Repeat("*", len([]rune(m.setupSecret)))
	if mask == "" {
		mask = "_"
	}

	lines := []string{
		styles.panelTitle.Render("Connect " + provider.Name),
		"",
		styles.readerHeader.Width(width).Render("Email: " + m.setupEmail),
		styles.readerHeader.Width(width).Render("Account name: " + m.setupAccount),
		"",
	}
	prompt := "Paste your " + provider.secretLabel() + ". clibox will save it to your OS credential store, configure your mail connection, and reload your inbox."
	if provider.Name == "Gmail" {
		prompt = "Paste the 16-character Google app password, not your Gmail address or normal Google password. clibox will save it to your OS credential store, configure your mail connection, and reload your inbox."
	}
	lines = append(lines, styledLines(wrapText(prompt, width-2), styles.readerBody, width)...)
	if provider.HelpURL != "" {
		lines = append(lines, "")
		lines = append(lines,
			styles.readerHeader.Width(width).Render(provider.HelpLabel),
			styles.readerBody.Width(width).Render(provider.HelpURL),
			styles.readerBody.Width(width).Render("Click the link if your terminal supports it, or press Ctrl+O to open it. Esc returns."),
		)
	}
	lines = append(lines,
		"",
		styles.readerHeader.Width(width).Render(provider.secretLabel()),
		styles.selected.Width(min(width, max(30, lipgloss.Width(mask)+2))).Render(" "+mask),
		"",
		styles.readerBody.Width(width).Render("Enter saves setup. Ctrl+O opens help. Ctrl+U clears. Esc returns."),
	)
	return fitHeight(strings.Join(lines, "\n"), height)
}

func (m model) renderSetupAccount(width, height int) string {
	styles := m.activeTheme().styles
	account := m.setupAccount + "_"
	lines := []string{
		styles.panelTitle.Render("Account name"),
		"",
		styles.readerBody.Width(width).Render("This is a short local name for the account inside clibox."),
		"",
		styles.readerHeader.Width(width).Render("Account name"),
		styles.selected.Width(min(width, max(24, lipgloss.Width(account)+2))).Render(" " + account),
		"",
		styles.readerBody.Width(width).Render("Examples: personal, work, gmail"),
		"",
		styles.readerBody.Width(width).Render("Enter returns to review. Esc returns without changing screens."),
	}
	return fitHeight(strings.Join(lines, "\n"), height)
}

func (m model) renderWideInbox(height int) string {
	styles := m.activeTheme().styles
	railWidth := 18
	listWidth := min(52, max(38, m.width/2))
	readerWidth := max(24, m.width-railWidth-listWidth-4)

	rail := m.renderMailboxRail(railWidth, height)
	list := strings.Join(append(
		[]string{styles.panelTitle.Render(m.inboxTitle())},
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
	title := "Mailboxes"
	if m.mailboxFocused {
		title = "Mailboxes *"
	}
	lines := []string{styles.panelTitle.Render(title)}
	for i, entry := range m.mailboxEntries() {
		cursor := " "
		if m.mailboxFocused && i == m.mailboxCursor {
			cursor = ">"
		}
		active := " "
		if m.mailboxEntryActive(entry.Mailbox, entry.Filter) {
			active = "*"
		}
		count := ""
		if entry.Count != "" {
			count = fmt.Sprintf("%4s", truncate(entry.Count, 4))
		}
		labelWidth := max(5, width-7)
		line := fmt.Sprintf("%s%s %-*s%s", cursor, active, labelWidth, truncate(entry.Label, labelWidth), count)
		style := styles.row
		if i%2 == 1 {
			style = styles.rowAlt
		}
		if m.mailboxFocused && i == m.mailboxCursor {
			style = styles.selected
		} else if m.mailboxEntryActive(entry.Mailbox, entry.Filter) {
			style = styles.unread
		}
		lines = append(lines, style.Width(width).Render(truncate(line, width)))
	}
	lines = append(lines,
		styles.screen.Width(width).Render(""),
		styles.panelTitle.Render("Theme"),
		styles.themeBadge.Width(width).Render(m.activeTheme().name),
		styles.screen.Width(width).Render(""),
		styles.panelTitle.Render("Accounts"),
		styles.row.Width(width).Render("  "+truncate(m.accountLabel(), max(1, width-2))),
	)
	return fitHeight(strings.Join(lines, "\n"), height)
}

func (m model) renderRows(width, height int) []string {
	styles := m.activeTheme().styles
	if m.loading && len(m.messages) == 0 {
		return []string{styles.row.Width(width).Render(truncate("  Loading "+m.scopeLabel()+"...", width))}
	}
	if len(m.messages) == 0 {
		if m.mailboxFilter == unreadMailFilter {
			return []string{styles.row.Width(width).Render(truncate("  No unread emails. Press Esc for all mail.", width))}
		}
		if strings.TrimSpace(m.searchQuery) != "" {
			return []string{styles.row.Width(width).Render(truncate("  No search results. Press / to search again or Esc to clear.", width))}
		}
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
	if m.loadingMore && end == len(m.messages) && len(rows) < visible {
		rows = append(rows, styles.muted.Width(width).Render(truncate("  Loading older mail...", width)))
	}

	return rows
}

func (m model) renderPreview(width, height int) string {
	return m.renderMessage(width, height, true)
}

func (m model) renderReader(height int) string {
	return m.renderMessage(max(32, m.width), height, false)
}

func (m model) renderDraftReview(height int) string {
	styles := m.activeTheme().styles
	width := max(32, m.width)
	summary := m.draft.Summary
	subject := firstNonEmpty(strings.TrimSpace(summary.Subject), "(no subject)")

	lines := []string{
		styles.panelTitle.Render(m.draft.Kind.title()),
		"",
		styles.readerHeader.Width(width).Render("From: " + terminalSafeLine(firstNonEmpty(summary.From, "(backend default)"))),
		styles.readerHeader.Width(width).Render("To: " + terminalSafeLine(firstNonEmpty(summary.To, "(missing recipient)"))),
	}
	if strings.TrimSpace(summary.Cc) != "" {
		lines = append(lines, styles.readerHeader.Width(width).Render("Cc: "+terminalSafeLine(summary.Cc)))
	}
	if strings.TrimSpace(summary.Bcc) != "" {
		lines = append(lines, styles.readerHeader.Width(width).Render("Bcc: "+terminalSafeLine(summary.Bcc)))
	}
	lines = append(lines,
		styles.readerHeader.Width(width).Render("Subject: "+terminalSafeLine(subject)),
		"",
	)

	if m.draft.Sending {
		lines = append(lines, styles.readerBody.Width(width).Render("Sending email..."))
	} else if err := validateDraftForSend(summary); err != nil {
		lines = append(lines, styles.unread.Width(width).Render(err.Error()), "")
	}

	body := firstNonEmpty(summary.Body, "(empty body)")
	bodyLines := wrapText(body, width-2)
	available := max(1, height-len(lines)-1)
	if len(bodyLines) > available {
		bodyLines = append(bodyLines[:available], "...")
	}
	lines = append(lines, styledLines(bodyLines, styles.readerBody, width)...)
	return fitHeight(strings.Join(lines, "\n"), height)
}

func (m model) renderMessage(width, height int, includePreview bool) string {
	styles := m.activeTheme().styles
	if len(m.messages) == 0 {
		return fitHeight(strings.Join([]string{
			styles.panelTitle.Render("Reader"),
			styles.readerBody.Width(width).Render("No message selected."),
			styles.readerBody.Width(width).Render("Finish account setup, then press R to load your inbox."),
		}, "\n"), height)
	}

	msg := m.selectedMessage()
	lines := []string{
		styles.panelTitle.Render("Reader"),
		styles.readerHeader.Width(width).Render("From: " + terminalSafeLine(msg.From) + " <" + terminalSafeLine(msg.Email) + ">"),
		styles.readerHeader.Width(width).Render("Subject: " + terminalSafeLine(msg.Subject)),
		styles.readerHeader.Width(width).Render("Date: " + terminalSafeLine(msg.Date)),
		styles.readerBody.Width(width).Render(""),
	}

	bodyLines := wrapText(m.messageBodyText(msg, includePreview), width-2)
	if !includePreview {
		bodyHeight := max(1, height-len(lines))
		offset := min(m.readerOffset, max(0, len(bodyLines)-bodyHeight))
		end := min(len(bodyLines), offset+bodyHeight)
		if offset > 0 || end < len(bodyLines) {
			lines[0] = styles.panelTitle.Render(fmt.Sprintf("Reader  %d-%d/%d", min(offset+1, len(bodyLines)), end, len(bodyLines)))
		}
		bodyLines = bodyLines[offset:end]
	}
	lines = append(lines, styledLines(bodyLines, styles.readerBody, width)...)
	return fitHeight(strings.Join(lines, "\n"), height)
}

func (m model) renderFooter() string {
	styles := m.activeTheme().styles
	themeHint := fmt.Sprintf("theme %s: t themes", m.activeTheme().name)
	hints := themeHint + "  |  tab mailboxes  j/k move  enter read  R refresh  A account  r reply  c compose  a archive  / search  ? help  q quit"
	if m.mode == readerView {
		hints = themeHint + "  |  j/k scroll  b back  r reply  a archive  d delete  ? help  q back"
	} else if m.mode == setupView {
		hints = m.setupFooterHints()
	} else if m.mode == draftReviewView {
		hints = "s send  e edit  b discard  ? help"
		if m.draft.Sending {
			hints = "sending email..."
		}
	} else if m.mailboxFocused {
		hints = "mailboxes  |  j/k choose  enter open  tab/right messages  R refresh  ? help  q quit"
	} else if m.searching {
		hints = "search: " + m.searchInput + "_  |  enter apply  esc cancel  ctrl+u clear"
	} else if m.confirmDelete {
		hints = "Move selected email to Trash?  y confirm  n cancel"
	} else if m.action.Running {
		hints = m.action.Kind.presentParticiple() + " email..."
	} else if strings.TrimSpace(m.searchQuery) != "" && m.mode == inboxView {
		hints = themeHint + "  |  tab mailboxes  / search again  esc clear search  j/k move  enter read  a archive  d delete  R refresh  ? help"
	} else if m.mailboxFilter == unreadMailFilter && m.mode == inboxView {
		hints = themeHint + "  |  tab mailboxes  esc all mail  j/k move  enter read  / search  R refresh  ? help"
	}
	if m.status != "" {
		hints = m.status + "  |  " + hints
	}
	return styles.footer.Width(m.width).Render(truncate(hints, max(1, m.width-2)))
}

func (m model) setupFooterHints() string {
	if m.configuring {
		return "finish account setup"
	}
	switch m.setupStep {
	case setupReviewStep:
		provider := m.setupProvider
		if provider.Name == "" {
			provider = detectProvider(m.setupEmail)
		}
		if _, ok := m.backend.(oauthAccountSetupBackend); ok && providerNeedsOAuth(provider) {
			return "enter browser login  o provider page  e edit email  n edit account name  q back"
		}
		return "o open provider page  enter setup  e edit email  n edit account name  q back"
	case setupAccountStep:
		return "type account name  enter review  backspace edit  q back"
	default:
		return "type email address  enter continue  backspace edit  q quit"
	}
}

func (m model) overlayHelp(content string) string {
	styles := m.activeTheme().styles
	help := strings.Join([]string{
		styles.panelTitle.Render("Help"),
		"",
		"Theme      " + m.activeTheme().name,
		"",
		"Tab        focus mailboxes",
		"Enter      open selected mailbox/filter",
		"j / k      move in inbox",
		"Enter      open selected email",
		"j / k      scroll in reader",
		"PgUp/PgDn  jump in reader",
		"b / Esc    back to inbox",
		"r          reply in $EDITOR",
		"c          compose in $EDITOR",
		"s          send reviewed draft",
		"e          edit reviewed draft",
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
