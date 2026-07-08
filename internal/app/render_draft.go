package app

import "strings"

func (m model) renderDraftReview(height int) string {
	styles := m.activeTheme().styles
	width := max(32, m.width)
	summary := m.draft.Summary

	lines := []string{
		styles.panelTitle.Render(m.draft.Kind.title()),
		"",
	}
	if strings.EqualFold(m.composeFormat, "markdown") {
		lines = append(lines, styles.themeBadge.Width(width).Render("Markdown mode: body will be sent as text + HTML"))
		lines = append(lines, "")
	}
	lines = append(lines,
		styles.readerHeader.Width(width).Render("From: "+terminalSafeLine(firstNonEmpty(summary.From, "(backend default)"))),
		m.renderDraftField(width, draftFieldTo, "To", summary.To),
	)
	if strings.TrimSpace(summary.Cc) != "" {
		lines = append(lines, styles.readerHeader.Width(width).Render("Cc: "+terminalSafeLine(summary.Cc)))
	}
	if strings.TrimSpace(summary.Bcc) != "" {
		lines = append(lines, styles.readerHeader.Width(width).Render("Bcc: "+terminalSafeLine(summary.Bcc)))
	}
	lines = append(lines,
		m.renderDraftField(width, draftFieldSubject, "Subject", summary.Subject),
		"",
	)

	if m.draft.Sending {
		lines = append(lines, styles.readerBody.Width(width).Render("Sending email..."))
	} else if err := validateDraftForSend(summary); err != nil {
		lines = append(lines, styles.unread.Width(width).Render(err.Error()), "")
	}

	lines = append(lines, styles.readerHeader.Width(width).Render("Body"))
	body := summary.Body
	if m.draft.Focus == draftFieldBody {
		body = textWithCursor(body, m.draft.Cursor)
	} else {
		body = firstNonEmpty(body, "(empty body)")
	}
	bodyLines := wrapText(body, width-2)
	available := max(1, height-len(lines)-1)
	if len(bodyLines) > available {
		bodyLines = append(bodyLines[:available], "...")
	}
	lines = append(lines, styledLines(bodyLines, styles.readerBody, width)...)
	return fitHeight(strings.Join(lines, "\n"), height)
}

func (m model) renderDraftField(width int, field draftField, label, value string) string {
	styles := m.activeTheme().styles
	display := value
	if m.draft.Focus != field && strings.TrimSpace(display) == "" {
		if field == draftFieldTo {
			display = "(missing recipient)"
		} else if field == draftFieldSubject {
			display = "(no subject)"
		}
	}
	text := label + ": " + terminalSafeLine(display)
	style := styles.readerHeader
	if m.draft.Focus == field {
		text = label + ": " + terminalSafeLine(textWithCursor(value, m.draft.Cursor))
		style = styles.selected
	}
	return style.Width(width).Render(truncate(text, width))
}
