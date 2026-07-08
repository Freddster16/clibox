package app

import (
	"os"
	"strings"
	"testing"
)

func TestLocalReplyTemplateSanitizesHeaders(t *testing.T) {
	template := localReplyTemplate(message{
		Email:   "alice@example.com\nBcc: attacker@example.com",
		Subject: "Hello\nInjected: yes",
		Preview: "Original body",
	}, "Freddy\nBad: value <freddy@example.com>")

	for _, line := range strings.Split(template, "\n") {
		if strings.HasPrefix(line, "Bcc: attacker") || strings.HasPrefix(line, "Injected:") || strings.HasPrefix(line, "Bad:") {
			t.Fatalf("expected fallback template to sanitize injected headers, got:\n%s", template)
		}
	}
	if !strings.Contains(template, "Subject: Re: Hello Injected: yes") {
		t.Fatalf("expected sanitized subject continuation, got:\n%s", template)
	}
}

func TestLocalReplyTemplateSanitizesQuotedBodyControls(t *testing.T) {
	template := localReplyTemplate(message{
		Email:   "alice@example.com",
		Subject: "Hello",
		Body:    "Hello\x1b]52;c;SGVsbG8=\a red\x1b[31m text",
	}, "Freddy <freddy@example.com>")

	for _, unsafe := range []string{"\x1b", "\a", "]52", "[31m"} {
		if strings.Contains(template, unsafe) {
			t.Fatalf("reply template leaked terminal control payload %q in:\n%s", unsafe, template)
		}
	}
	if !strings.Contains(template, "> Hello red text") {
		t.Fatalf("expected sanitized quoted body, got:\n%s", template)
	}
}

func TestSMTPDraftContentStripsBccHeaderAndContinuations(t *testing.T) {
	content := "From: me@example.com\nTo: visible@example.com\nBcc: hidden@example.com\n\tsecond@example.com\nSubject: Hi\n\nBody\nBcc: not a header\n"

	summary := parseDraftSummary(content)
	if !strings.Contains(summary.Bcc, "hidden@example.com") || !strings.Contains(summary.Bcc, "second@example.com") {
		t.Fatalf("expected draft parser to keep Bcc recipients for envelope, got %q", summary.Bcc)
	}

	payload := smtpDraftContent(content)
	headers, body, ok := strings.Cut(payload, "\n\n")
	if !ok {
		t.Fatalf("expected payload to contain header/body separator, got %q", payload)
	}
	for _, hidden := range []string{"Bcc:", "hidden@example.com", "second@example.com"} {
		if strings.Contains(headers, hidden) {
			t.Fatalf("SMTP payload headers leaked %q:\n%s", hidden, headers)
		}
	}
	if !strings.Contains(body, "Bcc: not a header") {
		t.Fatalf("expected body text to be preserved, got %q", body)
	}
}

func TestDraftTempFileUsesOwnerOnlyPermissions(t *testing.T) {
	path, err := writeDraftFile("To: alice@example.com\n\nHi\n")
	if err != nil {
		t.Fatalf("expected draft file to be created: %v", err)
	}
	defer removeDraftFile(path)

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected draft file to exist: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected draft file permissions 0600, got %#o", got)
	}
}

func TestLocalForwardTemplateSanitizesHeaders(t *testing.T) {
	template := localForwardTemplate(message{
		From:    "Alice\nBcc: attacker@example.com",
		Email:   "alice@example.com\nInjected: bad",
		Subject: "Hello\nX-Header: yes",
		Body:    "Original body",
	}, "Freddy\nBad: value <freddy@example.com>")

	for _, line := range strings.Split(template, "\n") {
		if strings.HasPrefix(line, "Bcc: attacker") || strings.HasPrefix(line, "Injected:") || strings.HasPrefix(line, "X-Header:") || strings.HasPrefix(line, "Bad:") {
			t.Fatalf("expected forward template to sanitize injected headers, got:\n%s", template)
		}
	}
	if !strings.Contains(template, "Subject: Fwd: Hello X-Header: yes") {
		t.Fatalf("expected sanitized Fwd: subject, got:\n%s", template)
	}
	if !strings.Contains(template, "--------- Forwarded message ---------") {
		t.Fatalf("expected forwarded message header marker, got:\n%s", template)
	}
	if !strings.Contains(template, "Original body") {
		t.Fatalf("expected original body in forward, got:\n%s", template)
	}
}

func TestLocalForwardTemplateSanitizesBodyControls(t *testing.T) {
	template := localForwardTemplate(message{
		From:    "Alice",
		Email:   "alice@example.com",
		Subject: "Hello",
		Body:    "Hello\x1b]52;c;SGVsbG8=\a red\x1b[31m text",
	}, "Freddy <freddy@example.com>")

	for _, unsafe := range []string{"\x1b", "\a", "]52", "[31m"} {
		if strings.Contains(template, unsafe) {
			t.Fatalf("forward template leaked terminal control payload %q in:\n%s", unsafe, template)
		}
	}
	if !strings.Contains(template, "Hello red text") {
		t.Fatalf("expected sanitized forwarded body, got:\n%s", template)
	}
}

func TestLocalForwardTemplatePrependsFwd(t *testing.T) {
	template := localForwardTemplate(message{
		From:    "Alice",
		Email:   "alice@example.com",
		Subject: "Fwd: Already forwarded",
		Body:    "Body",
	}, "Freddy <freddy@example.com>")

	if strings.Contains(template, "Fwd: Fwd:") {
		t.Fatalf("expected no double Fwd: prefix, got:\n%s", template)
	}
	if !strings.Contains(template, "Subject: Fwd: Already forwarded") {
		t.Fatalf("expected single Fwd: prefix, got:\n%s", template)
	}
}
