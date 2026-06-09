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
backend = "/usr/local/bin/himalaya"
page_size = 50
confirm_delete = true
`)
	if err != nil {
		t.Fatalf("expected config to parse: %v", err)
	}
	if config.Account != "personal" || config.Mailbox != "INBOX" || config.ArchiveFolder != "Archive" {
		t.Fatalf("unexpected mailbox config: %+v", config)
	}
	if config.Editor != "nvim" || config.Theme != "lagoon" || config.Backend != "/usr/local/bin/himalaya" {
		t.Fatalf("unexpected command config: %+v", config)
	}
	if config.PageSize != 50 {
		t.Fatalf("expected page size 50, got %d", config.PageSize)
	}
	if config.ConfirmDelete == nil || !*config.ConfirmDelete {
		t.Fatalf("expected confirm_delete true, got %+v", config.ConfirmDelete)
	}
}

func TestParseConfigRejectsUnknownKeys(t *testing.T) {
	_, err := parseConfig(`password = "nope"`)
	if err == nil || !strings.Contains(err.Error(), "unknown config key") {
		t.Fatalf("expected unknown key error, got %v", err)
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
	if config != (Config{}) {
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
