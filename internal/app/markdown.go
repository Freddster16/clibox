package app

import (
	"html"
	"regexp"
	"strings"
)

var (
	mdBoldRE       = regexp.MustCompile(`\*\*(.+?)\*\*`)
	mdItalicRE     = regexp.MustCompile(`\*(.+?)\*`)
	mdCodeRE       = regexp.MustCompile("`(.+?)`")
	mdLinkRE       = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	mdH1RE         = regexp.MustCompile(`^#\s+(.+)$`)
	mdH2RE         = regexp.MustCompile(`^##\s+(.+)$`)
	mdH3RE         = regexp.MustCompile(`^###\s+(.+)$`)
	mdUnorderedRE  = regexp.MustCompile(`^[-*]\s+(.+)$`)
	mdOrderedRE    = regexp.MustCompile(`^\d+\.\s+(.+)$`)
	mdBlockquoteRE = regexp.MustCompile(`^>\s?(.*)$`)
	mdHrRE         = regexp.MustCompile(`^---+$`)
)

func markdownToHTML(input string) string {
	lines := strings.Split(strings.ReplaceAll(input, "\r\n", "\n"), "\n")
	var out strings.Builder
	inList := ""
	inBlockquote := false

	flushList := func() {
		if inList != "" {
			out.WriteString("</" + inList + ">\n")
			inList = ""
		}
	}
	flushBlockquote := func() {
		if inBlockquote {
			out.WriteString("</blockquote>\n")
			inBlockquote = false
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			flushList()
			flushBlockquote()
			continue
		}

		if mdHrRE.MatchString(trimmed) {
			flushList()
			flushBlockquote()
			out.WriteString("<hr>\n")
			continue
		}

		if m := mdH1RE.FindStringSubmatch(trimmed); m != nil {
			flushList()
			flushBlockquote()
			out.WriteString("<h1>" + inlineMarkdown(m[1]) + "</h1>\n")
			continue
		}
		if m := mdH2RE.FindStringSubmatch(trimmed); m != nil {
			flushList()
			flushBlockquote()
			out.WriteString("<h2>" + inlineMarkdown(m[1]) + "</h2>\n")
			continue
		}
		if m := mdH3RE.FindStringSubmatch(trimmed); m != nil {
			flushList()
			flushBlockquote()
			out.WriteString("<h3>" + inlineMarkdown(m[1]) + "</h3>\n")
			continue
		}

		if m := mdUnorderedRE.FindStringSubmatch(trimmed); m != nil {
			flushBlockquote()
			if inList != "ul" {
				flushList()
				out.WriteString("<ul>\n")
				inList = "ul"
			}
			out.WriteString("<li>" + inlineMarkdown(m[1]) + "</li>\n")
			continue
		}

		if m := mdOrderedRE.FindStringSubmatch(trimmed); m != nil {
			flushBlockquote()
			if inList != "ol" {
				flushList()
				out.WriteString("<ol>\n")
				inList = "ol"
			}
			out.WriteString("<li>" + inlineMarkdown(m[1]) + "</li>\n")
			continue
		}

		if m := mdBlockquoteRE.FindStringSubmatch(trimmed); m != nil {
			flushList()
			if !inBlockquote {
				out.WriteString("<blockquote>\n")
				inBlockquote = true
			}
			out.WriteString(inlineMarkdown(m[1]) + "<br>\n")
			continue
		}

		flushList()
		flushBlockquote()
		out.WriteString("<p>" + inlineMarkdown(trimmed) + "</p>\n")
	}

	flushList()
	flushBlockquote()
	return out.String()
}

func inlineMarkdown(text string) string {
	text = html.EscapeString(text)
	text = mdLinkRE.ReplaceAllString(text, `<a href="$2">$1</a>`)
	text = mdBoldRE.ReplaceAllString(text, "<strong>$1</strong>")
	text = mdItalicRE.ReplaceAllString(text, "<em>$1</em>")
	text = mdCodeRE.ReplaceAllString(text, "<code>$1</code>")
	return text
}

func looksLikeMarkdown(body string) bool {
	indicators := []string{"**", "* ", "- ", "1. ", "# ", "## ", "### ", "> ", "[", "`", "---"}
	for _, ind := range indicators {
		if strings.Contains(body, ind) {
			return true
		}
	}
	return false
}
