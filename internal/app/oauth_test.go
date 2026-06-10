package app

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestNewOAuthAuthorizationUsesPKCEAndState(t *testing.T) {
	provider := gmailOAuthProvider()
	auth, err := newOAuthAuthorization(provider, "client-id", "http://127.0.0.1:1234/oauth/callback")
	if err != nil {
		t.Fatalf("expected authorization to build: %v", err)
	}
	parsed, err := url.Parse(auth.URL)
	if err != nil {
		t.Fatalf("expected auth URL to parse: %v", err)
	}
	query := parsed.Query()
	if query.Get("code_challenge_method") != "S256" {
		t.Fatalf("expected S256 PKCE challenge, got %q", query.Get("code_challenge_method"))
	}
	sum := sha256.Sum256([]byte(auth.CodeVerifier))
	wantChallenge := base64.RawURLEncoding.EncodeToString(sum[:])
	if auth.CodeChallenge != wantChallenge || query.Get("code_challenge") != wantChallenge {
		t.Fatalf("PKCE challenge did not match verifier")
	}
	if auth.State == "" || query.Get("state") != auth.State {
		t.Fatalf("expected state in authorization URL")
	}
	if !strings.Contains(query.Get("scope"), "https://mail.google.com/") {
		t.Fatalf("expected Gmail mail scope, got %q", query.Get("scope"))
	}
}

func TestValidateOAuthRedirectRejectsMismatchedState(t *testing.T) {
	_, err := validateOAuthRedirect("http://127.0.0.1/callback?code=abc&state=wrong", "expected")
	if err == nil || !strings.Contains(err.Error(), "state") {
		t.Fatalf("expected state mismatch, got %v", err)
	}
}

func TestExchangeOAuthCodeSendsPublicClientPKCERequest(t *testing.T) {
	var form url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("expected form to parse: %v", err)
		}
		form = r.Form
		if r.Form.Get("client_secret") != "" {
			t.Fatal("public native OAuth flow must not send a client_secret")
		}
		_ = json.NewEncoder(w).Encode(oauthToken{
			AccessToken:  "access",
			RefreshToken: "refresh",
			TokenType:    "Bearer",
			ExpiresIn:    3600,
		})
	}))
	defer server.Close()

	oldClient := oauthHTTPClient
	oauthHTTPClient = server.Client()
	defer func() { oauthHTTPClient = oldClient }()

	auth := oauthAuthorization{
		Provider:     oauthProvider{Name: "Test", TokenURL: server.URL},
		ClientID:     "client-id",
		RedirectURI:  "http://127.0.0.1/callback",
		CodeVerifier: "verifier",
	}
	token, err := exchangeOAuthCode(context.Background(), auth, "code")
	if err != nil {
		t.Fatalf("expected token exchange to succeed: %v", err)
	}
	if token.AccessToken != "access" || token.RefreshToken != "refresh" {
		t.Fatalf("unexpected token: %+v", token)
	}
	for key, want := range map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     "client-id",
		"redirect_uri":  "http://127.0.0.1/callback",
		"code":          "code",
		"code_verifier": "verifier",
	} {
		if got := form.Get(key); got != want {
			t.Fatalf("expected %s=%q, got %q", key, want, got)
		}
	}
}
