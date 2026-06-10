package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeStoreCachesAccountEnvelopesAndBodiesWithoutCredentialColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clibox.db")
	store, err := openNativeStore(path)
	if err != nil {
		t.Fatalf("expected native store to open: %v", err)
	}
	defer store.close()

	bad, columns, err := store.schemaHasCredentialColumns(context.Background())
	if err != nil {
		t.Fatalf("expected schema check to run: %v", err)
	}
	if bad {
		t.Fatalf("native cache schema must not contain credential columns: %v", columns)
	}

	account := nativeAccountFromProvider("gmail", "person@gmail.com", "Person", detectProvider("person@gmail.com"), "oauth2")
	account.OAuthClientID = "client-id"
	account.OAuthScopes = gmailOAuthProvider().Scopes
	if err := store.saveAccount(context.Background(), account); err != nil {
		t.Fatalf("expected account save: %v", err)
	}

	loaded, err := store.account(context.Background(), "gmail")
	if err != nil {
		t.Fatalf("expected account lookup: %v", err)
	}
	if loaded.Email != "person@gmail.com" || loaded.AuthType != "oauth2" || strings.Contains(strings.Join(loaded.OAuthScopes, " "), "refresh") {
		t.Fatalf("unexpected account metadata: %+v", loaded)
	}

	messages := []message{
		{ID: "101", From: "Alice", Email: "alice@example.com", Subject: "Design notes", Date: "10:00 AM", Preview: "prototype", Unread: true},
		{ID: "100", From: "Build", Email: "ci@example.com", Subject: "Deploy passed", Date: "Yesterday", Preview: "green"},
	}
	if err := store.upsertEnvelopes(context.Background(), "gmail", "INBOX", messages); err != nil {
		t.Fatalf("expected envelopes to cache: %v", err)
	}
	page, done, err := store.cachedEnvelopePage(context.Background(), "gmail", "INBOX", 1, 1, "")
	if err != nil {
		t.Fatalf("expected cached page: %v", err)
	}
	if done || len(page) != 1 || page[0].ID != "101" {
		t.Fatalf("unexpected first page: done=%v page=%+v", done, page)
	}
	results, _, err := store.cachedEnvelopePage(context.Background(), "gmail", "INBOX", 1, 10, "deploy")
	if err != nil {
		t.Fatalf("expected cached search: %v", err)
	}
	if len(results) != 1 || results[0].ID != "100" {
		t.Fatalf("unexpected search results: %+v", results)
	}

	if err := store.saveBody(context.Background(), "gmail", "INBOX", "101", "hello body"); err != nil {
		t.Fatalf("expected body save: %v", err)
	}
	body, ok, err := store.body(context.Background(), "gmail", "INBOX", "101")
	if err != nil || !ok || body != "hello body" {
		t.Fatalf("unexpected cached body ok=%v body=%q err=%v", ok, body, err)
	}
}

func TestNativeBackendSeedsConfigAccounts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clibox.db")
	backend := newNativeBackend(Options{
		StatePath: path,
		Account:   "gmail",
		Accounts: map[string]AccountConfig{
			"gmail": {
				Name:          "gmail",
				Email:         "person@gmail.com",
				Provider:      "gmail",
				Mailbox:       "INBOX",
				ArchiveFolder: "[Gmail]/All Mail",
				SyncPolicy:    "headers",
			},
		},
	})
	store, err := openNativeStore(path)
	if err != nil {
		t.Fatalf("expected store open: %v", err)
	}
	defer store.close()
	if err := backend.seedConfiguredAccounts(context.Background(), store); err != nil {
		t.Fatalf("expected seed to succeed: %v", err)
	}
	account, err := store.account(context.Background(), "gmail")
	if err != nil {
		t.Fatalf("expected seeded account: %v", err)
	}
	if account.AuthType != "oauth2" || account.ArchiveFolder != "[Gmail]/All Mail" {
		t.Fatalf("unexpected seeded account: %+v", account)
	}
}
