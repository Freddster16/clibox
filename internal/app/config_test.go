package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseConfigMinimalSurface(t *testing.T) {
	config, err := parseConfig(`
# clibox local preferences
account = "personal"
mailbox = "INBOX"
archive_folder = "Archive"
editor = "nvim"
theme = "lagoon"
backend = "himalaya"
himalaya_binary = "/usr/local/bin/himalaya"
page_size = 50
confirm_delete = true
`)
	if err != nil {
		t.Fatalf("expected config to parse: %v", err)
	}
	if config.Account != "personal" || config.Mailbox != "INBOX" || config.ArchiveFolder != "Archive" {
		t.Fatalf("unexpected mailbox config: %+v", config)
	}
	if config.Editor != "nvim" || config.Theme != "lagoon" || config.Backend != "himalaya" || config.Himalaya != "/usr/local/bin/himalaya" {
		t.Fatalf("unexpected command config: %+v", config)
	}
	if config.PageSize != 50 {
		t.Fatalf("expected page size 50, got %d", config.PageSize)
	}
	if config.ConfirmDelete == nil || !*config.ConfirmDelete {
		t.Fatalf("expected confirm_delete true, got %+v", config.ConfirmDelete)
	}
}

func TestParseConfigTreatsLegacyBackendPathAsHimalayaBinary(t *testing.T) {
	config, err := parseConfig(`backend = "/usr/local/bin/himalaya"`)
	if err != nil {
		t.Fatalf("expected legacy backend path to parse: %v", err)
	}
	if config.Backend != "himalaya" || config.Himalaya != "/usr/local/bin/himalaya" {
		t.Fatalf("unexpected legacy backend parse: %+v", config)
	}
}

func TestParseConfigNativeAccountSection(t *testing.T) {
	config, err := parseConfig(`
backend = "native"

[accounts.gmail]
provider = "gmail"
email = "person@gmail.com"
mailbox = "INBOX"
archive_folder = "[Gmail]/All Mail"
sync_policy = "headers"
editor = "nvim"
`)
	if err != nil {
		t.Fatalf("expected account section to parse: %v", err)
	}
	account := config.Accounts["gmail"]
	if config.Backend != "native" || account.Email != "person@gmail.com" || account.Provider != "gmail" || account.ArchiveFolder != "[Gmail]/All Mail" {
		t.Fatalf("unexpected native account config: %+v account=%+v", config, account)
	}
}

func TestParseConfigRejectsUnknownKeys(t *testing.T) {
	_, err := parseConfig(strings.Join([]string{"pass", "word"}, "") + ` = "nope"`)
	if err == nil || !strings.Contains(err.Error(), "credential key") {
		t.Fatalf("expected credential key error, got %v", err)
	}
}

func TestParseConfigRejectsCredentialKeysInsideAccountSections(t *testing.T) {
	key := strings.Join([]string{"refresh", "token"}, "_")
	_, err := parseConfig(`
[accounts.gmail]
` + key + ` = "nope"
`)
	if err == nil || !strings.Contains(err.Error(), "credential key") {
		t.Fatalf("expected credential key error, got %v", err)
	}
}

func TestLoadConfigMissingFileIsOK(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.toml")
	config, resolved, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("expected missing config to be optional: %v", err)
	}
	if resolved != path {
		t.Fatalf("expected resolved path %q, got %q", path, resolved)
	}
	if config.Theme != "" || config.Account != "" || config.Mailbox != "" || config.Accounts != nil {
		t.Fatalf("expected empty config, got %+v", config)
	}
}

func TestLoadConfigUsesEnvironmentPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	t.Setenv("CLIBOX_CONFIG", path)
	if err := os.WriteFile(path, []byte(`account = "work"`), 0o600); err != nil {
		t.Fatalf("expected config write to succeed: %v", err)
	}
	config, resolved, err := LoadConfig("")
	if err != nil {
		t.Fatalf("expected config to load: %v", err)
	}
	if resolved != path || config.Account != "work" {
		t.Fatalf("unexpected config result: %q %+v", resolved, config)
	}
}

func TestLoadConfigRejectsWritableByOthers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(`account = "work"`), 0o666); err != nil {
		t.Fatalf("expected config write to succeed: %v", err)
	}
	if err := os.Chmod(path, 0o666); err != nil {
		t.Fatalf("expected chmod to succeed: %v", err)
	}
	_, _, err := LoadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "must not be group- or world-writable") {
		t.Fatalf("expected writable config to be rejected, got %v", err)
	}
}
