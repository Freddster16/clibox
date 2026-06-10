package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type oauthProvider struct {
	Name         string
	Key          string
	AuthURL      string
	TokenURL     string
	Scopes       []string
	IMAPHost     string
	IMAPPort     int
	SMTPHost     string
	SMTPPort     int
	ArchiveLabel string
	TrashLabel   string
}

type oauthAuthorization struct {
	Provider      oauthProvider
	ClientID      string
	RedirectURI   string
	State         string
	CodeVerifier  string
	CodeChallenge string
	URL           string
}

type oauthToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

type oauthErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

var oauthHTTPClient = &http.Client{Timeout: 20 * time.Second}

func oauthProviderForEmail(email string) (oauthProvider, bool) {
	switch emailDomain(email) {
	case "gmail.com", "googlemail.com":
		return gmailOAuthProvider(), true
	case "outlook.com", "hotmail.com", "live.com", "msn.com":
		return outlookOAuthProvider(), true
	default:
		return oauthProvider{}, false
	}
}

func gmailOAuthProvider() oauthProvider {
	// #nosec G101 -- OAuth endpoints and mail scopes are provider metadata, not credentials.
	return oauthProvider{
		Name:         "Gmail",
		Key:          "gmail",
		AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:     "https://oauth2.googleapis.com/token",
		Scopes:       []string{"https://mail.google.com/"},
		IMAPHost:     "imap.gmail.com",
		IMAPPort:     993,
		SMTPHost:     "smtp.gmail.com",
		SMTPPort:     587,
		ArchiveLabel: "[Gmail]/All Mail",
		TrashLabel:   "[Gmail]/Trash",
	}
}

func outlookOAuthProvider() oauthProvider {
	// #nosec G101 -- OAuth endpoints and mail scopes are provider metadata, not credentials.
	return oauthProvider{
		Name:     "Outlook",
		Key:      "outlook",
		AuthURL:  "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
		TokenURL: "https://login.microsoftonline.com/common/oauth2/v2.0/token",
		Scopes: []string{
			"https://outlook.office.com/IMAP.AccessAsUser.All",
			"https://outlook.office.com/SMTP.Send",
			"offline_access",
		},
		IMAPHost:     "outlook.office365.com",
		IMAPPort:     993,
		SMTPHost:     "smtp-mail.outlook.com",
		SMTPPort:     587,
		ArchiveLabel: "Archive",
		TrashLabel:   "Deleted Items",
	}
}

func newOAuthAuthorization(provider oauthProvider, clientID, redirectURI string) (oauthAuthorization, error) {
	clientID = strings.TrimSpace(clientID)
	redirectURI = strings.TrimSpace(redirectURI)
	if clientID == "" {
		return oauthAuthorization{}, fmt.Errorf("%s OAuth client ID is not configured", provider.Name)
	}
	if redirectURI == "" {
		return oauthAuthorization{}, errors.New("missing OAuth redirect URI")
	}
	verifier, challenge, err := newPKCEPair()
	if err != nil {
		return oauthAuthorization{}, err
	}
	state, err := randomURLToken(32)
	if err != nil {
		return oauthAuthorization{}, err
	}

	query := url.Values{}
	query.Set("client_id", clientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("response_type", "code")
	query.Set("scope", strings.Join(provider.Scopes, " "))
	query.Set("state", state)
	query.Set("code_challenge", challenge)
	query.Set("code_challenge_method", "S256")
	query.Set("access_type", "offline")
	query.Set("prompt", "consent")

	authURL := provider.AuthURL
	if strings.Contains(authURL, "?") {
		authURL += "&" + query.Encode()
	} else {
		authURL += "?" + query.Encode()
	}
	return oauthAuthorization{
		Provider:      provider,
		ClientID:      clientID,
		RedirectURI:   redirectURI,
		State:         state,
		CodeVerifier:  verifier,
		CodeChallenge: challenge,
		URL:           authURL,
	}, nil
}

func newPKCEPair() (verifier, challenge string, err error) {
	verifier, err = randomURLToken(64)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

func randomURLToken(byteCount int) (string, error) {
	if byteCount <= 0 {
		byteCount = 32
	}
	raw := make([]byte, byteCount)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("could not generate secure random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func validateOAuthRedirect(rawURL, expectedState string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid OAuth redirect: %w", err)
	}
	if got := parsed.Query().Get("state"); got == "" || got != expectedState {
		return "", errors.New("OAuth redirect state did not match")
	}
	if oauthErr := parsed.Query().Get("error"); oauthErr != "" {
		return "", fmt.Errorf("OAuth provider rejected login: %s", oauthErr)
	}
	code := strings.TrimSpace(parsed.Query().Get("code"))
	if code == "" {
		return "", errors.New("OAuth redirect did not include an authorization code")
	}
	return code, nil
}

func exchangeOAuthCode(ctx context.Context, auth oauthAuthorization, code string) (oauthToken, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", auth.ClientID)
	form.Set("code", strings.TrimSpace(code))
	form.Set("redirect_uri", auth.RedirectURI)
	form.Set("code_verifier", auth.CodeVerifier)
	return requestOAuthToken(ctx, auth.Provider.TokenURL, form)
}

func refreshOAuthAccessToken(ctx context.Context, provider oauthProvider, clientID, refreshToken string) (oauthToken, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", strings.TrimSpace(clientID))
	form.Set("refresh_token", strings.TrimSpace(refreshToken))
	return requestOAuthToken(ctx, provider.TokenURL, form)
}

func requestOAuthToken(ctx context.Context, endpoint string, form url.Values) (oauthToken, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return oauthToken{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := oauthHTTPClient.Do(req)
	if err != nil {
		return oauthToken{}, fmt.Errorf("OAuth token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return oauthToken{}, fmt.Errorf("OAuth token response could not be read: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var oauthErr oauthErrorResponse
		_ = json.Unmarshal(body, &oauthErr)
		detail := firstNonEmpty(oauthErr.ErrorDescription, oauthErr.Error, string(body), resp.Status)
		return oauthToken{}, fmt.Errorf("OAuth token request failed: %s", oneLine(detail))
	}

	var token oauthToken
	if err := json.Unmarshal(body, &token); err != nil {
		return oauthToken{}, fmt.Errorf("OAuth token response was unreadable: %w", err)
	}
	if strings.TrimSpace(token.AccessToken) == "" {
		return oauthToken{}, errors.New("OAuth token response did not include an access token")
	}
	return token, nil
}

func providerNeedsOAuth(provider providerInfo) bool {
	switch provider.Name {
	case "Gmail", "Outlook":
		return true
	default:
		return false
	}
}

func oauthClientID(provider oauthProvider) string {
	envKey := "CLIBOX_" + strings.ToUpper(provider.Key) + "_CLIENT_ID"
	return strings.TrimSpace(os.Getenv(envKey))
}
