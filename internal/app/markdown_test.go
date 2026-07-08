package app

import (
	"strings"
	"testing"
)

func TestMarkdownToHTMLBasicFormatting(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "bold and italic",
			input: "This is **bold** and *italic*",
			want:  []string{"<strong>bold</strong>", "<em>italic</em>"},
		},
		{
			name:  "inline code",
			input: "Run `go test` now",
			want:  []string{"<code>go test</code>"},
		},
		{
			name:  "link",
			input: "See [docs](https://example.com) here",
			want:  []string{`<a href="https://example.com">docs</a>`},
		},
		{
			name:  "heading h1",
			input: "# Title\n\nBody",
			want:  []string{"<h1>Title</h1>", "<p>Body</p>"},
		},
		{
			name:  "heading h2 and h3",
			input: "## Subtitle\n### Section",
			want:  []string{"<h2>Subtitle</h2>", "<h3>Section</h3>"},
		},
		{
			name:  "unordered list",
			input: "- one\n- two\n- three",
			want:  []string{"<ul>", "<li>one</li>", "<li>two</li>", "<li>three</li>", "</ul>"},
		},
		{
			name:  "ordered list",
			input: "1. first\n2. second",
			want:  []string{"<ol>", "<li>first</li>", "<li>second</li>", "</ol>"},
		},
		{
			name:  "blockquote",
			input: "> quoted text",
			want:  []string{"<blockquote>", "quoted text", "</blockquote>"},
		},
		{
			name:  "horizontal rule",
			input: "Above\n---\nBelow",
			want:  []string{"<hr>"},
		},
		{
			name:  "paragraph separation",
			input: "First paragraph\n\nSecond paragraph",
			want:  []string{"<p>First paragraph</p>", "<p>Second paragraph</p>"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := markdownToHTML(tc.input)
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Fatalf("expected HTML to contain %q, got:\n%s", want, got)
				}
			}
		})
	}
}

func TestMarkdownToHTMLEscapesHTML(t *testing.T) {
	html := markdownToHTML("Text with <script>alert(1)</script> and & ampersand")
	if strings.Contains(html, "<script>") {
		t.Fatalf("expected HTML to be escaped, got:\n%s", html)
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Fatalf("expected escaped script tag, got:\n%s", html)
	}
	if !strings.Contains(html, "&amp;") {
		t.Fatalf("expected escaped ampersand, got:\n%s", html)
	}
}

func TestLooksLikeMarkdown(t *testing.T) {
	if !looksLikeMarkdown("This is **bold**") {
		t.Fatal("expected **bold** to look like markdown")
	}
	if !looksLikeMarkdown("# Heading") {
		t.Fatal("expected # Heading to look like markdown")
	}
	if !looksLikeMarkdown("- list item") {
		t.Fatal("expected - list item to look like markdown")
	}
	if looksLikeMarkdown("Just a plain text email with no formatting at all") {
		t.Fatal("expected plain text not to look like markdown")
	}
}

func TestBuildMultipartAlternative(t *testing.T) {
	summary := draftSummary{
		From:    "Freddy <freddy@example.com>",
		To:      "alice@example.com",
		Subject: "Test",
		Body:    "# Hello\n\nThis is **bold**.",
	}
	html := markdownToHTML(summary.Body)
	payload := buildMultipartAlternative(summary, html)

	if !strings.Contains(payload, "multipart/alternative") {
		t.Fatalf("expected MIME multipart/alternative content type, got:\n%s", payload)
	}
	if !strings.Contains(payload, "boundary=") {
		t.Fatalf("expected MIME boundary, got payload without one")
	}
	if !strings.Contains(payload, "text/plain") {
		t.Fatalf("expected text/plain part, got:\n%s", payload)
	}
	if !strings.Contains(payload, "text/html") {
		t.Fatalf("expected text/html part, got:\n%s", payload)
	}
	if !strings.Contains(payload, "# Hello") {
		t.Fatalf("expected plain text to contain original markdown, got:\n%s", payload)
	}
	if !strings.Contains(payload, "<h1>Hello</h1>") {
		t.Fatalf("expected HTML to contain rendered heading, got:\n%s", payload)
	}
	if !strings.Contains(payload, "<strong>bold</strong>") {
		t.Fatalf("expected HTML to contain rendered bold, got:\n%s", payload)
	}
	if strings.Contains(payload, "Bcc:") {
		t.Fatalf("expected no Bcc header in multipart payload, got:\n%s", payload)
	}
}
