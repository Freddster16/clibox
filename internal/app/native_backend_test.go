package app

import (
	"strings"
	"testing"
)

func TestNativePageWindowUsesRemoteTotalForDone(t *testing.T) {
	start, end, done := nativePageWindow(120, 1, 50)
	if start != 0 || end != 50 || done {
		t.Fatalf("page 1 = start %d end %d done %v, want 0 50 false", start, end, done)
	}

	start, end, done = nativePageWindow(120, 3, 50)
	if start != 100 || end != 120 || !done {
		t.Fatalf("page 3 = start %d end %d done %v, want 100 120 true", start, end, done)
	}

	start, end, done = nativePageWindow(120, 4, 50)
	if start != 0 || end != 0 || !done {
		t.Fatalf("page beyond total = start %d end %d done %v, want 0 0 true", start, end, done)
	}
}

func TestNativeEnvelopeSeqRangeUsesRemoteTotalForDone(t *testing.T) {
	from, to, done, ok := nativeEnvelopeSeqRange(120, 1, 50)
	if from != 71 || to != 120 || done || !ok {
		t.Fatalf("page 1 = from %d to %d done %v ok %v, want 71 120 false true", from, to, done, ok)
	}

	from, to, done, ok = nativeEnvelopeSeqRange(120, 3, 50)
	if from != 1 || to != 20 || !done || !ok {
		t.Fatalf("page 3 = from %d to %d done %v ok %v, want 1 20 true true", from, to, done, ok)
	}

	_, _, done, ok = nativeEnvelopeSeqRange(120, 4, 50)
	if !done || ok {
		t.Fatalf("page beyond total = done %v ok %v, want true false", done, ok)
	}
}

func TestHtmlToTextPreservesParagraphBreaks(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "br and paragraphs",
			input: `<p>Hello</p><p>World</p>`,
			want:  "Hello\n\nWorld",
		},
		{
			name:  "br tag",
			input: `line one<br>line two<br/>line three`,
			want:  "line one\nline two\nline three",
		},
		{
			name:  "inline whitespace collapsed, newlines kept",
			input: "<p>foo   bar\tbaz</p><p>second</p>",
			want:  "foo bar baz\n\nsecond",
		},
		{
			name:  "list items on separate lines",
			input: `<ul><li>one</li><li>two</li></ul>`,
			want:  "one\ntwo",
		},
		{
			name:  "script and style stripped",
			input: `<style>body{color:red}</style><script>alert(1)</script><p>visible</p>`,
			want:  "visible",
		},
		{
			name:  "entities unescaped",
			input: `<p>tom &amp; jerry &lt;3</p>`,
			want:  "tom & jerry <3",
		},
		{
			name:  "headings break",
			input: `<h1>Title</h1><p>Body</p>`,
			want:  "Title\n\nBody",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := htmlToText(tc.input)
			if got != tc.want {
				t.Fatalf("htmlToText(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestExtractReadableMessageContentIncludesImages(t *testing.T) {
	raw := strings.Join([]string{
		"MIME-Version: 1.0",
		`Content-Type: multipart/mixed; boundary="clibox-test"`,
		"",
		"--clibox-test",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"Hello with image.",
		"--clibox-test",
		"Content-Type: image/png",
		`Content-Disposition: inline; filename="pixel.png"`,
		"Content-Transfer-Encoding: base64",
		"",
		"aGVsbG8=",
		"--clibox-test--",
		"",
	}, "\r\n")

	content := extractReadableMessageContent([]byte(raw))
	if content.Body != "Hello with image." {
		t.Fatalf("unexpected body: %q", content.Body)
	}
	if len(content.Images) != 1 {
		t.Fatalf("expected one image, got %+v", content.Images)
	}
	image := content.Images[0]
	if image.Name != "pixel.png" || image.ContentType != "image/png" || string(image.Data) != "hello" {
		t.Fatalf("unexpected image: %+v", image)
	}
	if content.Notice != "" {
		t.Fatalf("expected no notice for a single small image, got %q", content.Notice)
	}
}

func TestExtractReadableMessageContentNoticesDroppedImages(t *testing.T) {
	raw := strings.Join([]string{
		"MIME-Version: 1.0",
		`Content-Type: multipart/mixed; boundary="clibox-test"`,
		"",
		"--clibox-test",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"Two images attached.",
		"--clibox-test",
		"Content-Type: image/png",
		`Content-Disposition: inline; filename="first.png"`,
		"Content-Transfer-Encoding: base64",
		"",
		"aGVsbG8=",
		"--clibox-test",
		"Content-Type: image/png",
		`Content-Disposition: inline; filename="second.png"`,
		"Content-Transfer-Encoding: base64",
		"",
		"d29ybGQ=",
		"--clibox-test--",
		"",
	}, "\r\n")

	content := extractReadableMessageContent([]byte(raw))
	if len(content.Images) != 1 {
		t.Fatalf("expected one kept image, got %d", len(content.Images))
	}
	if content.Notice == "" || !strings.Contains(content.Notice, "1 inline image") {
		t.Fatalf("expected dropped-image notice, got %q", content.Notice)
	}
}
