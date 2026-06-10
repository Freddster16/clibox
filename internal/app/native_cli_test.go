package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeAuthAddCreatesOAuthMetadataWithoutCredential(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clibox.db")
	report, err := NativeAuthAdd(context.Background(), Options{StatePath: path, Account: "gmail"}, "person@gmail.com")
	if err != nil {
		t.Fatalf("expected auth add to succeed: %v", err)
	}
	if !strings.Contains(report, "Added native OAuth account gmail") || !strings.Contains(report, "CLIBOX_GMAIL_CLIENT_ID") {
		t.Fatalf("expected useful auth add report, got %q", report)
	}

	store, err := openNativeStore(path)
	if err != nil {
		t.Fatalf("expected store open: %v", err)
	}
	defer store.close()
	account, err := store.account(context.Background(), "gmail")
	if err != nil {
		t.Fatalf("expected account metadata: %v", err)
	}
	if account.AuthType != "oauth2" || account.Email != "person@gmail.com" {
		t.Fatalf("unexpected account: %+v", account)
	}
}

func TestNativeAuthLoginExplainsMissingClientID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clibox.db")
	if _, err := NativeAuthAdd(context.Background(), Options{StatePath: path, Account: "gmail"}, "person@gmail.com"); err != nil {
		t.Fatalf("expected auth add to succeed: %v", err)
	}
	_, err := NativeAuthLogin(context.Background(), Options{StatePath: path, Account: "gmail"})
	if err == nil || !strings.Contains(err.Error(), "CLIBOX_GMAIL_CLIENT_ID") {
		t.Fatalf("expected missing client ID guidance, got %v", err)
	}
}

func TestNativeAuthAddPreservesExistingOAuthClientID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clibox.db")
	t.Setenv("CLIBOX_GMAIL_CLIENT_ID", "client-id")
	if _, err := NativeAuthAdd(context.Background(), Options{StatePath: path, Account: "gmail"}, "person@gmail.com"); err != nil {
		t.Fatalf("expected first auth add to succeed: %v", err)
	}
	t.Setenv("CLIBOX_GMAIL_CLIENT_ID", "")
	if _, err := NativeAuthAdd(context.Background(), Options{StatePath: path, Account: "gmail"}, "person@gmail.com"); err != nil {
		t.Fatalf("expected second auth add to succeed: %v", err)
	}
	store, err := openNativeStore(path)
	if err != nil {
		t.Fatalf("expected store open: %v", err)
	}
	defer store.close()
	account, err := store.account(context.Background(), "gmail")
	if err != nil {
		t.Fatalf("expected account lookup: %v", err)
	}
	if account.OAuthClientID != "client-id" {
		t.Fatalf("expected OAuth client ID to be preserved, got %q", account.OAuthClientID)
	}
}
