package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m model) renderSetup(height int) string {
	styles := m.activeTheme().styles
	width := max(32, m.width)
	if m.configuring {
		lines := []string{
			styles.panelTitle.Render("Email setup"),
			"",
			styles.readerBody.Width(width).Render("clibox is saving your account settings and password to your OS credential store."),
			styles.readerBody.Width(width).Render("Your inbox will reload automatically when setup finishes."),
		}
		return fitHeight(strings.Join(lines, "\n"), height)
	}

	switch m.setupStep {
	case setupReviewStep:
		return m.renderSetupReview(width, height)
	case setupSecretStep:
		return m.renderSetupSecret(width, height)
	case setupAccountStep:
		return m.renderSetupAccount(width, height)
	default:
		return m.renderSetupEmail(width, height)
	}
}

func (m model) renderSetupEmail(width, height int) string {
	styles := m.activeTheme().styles
	email := m.setupEmail + "_"
	lines := []string{
		styles.panelTitle.Render("Add email account"),
		"",
		styles.readerBody.Width(width).Render("Start with your email address. clibox will detect the provider, choose the mail servers, and set up your account without sending you through another wizard."),
		"",
		styles.readerHeader.Width(width).Render("Email address"),
		styles.selected.Width(min(width, max(30, lipgloss.Width(email)+2))).Render(" " + email),
		"",
		styles.readerBody.Width(width).Render("Examples: you@gmail.com, you@icloud.com, work@company.com"),
		"",
		styles.readerBody.Width(width).Render("Enter continues. q quits."),
	}
	return fitHeight(strings.Join(lines, "\n"), height)
}

func (m model) renderSetupReview(width, height int) string {
	styles := m.activeTheme().styles
	provider := m.setupProvider
	if provider.Name == "" {
		provider = detectProvider(m.setupEmail)
	}

	lines := []string{
		styles.panelTitle.Render("Review account setup"),
		"",
		styles.readerHeader.Width(width).Render("Email: " + m.setupEmail),
		styles.readerHeader.Width(width).Render("Provider: " + provider.Name),
		styles.readerHeader.Width(width).Render("Account name: " + m.setupAccount),
		"",
	}
	lines = append(lines, styledLines(wrapText(provider.AuthSummary, width-2), styles.readerBody, width)...)
	if provider.ManualWarning != "" {
		lines = append(lines, "")
		lines = append(lines, styledLines(wrapText(provider.ManualWarning, width-2), styles.unread, width)...)
	}
	lines = append(lines, "")
	for _, instruction := range provider.Instructions {
		lines = append(lines, styledLines(wrapText("- "+instruction, width-2), styles.readerBody, width)...)
	}
	lines = append(lines, "")
	if provider.HelpURL != "" {
		lines = append(lines,
			styles.readerHeader.Width(width).Render(provider.HelpLabel),
			styles.readerBody.Width(width).Render(provider.HelpURL),
			styles.readerBody.Width(width).Render("Click the link if your terminal supports it, or press o to open it."),
		)
	}
	if _, ok := m.backend.(oauthAccountSetupBackend); ok && providerNeedsOAuth(provider) {
		lines = append(lines, styles.readerBody.Width(width).Render("Enter opens browser login. e edits email. n edits account name."))
	} else if provider.canAutoConfigure() {
		lines = append(lines, styles.readerBody.Width(width).Render("Enter continues to password setup. e edits email. n edits account name."))
	} else {
		lines = append(lines, styles.readerBody.Width(width).Render("Automatic setup for this provider needs manual server settings first. e edits email. n edits account name."))
	}
	return fitHeight(strings.Join(lines, "\n"), height)
}

func (m model) renderSetupSecret(width, height int) string {
	styles := m.activeTheme().styles
	provider := m.setupProvider
	if provider.Name == "" {
		provider = detectProvider(m.setupEmail)
	}
	mask := strings.Repeat("*", len([]rune(m.setupSecret)))
	if mask == "" {
		mask = "_"
	}

	lines := []string{
		styles.panelTitle.Render("Connect " + provider.Name),
		"",
		styles.readerHeader.Width(width).Render("Email: " + m.setupEmail),
		styles.readerHeader.Width(width).Render("Account name: " + m.setupAccount),
		"",
	}
	prompt := "Paste your " + provider.secretLabel() + ". clibox will save it to your OS credential store, configure your mail connection, and reload your inbox."
	if provider.Name == "Gmail" {
		prompt = "Paste the 16-character Google app password, not your Gmail address or normal Google password. clibox will save it to your OS credential store, configure your mail connection, and reload your inbox."
	}
	lines = append(lines, styledLines(wrapText(prompt, width-2), styles.readerBody, width)...)
	if provider.HelpURL != "" {
		lines = append(lines, "")
		lines = append(lines,
			styles.readerHeader.Width(width).Render(provider.HelpLabel),
			styles.readerBody.Width(width).Render(provider.HelpURL),
			styles.readerBody.Width(width).Render("Click the link if your terminal supports it, or press Ctrl+O to open it. Esc returns."),
		)
	}
	lines = append(lines,
		"",
		styles.readerHeader.Width(width).Render(provider.secretLabel()),
		styles.selected.Width(min(width, max(30, lipgloss.Width(mask)+2))).Render(" "+mask),
		"",
		styles.readerBody.Width(width).Render("Enter saves setup. Ctrl+O opens help. Ctrl+U clears. Esc returns."),
	)
	return fitHeight(strings.Join(lines, "\n"), height)
}

func (m model) renderSetupAccount(width, height int) string {
	styles := m.activeTheme().styles
	account := m.setupAccount + "_"
	lines := []string{
		styles.panelTitle.Render("Account name"),
		"",
		styles.readerBody.Width(width).Render("This is a short local name for the account inside clibox."),
		"",
		styles.readerHeader.Width(width).Render("Account name"),
		styles.selected.Width(min(width, max(24, lipgloss.Width(account)+2))).Render(" " + account),
		"",
		styles.readerBody.Width(width).Render("Examples: personal, work, gmail"),
		"",
		styles.readerBody.Width(width).Render("Enter returns to review. Esc returns without changing screens."),
	}
	return fitHeight(strings.Join(lines, "\n"), height)
}

func (m model) setupFooterHints() string {
	if m.configuring {
		return "finish account setup"
	}
	switch m.setupStep {
	case setupReviewStep:
		provider := m.setupProvider
		if provider.Name == "" {
			provider = detectProvider(m.setupEmail)
		}
		if _, ok := m.backend.(oauthAccountSetupBackend); ok && providerNeedsOAuth(provider) {
			return "enter browser login  o provider page  e edit email  n edit account name  q back"
		}
		return "o open provider page  enter setup  e edit email  n edit account name  q back"
	case setupAccountStep:
		return "type account name  enter review  backspace edit  q back"
	default:
		return "type email address  enter continue  backspace edit  q quit"
	}
}
