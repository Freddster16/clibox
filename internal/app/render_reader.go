package app

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const inlineImageRows = 6

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
			styles.readerBody.Width(width).Render("Finish account setup, then press R to load your inbox."),
		}, "\n"), height)
	}

	msg := m.selectedMessage()
	lines := []string{
		styles.panelTitle.Render("Reader"),
		renderReaderHeaderLine(styles.readerHeader, width, "From: "+terminalSafeLine(msg.From)+" <"+terminalSafeLine(msg.Email)+">"),
		renderReaderHeaderLine(styles.readerHeader, width, "Subject: "+terminalSafeLine(msg.Subject)),
		renderReaderHeaderLine(styles.readerHeader, width, "Date: "+terminalSafeLine(msg.Date)),
		styles.readerBody.Width(width).Render(""),
	}

	bodyLines := m.renderMessageBodyLines(msg, includePreview, width)
	if !includePreview {
		bodyHeight := max(1, height-len(lines))
		offset := min(m.readerOffset, max(0, len(bodyLines)-bodyHeight))
		end := min(len(bodyLines), offset+bodyHeight)
		if offset > 0 || end < len(bodyLines) {
			lines[0] = styles.panelTitle.Render(fmt.Sprintf("Reader  %d-%d/%d", min(offset+1, len(bodyLines)), end, len(bodyLines)))
		}
		bodyLines = bodyLines[offset:end]
	}
	lines = append(lines, bodyLines...)
	return fitHeight(strings.Join(lines, "\n"), height)
}

func renderReaderHeaderLine(style lipgloss.Style, width int, text string) string {
	return style.Width(width).Render(truncate(text, max(1, width-2)))
}

func (m model) renderMessageBodyLines(msg message, includePreview bool, width int) []string {
	styles := m.activeTheme().styles
	bodyLines := styledLines(wrapText(m.messageBodyText(msg, includePreview), width-2), styles.readerBody, width)
	if !includePreview && strings.TrimSpace(msg.Notice) != "" {
		if len(bodyLines) > 0 && strings.TrimSpace(bodyLines[len(bodyLines)-1]) != "" {
			bodyLines = append(bodyLines, styles.readerBody.Width(width).Render(""))
		}
		bodyLines = append(bodyLines, styledLines(wrapText(msg.Notice, width-2), styles.unread, width)...)
	}
	if !includePreview && messageBodyReady(msg) && len(msg.Images) > 0 {
		if len(bodyLines) > 0 && strings.TrimSpace(bodyLines[len(bodyLines)-1]) != "" {
			bodyLines = append(bodyLines, styles.readerBody.Width(width).Render(""))
		}
		bodyLines = append(bodyLines, renderMessageImages(msg.Images, styles.readerBody, width)...)
	}
	return bodyLines
}

func renderMessageImages(images []messageImage, style lipgloss.Style, width int) []string {
	var lines []string
	for i, image := range images {
		label := fmt.Sprintf("Image %d: %s", i+1, firstNonEmpty(image.Name, "inline image"))
		lines = append(lines, style.Width(width).Render(truncate(label, width)))
		if len(image.Data) == 0 {
			continue
		}
		lines = append(lines, style.Width(width).Render(inlineImageSequence(image, width)))
		for row := 1; row < inlineImageRows; row++ {
			lines = append(lines, style.Width(width).Render(""))
		}
	}
	return lines
}

func inlineImageSequence(image messageImage, width int) string {
	imageWidth := max(8, min(width-2, 80))
	name := base64.StdEncoding.EncodeToString([]byte(firstNonEmpty(image.Name, "image")))
	data := base64.StdEncoding.EncodeToString(image.Data)
	return fmt.Sprintf("\x1b]1337;File=inline=1;width=%d;height=%d;preserveAspectRatio=1;name=%s:%s\a", imageWidth, inlineImageRows, name, data)
}
