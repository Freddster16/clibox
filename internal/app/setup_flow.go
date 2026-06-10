package app

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) updateSetup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.configuring {
		if key == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil
	}

	switch key {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "q":
		if m.setupStep == setupReviewStep {
			m.setupStep = setupEmailStep
			return m.withStatus("edit your email address, then press Enter"), nil
		}
		if m.setupStep == setupSecretStep {
			m.setupStep = setupReviewStep
			return m.withStatus("review setup, then press Enter"), nil
		}
		if m.setupStep == setupAccountStep {
			m.setupStep = setupReviewStep
			return m.withStatus("review setup, then press Enter"), nil
		}
		if len(m.messages) == 0 {
			return m, tea.Quit
		}
		m.mode = inboxView
		return m.withStatus("account setup canceled"), nil
	}

	switch m.setupStep {
	case setupEmailStep:
		return m.updateSetupEmail(msg)
	case setupSecretStep:
		return m.updateSetupSecret(msg)
	case setupAccountStep:
		return m.updateSetupAccount(msg)
	default:
		return m.updateSetupReview(msg)
	}
}

func (m model) updateSetupEmail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		email := strings.TrimSpace(m.setupEmail)
		if !validEmailAddress(email) {
			return m.withStatus("type a valid email address first"), nil
		}
		m.setupEmail = email
		m.setupProvider = detectProvider(email)
		if strings.TrimSpace(m.account) != "" {
			m.setupAccount = sanitizeAccountName(m.account, m.setupProvider.Account)
		} else {
			m.setupAccount = firstNonEmpty(m.setupProvider.Account, accountNameFromDomain(emailDomain(email)), "personal")
		}
		m.setupStep = setupReviewStep
		return m.withStatus(m.setupProvider.Name + " detected; review setup, then press Enter"), nil
	case "backspace", "ctrl+h":
		m.setupEmail = dropLastRune(m.setupEmail)
		return m.withStatus("type your email address, then press Enter"), nil
	case "delete":
		return m, nil
	}

	if len(msg.Runes) > 0 {
		for _, r := range msg.Runes {
			if isEmailRune(r) {
				m.setupEmail += string(r)
			}
		}
		return m.withStatus("type your email address, then press Enter"), nil
	}
	return m, nil
}

func (m model) updateSetupReview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		provider := m.setupProvider
		if provider.Name == "" {
			provider = detectProvider(m.setupEmail)
		}
		if !provider.canAutoConfigure() {
			return m.withStatus(provider.Name + " needs manual server settings before automatic setup can run"), nil
		}
		m.setupProvider = provider
		if _, ok := m.backend.(oauthAccountSetupBackend); ok && providerNeedsOAuth(provider) {
			return m.startOAuthAccountConfiguration()
		}
		m.setupStep = setupSecretStep
		return m.withStatus("paste your " + strings.ToLower(provider.secretLabel()) + ", then press Enter"), nil
	case "o":
		provider := m.setupProvider
		if provider.Name == "" {
			provider = detectProvider(m.setupEmail)
		}
		if strings.TrimSpace(provider.HelpURL) == "" {
			return m.withStatus("no browser setup link for " + provider.Name), nil
		}
		m.status = "opening " + provider.Name + " setup in your browser..."
		return m, func() tea.Msg {
			return providerHelpOpenedMsg{provider: provider.Name, err: openURL(provider.HelpURL)}
		}
	case "e":
		m.setupStep = setupEmailStep
		return m.withStatus("edit your email address, then press Enter"), nil
	case "n":
		m.setupStep = setupAccountStep
		return m.withStatus("edit the local account name, then press Enter"), nil
	}
	return m, nil
}

func (m model) updateSetupSecret(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if strings.TrimSpace(m.setupSecret) == "" {
			provider := m.setupProvider
			if provider.Name == "" {
				provider = detectProvider(m.setupEmail)
			}
			return m.withStatus("paste your " + strings.ToLower(provider.secretLabel()) + " first"), nil
		}
		return m.startAccountConfiguration()
	case "backspace", "ctrl+h":
		m.setupSecret = dropLastRune(m.setupSecret)
		provider := m.setupProvider
		if provider.Name == "" {
			provider = detectProvider(m.setupEmail)
		}
		return m.withStatus("paste your " + strings.ToLower(provider.secretLabel()) + ", then press Enter"), nil
	case "delete":
		return m, nil
	case "ctrl+u":
		m.setupSecret = ""
		return m.withStatus("password cleared"), nil
	case "ctrl+o":
		provider := m.setupProvider
		if provider.Name == "" {
			provider = detectProvider(m.setupEmail)
		}
		if strings.TrimSpace(provider.HelpURL) == "" {
			return m.withStatus("no browser setup link for " + provider.Name), nil
		}
		m.status = "opening " + provider.Name + " setup in your browser..."
		return m, func() tea.Msg {
			return providerHelpOpenedMsg{provider: provider.Name, err: openURL(provider.HelpURL)}
		}
	}

	if len(msg.Runes) > 0 {
		for _, r := range msg.Runes {
			if r >= 32 && r != 127 {
				m.setupSecret += string(r)
			}
		}
		provider := m.setupProvider
		if provider.Name == "" {
			provider = detectProvider(m.setupEmail)
		}
		return m.withStatus("paste your " + strings.ToLower(provider.secretLabel()) + ", then press Enter"), nil
	}
	return m, nil
}

func (m model) updateSetupAccount(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		account := sanitizeAccountName(m.setupAccount, "")
		if account == "" {
			return m.withStatus("type an account name first"), nil
		}
		m.setupAccount = account
		m.setupStep = setupReviewStep
		return m.withStatus("review setup, then press Enter"), nil
	case "backspace", "ctrl+h":
		m.setupAccount = dropLastRune(m.setupAccount)
		return m.withStatus("edit the local account name, then press Enter"), nil
	case "delete":
		return m, nil
	}

	if len(msg.Runes) > 0 {
		for _, r := range msg.Runes {
			if isAccountNameRune(r) {
				m.setupAccount += string(r)
			}
		}
		return m.withStatus("edit the local account name, then press Enter"), nil
	}
	return m, nil
}

func (m model) startAccountConfiguration() (tea.Model, tea.Cmd) {
	account := sanitizeAccountName(m.setupAccount, "")
	if account == "" {
		return m.withStatus("type an account name first"), nil
	}
	backend, ok := m.backend.(accountSetupBackend)
	if !ok {
		return m.withStatus("this backend cannot configure accounts"), nil
	}
	m.setupAccount = account
	m.configuring = true
	provider := m.setupProvider
	if provider.Name == "" {
		provider = detectProvider(m.setupEmail)
	}
	setup := accountSetup{
		Account:     account,
		Email:       m.setupEmail,
		DisplayName: displayNameFromEmail(m.setupEmail),
		Provider:    provider,
		Secret:      m.setupSecret,
	}
	m.status = "configuring " + provider.Name + " in the background..."
	return m, func() tea.Msg {
		err := backend.SaveAccountSetup(setup)
		return accountConfiguredMsg{account: account, err: err}
	}
}

func (m model) startOAuthAccountConfiguration() (tea.Model, tea.Cmd) {
	account := sanitizeAccountName(m.setupAccount, "")
	if account == "" {
		return m.withStatus("type an account name first"), nil
	}
	backend, ok := m.backend.(oauthAccountSetupBackend)
	if !ok {
		return m.withStatus("this backend cannot configure browser OAuth accounts"), nil
	}
	m.setupAccount = account
	m.configuring = true
	provider := m.setupProvider
	if provider.Name == "" {
		provider = detectProvider(m.setupEmail)
	}
	setup := accountSetup{
		Account:     account,
		Email:       m.setupEmail,
		DisplayName: displayNameFromEmail(m.setupEmail),
		Provider:    provider,
	}
	m.status = "opening " + provider.Name + " browser login..."
	return m, func() tea.Msg {
		err := backend.SaveOAuthAccountSetup(context.Background(), setup)
		return accountConfiguredMsg{account: account, err: err}
	}
}
