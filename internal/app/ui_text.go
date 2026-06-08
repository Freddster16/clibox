package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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

func dropLastRune(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return ""
	}
	return string(runes[:len(runes)-1])
}

func isAccountNameRune(r rune) bool {
	if r >= 'a' && r <= 'z' {
		return true
	}
	if r >= 'A' && r <= 'Z' {
		return true
	}
	if r >= '0' && r <= '9' {
		return true
	}
	return r == '-' || r == '_' || r == '.'
}
