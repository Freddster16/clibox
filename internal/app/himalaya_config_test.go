package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildHimalayaAccountBlockUsesBackendConfig(t *testing.T) {
	setup := accountSetup{
		Account:     "gmail",
		Email:       "freddy@gmail.com",
		DisplayName: "Freddy",
		Provider:    detectProvider("freddy@gmail.com"),
		PageSize:    25,
	}
	block := buildHimalayaAccountBlock(setup, credentialRef{Command: "security find-generic-password -a freddy@gmail.com -s clibox:gmail:freddy@gmail.com -w"}, true)

	for _, want := range []string{
		"[accounts.gmail]",
		"default = true",
		"email = \"freddy@gmail.com\"",
		"folder.aliases.sent = \"[Gmail]/Sent Mail\"",
		"folder.aliases.archive = \"[Gmail]/All Mail\"",
		"backend.type = \"imap\"",
		"backend.host = \"imap.gmail.com\"",
		"backend.port = 993",
		"backend.auth.cmd = \"security find-generic-password -a freddy@gmail.com -s clibox:gmail:freddy@gmail.com -w\"",
		"message.send.backend.type = \"smtp\"",
		"message.send.backend.host = \"smtp.gmail.com\"",
		"message.send.backend.port = 587",
		"message.send.backend.encryption.type = \"start-tls\"",
	} {
		if !strings.Contains(block, want) {
			t.Fatalf("expected block to contain %q:\n%s", want, block)
		}
	}
}

func TestBuildHimalayaAccountBlockOmitsPageSizeWhenUnset(t *testing.T) {
	setup := accountSetup{
		Account:     "gmail",
		Email:       "freddy@gmail.com",
		DisplayName: "Freddy",
		Provider:    detectProvider("freddy@gmail.com"),
	}
	block := buildHimalayaAccountBlock(setup, credentialRef{Raw: "secret"}, true)
	if strings.Contains(block, "envelope.list.page-size") {
		t.Fatalf("expected generated config not to set a page size by default:\n%s", block)
	}
}

func TestWriteHimalayaAccountConfigCreatesAndReplacesAccount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "himalaya", "config.toml")
	setup := accountSetup{
		Account:     "gmail",
		Email:       "freddy@gmail.com",
		DisplayName: "Freddy",
		Provider:    detectProvider("freddy@gmail.com"),
		PageSize:    25,
	}
	credential := credentialRef{Raw: "first"}

	if err := writeHimalayaAccountConfig(path, setup, credential); err != nil {
		t.Fatalf("expected first write to succeed: %v", err)
	}
	setup.Email = "other@gmail.com"
	credential.Raw = "second"
	if err := writeHimalayaAccountConfig(path, setup, credential); err != nil {
		t.Fatalf("expected replacement write to succeed: %v", err)
	}

	data, err := readFileString(path)
	if err != nil {
		t.Fatalf("expected config to be readable: %v", err)
	}
	if strings.Count(data, "[accounts.gmail]") != 1 {
		t.Fatalf("expected one gmail account block, got:\n%s", data)
	}
	if strings.Contains(data, "freddy@gmail.com") || !strings.Contains(data, "other@gmail.com") {
		t.Fatalf("expected gmail block to be replaced, got:\n%s", data)
	}
}

func TestReplacingDefaultAccountPreservesDefault(t *testing.T) {
	content := `[accounts.gmail]
default = true
email = "old@gmail.com"

[accounts.work]
default = false
email = "work@example.com"
`
	setup := accountSetup{
		Account:     "gmail",
		Email:       "new@gmail.com",
		DisplayName: "New",
		Provider:    detectProvider("new@gmail.com"),
		PageSize:    25,
	}
	block := buildHimalayaAccountBlock(setup, credentialRef{Raw: "secret"}, defaultAccountValue(content, setup.Account))
	updated := upsertAccountBlock(content, setup.Account, block)

	if !strings.Contains(updated, "[accounts.gmail]\ndefault = true") {
		t.Fatalf("expected gmail to remain default, got:\n%s", updated)
	}
	if !strings.Contains(updated, "[accounts.work]\ndefault = false") {
		t.Fatalf("expected work account to be preserved, got:\n%s", updated)
	}
}

func TestHimalayaAccountHintReadsDefaultAccount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "himalaya", "config.toml")
	t.Setenv("HIMALAYA_CONFIG", path)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("expected config dir to be created: %v", err)
	}
	content := `downloads-dir = "~/Downloads"

[accounts.work]
default = false
email = "work@example.com"

[accounts.gmail]
default = true
email = "person@gmail.com"
display-name = "Example Person"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("expected config write to succeed: %v", err)
	}

	hint, ok := himalayaAccountHint("")
	if !ok {
		t.Fatal("expected account hint")
	}
	if hint.Account != "gmail" || hint.Email != "person@gmail.com" {
		t.Fatalf("unexpected hint: %+v", hint)
	}
	if hint.DisplayName != "Example Person" {
		t.Fatalf("expected display name from config, got %q", hint.DisplayName)
	}
	if hint.Provider.Name != "Gmail" {
		t.Fatalf("expected Gmail provider, got %+v", hint.Provider)
	}
}

func readFileString(path string) (string, error) {
	data, err := os.ReadFile(path)
	return string(data), err
}
