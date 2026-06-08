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
