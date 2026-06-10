package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/zalando/go-keyring"
)

var (
	keyringSet = keyring.Set
	keyringGet = keyring.Get
)

type storedSecretKind string

const (
	storedSecretPassword storedSecretKind = "password"
	// #nosec G101 -- this is a keychain item label, not an embedded token value.
	storedSecretRefreshToken storedSecretKind = "oauth-refresh-token"
)

func saveNativeSecret(account, email string, kind storedSecretKind, secret string) error {
	account = sanitizeAccountName(account, "")
	email = strings.TrimSpace(strings.ToLower(email))
	secret = strings.TrimSpace(secret)
	if account == "" || email == "" {
		return errors.New("missing account or email for credential storage")
	}
	if secret == "" {
		return errors.New("missing credential value")
	}
	if err := keyringSet(nativeSecretService(account, kind), email, secret); err != nil {
		return fmt.Errorf("could not save credential to OS keychain: %w", err)
	}
	return nil
}

func loadNativeSecret(account, email string, kind storedSecretKind) (string, error) {
	account = sanitizeAccountName(account, "")
	email = strings.TrimSpace(strings.ToLower(email))
	if account == "" || email == "" {
		return "", errors.New("missing account or email for credential lookup")
	}
	secret, err := keyringGet(nativeSecretService(account, kind), email)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", setupRequiredError{detail: "missing native credential"}
		}
		return "", fmt.Errorf("could not read credential from OS keychain: %w", err)
	}
	if strings.TrimSpace(secret) == "" {
		return "", setupRequiredError{detail: "empty native credential"}
	}
	return secret, nil
}

func nativeSecretService(account string, kind storedSecretKind) string {
	return "clibox:" + sanitizeAccountName(account, "default") + ":" + string(kind)
}
