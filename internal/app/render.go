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
	content = m.activeTheme().styles.screen.Render(content)
	return fitFrame(content, max(1, m.width), max(1, m.height))
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

func (m model) renderFooter() string {
	styles := m.activeTheme().styles
	themeHint := fmt.Sprintf("theme %s: t themes", m.activeTheme().name)
	hints := themeHint + "  |  tab mailboxes  j/k move  enter full reader  R refresh  A account  r reply  c compose  a archive  / search  ? help  q quit"
	if m.mode == readerView {
		hints = themeHint + "  |  j/k scroll  b back  r reply  a archive  d delete  ? help  q back"
	} else if m.mode == setupView {
		hints = m.setupFooterHints()
	} else if m.mode == draftReviewView {
		hints = "draft  |  tab fields  enter next/newline  ctrl+s send  ctrl+o editor  esc discard  ? help"
		if m.draft.Sending {
			hints = "sending email..."
		}
	} else if m.mailboxFocused {
		hints = "mailboxes  |  tab/j/k choose  enter open  right messages  R refresh  ? help  q quit"
	} else if m.searching {
		hints = "search: " + m.searchInput + "_  |  enter apply  esc cancel  ctrl+u clear"
	} else if m.confirmDelete {
		hints = "Move selected email to Trash?  y confirm  n cancel"
	} else if m.action.Running {
		hints = m.action.Kind.presentParticiple() + " email..."
	} else if strings.TrimSpace(m.searchQuery) != "" && m.mode == inboxView {
		hints = themeHint + "  |  tab mailboxes  / search again  esc clear search  j/k move  enter full reader  a archive  d delete  R refresh  ? help"
	} else if m.mailboxFilter == unreadMailFilter && m.mode == inboxView {
		hints = themeHint + "  |  tab mailboxes  esc all mail  j/k move  enter full reader  / search  R refresh  ? help"
	}
	if m.status != "" {
		hints = m.status + "  |  " + hints
	}
	return styles.footer.Width(m.width).Render(truncate(hints, max(1, m.width-2)))
}
