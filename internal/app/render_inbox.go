package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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
	} else if !m.loadedAll && end == len(m.messages) && len(rows) < visible {
		rows = append(rows, styles.muted.Width(width).Render(truncate("  Press j to load older mail...", width)))
	}

	return rows
}
