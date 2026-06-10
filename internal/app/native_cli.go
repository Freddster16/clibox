package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

func NativeAuthAdd(ctx context.Context, options Options, email string) (string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if !validEmailAddress(email) {
		return "", errors.New("auth add needs --email you@example.com")
	}
	provider := detectProvider(email)
	accountName := sanitizeAccountName(firstNonEmpty(options.Account, provider.Account, accountNameFromDomain(emailDomain(email))), "personal")
	if !provider.canAutoConfigure() {
		return "", fmt.Errorf("%s needs manual server settings; native auth add cannot configure it yet", provider.Name)
	}

	store, err := openNativeStore(options.StatePath)
	if err != nil {
		return "", err
	}
	defer store.close()

	authType := "password"
	var scopes []string
	var clientID string
	if oauthProvider, ok := oauthProviderForEmail(email); ok {
		authType = "oauth2"
		scopes = oauthProvider.Scopes
		clientID = oauthClientID(oauthProvider)
	}
	account := nativeAccountFromProvider(accountName, email, displayNameFromEmail(email), provider, authType)
	if clientID == "" {
		if existing, err := store.account(ctx, account.Name); err == nil && strings.TrimSpace(existing.OAuthClientID) != "" {
			clientID = existing.OAuthClientID
		}
	}
	account.OAuthScopes = scopes
	account.OAuthClientID = clientID
	account.Mailbox = firstNonEmpty(options.Mailbox, account.Mailbox)
	account.ArchiveFolder = firstNonEmpty(options.ArchiveFolder, account.ArchiveFolder)
	account.Editor = strings.TrimSpace(options.Editor)
	if err := store.saveAccount(ctx, account); err != nil {
		return "", err
	}

	if authType == "oauth2" {
		next := "Next: set CLIBOX_" + strings.ToUpper(account.Provider) + "_CLIENT_ID, then run clibox auth login --account " + account.Name + "."
		if clientID != "" {
			next = "Next: run clibox auth login --account " + account.Name + "."
		}
		return fmt.Sprintf("Added native OAuth account %s <%s>.\n%s", account.Name, account.Email, next), nil
	}
	return fmt.Sprintf("Added native password account %s <%s>.\nOpen clibox, press A, and paste the app password so clibox can save it to your OS keychain.", account.Name, account.Email), nil
}

func NativeAuthLogin(ctx context.Context, options Options) (string, error) {
	store, err := openNativeStore(options.StatePath)
	if err != nil {
		return "", err
	}
	defer store.close()
	account, err := store.account(ctx, options.Account)
	if err != nil {
		return "", fmt.Errorf("%w; run clibox auth add --email you@gmail.com --account gmail first", err)
	}
	provider, ok := oauthProviderForEmail(account.Email)
	if !ok {
		return "", fmt.Errorf("%s does not support native OAuth in clibox yet", account.Email)
	}
	clientID := firstNonEmpty(account.OAuthClientID, oauthClientID(provider))
	if clientID == "" {
		return "", fmt.Errorf("%s OAuth client ID is missing; set CLIBOX_%s_CLIENT_ID to a desktop/native OAuth client ID, then retry", provider.Name, strings.ToUpper(provider.Key))
	}

	token, err := runLoopbackOAuth(ctx, provider, clientID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(token.RefreshToken) == "" {
		return "", errors.New("OAuth provider did not return a refresh token; revoke the test grant and retry with consent prompt")
	}
	account.AuthType = "oauth2"
	account.OAuthClientID = clientID
	account.OAuthScopes = provider.Scopes
	if err := store.saveAccount(ctx, account); err != nil {
		return "", err
	}
	if err := saveNativeSecret(account.Name, account.Email, storedSecretRefreshToken, token.RefreshToken); err != nil {
		return "", err
	}
	return fmt.Sprintf("OAuth login complete for %s <%s>. Run clibox sync --account %s --mailbox %s or open clibox.", account.Name, account.Email, account.Name, firstNonEmpty(options.Mailbox, account.Mailbox, "INBOX")), nil
}

func NativeAccountsReport(ctx context.Context, options Options) (string, error) {
	store, err := openNativeStore(options.StatePath)
	if err != nil {
		return "", err
	}
	defer store.close()
	accounts, err := store.listAccounts(ctx)
	if err != nil {
		return "", err
	}
	if len(accounts) == 0 {
		return "No native accounts configured.\nRun clibox auth add --email you@gmail.com --account gmail.", nil
	}
	lines := []string{"Native accounts:"}
	for _, account := range accounts {
		lines = append(lines, fmt.Sprintf("- %s <%s> provider=%s auth=%s mailbox=%s", account.Name, account.Email, account.Provider, account.AuthType, account.Mailbox))
	}
	return strings.Join(lines, "\n"), nil
}

func NativeSync(ctx context.Context, options Options) (string, error) {
	backend := newNativeBackend(options)
	total := 0
	for page := 1; ; page++ {
		messages, done, err := backend.ListEnvelopePage(ctx, page)
		if err != nil {
			return "", err
		}
		total += len(messages)
		if done {
			return fmt.Sprintf("Synced %d envelopes from %s.", total, backend.Label()), nil
		}
	}
}

func runLoopbackOAuth(ctx context.Context, provider oauthProvider, clientID string) (oauthToken, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return oauthToken{}, fmt.Errorf("could not start local OAuth callback server: %w", err)
	}
	defer listener.Close()

	redirectURI := "http://" + listener.Addr().String() + "/oauth/callback"
	auth, err := newOAuthAuthorization(provider, clientID, redirectURI)
	if err != nil {
		return oauthToken{}, err
	}

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	server := &http.Server{
		ReadHeaderTimeout: 10 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/oauth/callback" {
				http.NotFound(w, r)
				return
			}
			code, err := validateOAuthRedirect(redirectURI+"?"+r.URL.RawQuery, auth.State)
			if err != nil {
				http.Error(w, "clibox OAuth failed. Return to the terminal.", http.StatusBadRequest)
				errCh <- err
				return
			}
			fmt.Fprintln(w, "clibox OAuth login finished. You can close this tab and return to the terminal.")
			codeCh <- code
		}),
	}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	defer server.Shutdown(context.Background())

	if err := openURL(auth.URL); err != nil {
		return oauthToken{}, fmt.Errorf("could not open browser for OAuth login: %w", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	select {
	case code := <-codeCh:
		return exchangeOAuthCode(waitCtx, auth, code)
	case err := <-errCh:
		return oauthToken{}, err
	case <-waitCtx.Done():
		return oauthToken{}, errors.New("OAuth login timed out waiting for browser callback")
	}
}
