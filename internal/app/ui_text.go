package app

import (
	"strings"
	"unicode/utf8"

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
	text = terminalSafeText(text)
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
	value = terminalSafeLine(value)
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

func terminalSafeLine(value string) string {
	return strings.Join(strings.Fields(terminalSafeText(value)), " ")
}

func terminalSafeText(value string) string {
	if value == "" {
		return ""
	}
	var out strings.Builder
	for i := 0; i < len(value); {
		r, size := utf8.DecodeRuneInString(value[i:])
		if r == utf8.RuneError && size == 1 {
			i++
			continue
		}
		switch {
		case r == '\x1b':
			i += skipEscapeSequence(value[i:])
			continue
		case r == '\u009b':
			i += size + skipControlSequence(value[i+size:])
			continue
		case r == '\u0090' || r == '\u0098' || r == '\u009d' || r == '\u009e' || r == '\u009f':
			i += size + skipStringControl(value[i+size:])
			continue
		case r == '\n':
			out.WriteRune(r)
		case r == '\t':
			out.WriteRune(' ')
		case isTerminalControlRune(r) || isBidiControlRune(r):
			// Drop terminal controls so remote mail cannot move the cursor,
			// rewrite clipboard/window state, or visually reorder trusted text.
		default:
			out.WriteRune(r)
		}
		i += size
	}
	return out.String()
}

func skipEscapeSequence(value string) int {
	if value == "" || value[0] != '\x1b' {
		return 0
	}
	if len(value) == 1 {
		return 1
	}
	switch value[1] {
	case '[':
		return 2 + skipControlSequence(value[2:])
	case ']', 'P', 'X', '^', '_':
		return 2 + skipStringControl(value[2:])
	case '(', ')', '*', '+', '-', '.', '/':
		return min(len(value), 3)
	default:
		return min(len(value), 2)
	}
}

func skipControlSequence(value string) int {
	for i := 0; i < len(value); i++ {
		if value[i] >= 0x40 && value[i] <= 0x7e {
			return i + 1
		}
	}
	return len(value)
}

func skipStringControl(value string) int {
	for i := 0; i < len(value); i++ {
		if value[i] == '\a' {
			return i + 1
		}
		if value[i] == '\x1b' && i+1 < len(value) && value[i+1] == '\\' {
			return i + 2
		}
	}
	return len(value)
}

func isTerminalControlRune(r rune) bool {
	return (r >= 0 && r < 0x20) || r == 0x7f || (r >= 0x80 && r <= 0x9f)
}

func isBidiControlRune(r rune) bool {
	return (r >= '\u202a' && r <= '\u202e') || (r >= '\u2066' && r <= '\u2069') || r == '\u200e' || r == '\u200f'
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
