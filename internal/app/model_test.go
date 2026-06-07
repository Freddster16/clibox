package app

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestInboxNavigation(t *testing.T) {
	m := newTestModel()

	m = pressKey(t, m, "j")
	if m.cursor != 1 {
		t.Fatalf("expected cursor to move down to 1, got %d", m.cursor)
	}

	m = pressKey(t, m, "k")
	if m.cursor != 0 {
		t.Fatalf("expected cursor to move up to 0, got %d", m.cursor)
	}
}

func TestOpenReaderAndBack(t *testing.T) {
	m := newTestModel()
	if !m.messages[0].Unread {
		t.Fatal("expected first fake message to start unread")
	}

	m = pressKey(t, m, "enter")
	if m.mode != readerView {
		t.Fatalf("expected reader view, got %v", m.mode)
	}
	if m.messages[0].Unread {
		t.Fatal("expected opening a message to mark it read")
	}

	m = pressKey(t, m, "b")
	if m.mode != inboxView {
		t.Fatalf("expected inbox view, got %v", m.mode)
	}
}

func TestHelpOverlayConsumesNavigation(t *testing.T) {
	m := newTestModel()

	m = pressKey(t, m, "?")
	if !m.showHelp {
		t.Fatal("expected help overlay to open")
	}

	m = pressKey(t, m, "j")
	if m.cursor != 0 {
		t.Fatalf("expected navigation to be ignored while help is open, got %d", m.cursor)
	}
	if !m.showHelp {
		t.Fatal("expected help overlay to remain open after ignored key")
	}

	m = pressKey(t, m, "q")
	if m.showHelp {
		t.Fatal("expected q to close help overlay")
	}
	if m.cursor != 0 {
		t.Fatalf("expected q in help overlay not to move cursor, got %d", m.cursor)
	}
}

func TestQuitFromInbox(t *testing.T) {
	m := newTestModel()

	_, cmd := m.Update(keyMsg("q"))
	if cmd == nil {
		t.Fatal("expected q from inbox to return a quit command")
	}
}

func TestPlannedActionsShowStatus(t *testing.T) {
	m := newTestModel()

	m = pressKey(t, m, "a")
	if m.status == "" {
		t.Fatal("expected planned archive action to show status")
	}

	m = pressKey(t, m, "j")
	if m.status != "" {
		t.Fatal("expected navigation to clear status")
	}
}

func TestThemeKeyOpensThemePicker(t *testing.T) {
	t.Setenv("CLIBOX_THEME", "")
	m := newTestModel()

	m = pressKey(t, m, "t")
	if !m.showThemes {
		t.Fatal("expected t to open the theme picker")
	}
	if m.themeCursor != m.theme {
		t.Fatalf("expected theme cursor to start on active theme, got cursor %d and theme %d", m.themeCursor, m.theme)
	}
}

func TestSetupRequiredErrorOpensAccountSetupView(t *testing.T) {
	m := NewWithOptions(Options{backend: &configurableBackend{}})

	next, _ := m.Update(inboxLoadedMsg{err: setupRequiredError{}})
	updated := next.(model)
	if updated.mode != setupView {
		t.Fatalf("expected setup view, got %v", updated.mode)
	}
	if updated.setupAccount != "personal" {
		t.Fatalf("expected default setup account personal, got %q", updated.setupAccount)
	}
	if updated.setupStep != setupEmailStep {
		t.Fatalf("expected setup to start with email step, got %v", updated.setupStep)
	}
}

func TestSetupRequiredErrorWithKnownEmailOpensSecretStep(t *testing.T) {
	m := NewWithOptions(Options{backend: &configurableBackend{}})
	m.setupEmail = "freddy@gmail.com"
	m.setupAccount = "gmail"
	m.setupProvider = detectProvider(m.setupEmail)

	next, _ := m.Update(inboxLoadedMsg{err: setupRequiredError{}})
	updated := next.(model)
	if updated.mode != setupView {
		t.Fatalf("expected setup view, got %v", updated.mode)
	}
	if updated.setupStep != setupSecretStep {
		t.Fatalf("expected setup to start at secret step, got %v", updated.setupStep)
	}
}

func TestSetupEmailDetectsProviderAndAccountName(t *testing.T) {
	m := NewWithOptions(Options{backend: &configurableBackend{}})
	m.mode = setupView
	m.setupEmail = ""

	m = pressKey(t, m, "freddy@gmail.com")
	if m.setupEmail != "freddy@gmail.com" {
		t.Fatalf("expected email input to append runes, got %q", m.setupEmail)
	}

	m = pressKey(t, m, "backspace")
	if m.setupEmail != "freddy@gmail.co" {
		t.Fatalf("expected backspace to drop last email rune, got %q", m.setupEmail)
	}

	m = pressKey(t, m, "m")
	m = pressKey(t, m, "enter")
	if m.setupStep != setupReviewStep {
		t.Fatalf("expected valid email to advance to review, got %v", m.setupStep)
	}
	if m.setupProvider.Name != "Gmail" {
		t.Fatalf("expected Gmail provider, got %q", m.setupProvider.Name)
	}
	if m.setupAccount != "gmail" {
		t.Fatalf("expected Gmail account name, got %q", m.setupAccount)
	}
}

func TestAccountSetupEnterMovesToSecretStep(t *testing.T) {
	m := NewWithOptions(Options{backend: &configurableBackend{}})
	m.mode = setupView
	m.setupStep = setupReviewStep
	m.setupEmail = "freddy@gmail.com"
	m.setupProvider = detectProvider(m.setupEmail)
	m.setupAccount = "personal"

	next, cmd := m.Update(keyMsg("enter"))
	updated := next.(model)
	if updated.setupStep != setupSecretStep {
		t.Fatalf("expected setup enter to move to secret step, got %v", updated.setupStep)
	}
	if cmd != nil {
		t.Fatal("expected review enter not to launch a wizard command")
	}
}

func TestSecretStepStartsBackgroundSetup(t *testing.T) {
	backend := &configurableBackend{}
	m := NewWithOptions(Options{backend: backend})
	m.mode = setupView
	m.setupStep = setupSecretStep
	m.setupEmail = "freddy@gmail.com"
	m.setupProvider = detectProvider(m.setupEmail)
	m.setupAccount = "personal"

	m = pressKey(t, m, "abcd efgh ijkl mnop")
	next, cmd := m.Update(keyMsg("enter"))
	updated := next.(model)
	if !updated.configuring {
		t.Fatal("expected secret enter to mark model as configuring")
	}
	if cmd == nil {
		t.Fatal("expected secret enter to return background setup command")
	}
	msg := cmd()
	configured, ok := msg.(accountConfiguredMsg)
	if !ok {
		t.Fatalf("expected accountConfiguredMsg, got %T", msg)
	}
	if configured.err != nil {
		t.Fatalf("expected fake setup to succeed: %v", configured.err)
	}
	if backend.saved.Account != "personal" || backend.saved.Email != "freddy@gmail.com" {
		t.Fatalf("unexpected saved setup: %+v", backend.saved)
	}
	if backend.saved.Secret != "abcd efgh ijkl mnop" {
		t.Fatalf("expected secret to be captured, got %q", backend.saved.Secret)
	}
}

func TestSecretStepCanOpenProviderHelp(t *testing.T) {
	m := NewWithOptions(Options{backend: &configurableBackend{}})
	m.mode = setupView
	m.setupStep = setupSecretStep
	m.setupEmail = "freddy@gmail.com"
	m.setupProvider = detectProvider(m.setupEmail)
	m.setupAccount = "gmail"

	next, cmd := m.Update(keyMsg("ctrl+o"))
	updated := next.(model)
	if cmd == nil {
		t.Fatal("expected ctrl+o to return browser open command")
	}
	if !strings.Contains(updated.status, "opening Gmail setup") {
		t.Fatalf("expected opening status, got %q", updated.status)
	}
}

func TestAccountSetupCanEditAccountName(t *testing.T) {
	m := NewWithOptions(Options{backend: &configurableBackend{}})
	m.mode = setupView
	m.setupStep = setupReviewStep
	m.setupAccount = "gmail"

	m = pressKey(t, m, "n")
	if m.setupStep != setupAccountStep {
		t.Fatalf("expected n to switch to account edit step, got %v", m.setupStep)
	}

	m.setupAccount = ""
	m = pressKey(t, m, "work")
	m = pressKey(t, m, "enter")
	if m.setupStep != setupReviewStep {
		t.Fatalf("expected edited account to return to review, got %v", m.setupStep)
	}
	if m.setupAccount != "work" {
		t.Fatalf("expected edited account name work, got %q", m.setupAccount)
	}
}

func TestProviderReviewCanOpenBrowserHelp(t *testing.T) {
	m := NewWithOptions(Options{backend: &configurableBackend{}})
	m.mode = setupView
	m.setupStep = setupReviewStep
	m.setupEmail = "freddy@gmail.com"
	m.setupProvider = detectProvider(m.setupEmail)

	next, cmd := m.Update(keyMsg("o"))
	updated := next.(model)
	if cmd == nil {
		t.Fatal("expected o to return browser open command")
	}
	if !strings.Contains(updated.status, "opening Gmail setup") {
		t.Fatalf("expected opening status, got %q", updated.status)
	}
}

func TestAccountConfiguredReloadsInboxWithAccount(t *testing.T) {
	m := NewWithOptions(Options{backend: &configurableBackend{}})
	m.mode = setupView
	m.configuring = true

	next, cmd := m.Update(accountConfiguredMsg{account: "work"})
	updated := next.(model)
	if updated.mode != inboxView {
		t.Fatalf("expected inbox view after setup, got %v", updated.mode)
	}
	if updated.account != "work" {
		t.Fatalf("expected account to be updated, got %q", updated.account)
	}
	if !updated.loading {
		t.Fatal("expected inbox to reload after setup")
	}
	if cmd == nil {
		t.Fatal("expected setup completion to return reload command")
	}
}

func TestThemeCanBeSelectedFromEnvironment(t *testing.T) {
	t.Setenv("CLIBOX_THEME", "lagoon")

	m := New()
	if got := m.activeTheme().name; got != "Lagoon" {
		t.Fatalf("expected CLIBOX_THEME to select Lagoon, got %q", got)
	}
}

func TestCopperThemeAliasSelectsEmber(t *testing.T) {
	t.Setenv("CLIBOX_THEME", "copper")

	m := New()
	if got := m.activeTheme().name; got != "Ember" {
		t.Fatalf("expected copper alias to select Ember, got %q", got)
	}
}

func TestThemeCanBeSelectedFromConstructor(t *testing.T) {
	m := NewWithTheme("ember")
	if got := m.activeTheme().name; got != "Ember" {
		t.Fatalf("expected constructor theme Ember, got %q", got)
	}
}

func TestBlankConstructorThemeFallsBackToEnvironment(t *testing.T) {
	t.Setenv("CLIBOX_THEME", "lagoon")

	m := NewWithTheme("")
	if got := m.activeTheme().name; got != "Lagoon" {
		t.Fatalf("expected blank constructor theme to use environment, got %q", got)
	}
}

func TestUnknownThemeShowsFallbackStatus(t *testing.T) {
	m := NewWithTheme("banana")
	if got := m.activeTheme().name; got != "Nocturne" {
		t.Fatalf("expected unknown theme to fall back to Nocturne, got %q", got)
	}
	if !strings.Contains(m.status, `unknown theme "banana"`) {
		t.Fatalf("expected unknown theme status, got %q", m.status)
	}
}

func TestThemePickerPreviewsAndAppliesTheme(t *testing.T) {
	m := NewWithTheme("nocturne")

	m = pressKey(t, m, "t")
	m = pressKey(t, m, "j")
	if got := m.activeTheme().name; got != "Ember" {
		t.Fatalf("expected j in theme picker to preview Ember, got %q", got)
	}
	if !m.showThemes {
		t.Fatal("expected theme picker to stay open while previewing")
	}

	m = pressKey(t, m, "enter")
	if m.showThemes {
		t.Fatal("expected enter to close theme picker")
	}
	if got := m.activeTheme().name; got != "Ember" {
		t.Fatalf("expected enter to apply Ember, got %q", got)
	}
	if want := "theme Ember applied"; m.status != want {
		t.Fatalf("expected applied status %q, got %q", want, m.status)
	}
}

func TestThemePickerCanCancelPreview(t *testing.T) {
	m := NewWithTheme("nocturne")

	m = pressKey(t, m, "t")
	m = pressKey(t, m, "j")
	m = pressKey(t, m, "esc")
	if m.showThemes {
		t.Fatal("expected esc to close theme picker")
	}
	if got := m.activeTheme().name; got != "Nocturne" {
		t.Fatalf("expected esc to restore Nocturne, got %q", got)
	}
}

func TestThemePickerNumberAppliesTheme(t *testing.T) {
	m := NewWithTheme("nocturne")

	m = pressKey(t, m, "t")
	m = pressKey(t, m, "3")
	if got := m.activeTheme().name; got != "Lagoon" {
		t.Fatalf("expected number shortcut to apply Lagoon, got %q", got)
	}
	if m.showThemes {
		t.Fatal("expected number shortcut to close theme picker")
	}
}

func TestViewShowsThemeOnNarrowTerminal(t *testing.T) {
	m := NewWithTheme("lagoon")
	m.width = 34
	m.height = 10

	view := m.View()
	if !strings.Contains(view, "Lagoon") {
		t.Fatalf("expected narrow view to show active theme, got %q", view)
	}
}

func TestViewShowsThemePickerInsideTUI(t *testing.T) {
	m := NewWithTheme("lagoon")
	m.width = 72
	m.height = 22
	m = pressKey(t, m, "t")

	view := m.View()
	for _, want := range []string{"Themes", "Nocturne", "Ember", "Lagoon", "Enter applies"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected theme picker view to contain %q, got %q", want, view)
		}
	}
}

func TestThemeHelpListsCommands(t *testing.T) {
	help := ThemeHelp()
	for _, want := range []string{"nocturne", "ember", "lagoon", "clibox --theme lagoon", "theme picker"} {
		if !strings.Contains(help, want) {
			t.Fatalf("expected theme help to contain %q, got %q", want, help)
		}
	}
}

func TestThemesHaveDistinctVisibleSurfaces(t *testing.T) {
	seen := map[string]string{}
	for _, theme := range appThemes {
		signature := strings.Join([]string{
			theme.palette.background,
			theme.palette.header,
			theme.palette.surface,
			theme.palette.surfaceAlt,
			theme.palette.selected,
			theme.palette.footer,
		}, "|")
		if previous, ok := seen[signature]; ok {
			t.Fatalf("expected %s theme to differ from %s", theme.name, previous)
		}
		seen[signature] = theme.name
	}
}

func TestProviderDetectionGivesFriendlyGuidance(t *testing.T) {
	cases := map[string][]string{
		"freddy@gmail.com":      {"Gmail", "app password"},
		"freddy@icloud.com":     {"iCloud Mail", "app-specific password"},
		"freddy@outlook.com":    {"Outlook", "app password"},
		"freddy@yahoo.com":      {"Yahoo Mail", "app password"},
		"freddy@fastmail.com":   {"Fastmail", "app password"},
		"freddy@protonmail.com": {"Proton Mail", "Proton Mail Bridge"},
		"freddy@example.com":    {"Custom mail", "IMAP and SMTP"},
	}

	for email, wants := range cases {
		provider := detectProvider(email)
		combined := provider.Name + " " + provider.AuthSummary + " " + provider.SecretLabel + " " + provider.ManualWarning + " " + strings.Join(provider.Instructions, " ")
		for _, want := range wants {
			if !strings.Contains(combined, want) {
				t.Fatalf("expected provider guidance for %s to contain %q, got %+v", email, want, provider)
			}
		}
		if strings.Contains(provider.Name, "Gmail") && provider.HelpURL == "" {
			t.Fatal("expected Gmail to include browser setup URL")
		}
	}
}

func TestGmailSecretNormalizationRemovesSpaces(t *testing.T) {
	provider := detectProvider("freddy@gmail.com")
	got := provider.normalizeSecret(" abcd efgh ijkl mnop ")
	if got != "abcdefghijklmnop" {
		t.Fatalf("expected Gmail app password spaces to be removed, got %q", got)
	}
}

func TestValidEmailAddress(t *testing.T) {
	for _, email := range []string{"freddy@gmail.com", "work@example.co"} {
		if !validEmailAddress(email) {
			t.Fatalf("expected %q to be valid", email)
		}
	}
	for _, email := range []string{"", "freddy", "@example.com", "freddy@example", "fre ddy@example.com"} {
		if validEmailAddress(email) {
			t.Fatalf("expected %q to be invalid", email)
		}
	}
}

func pressKey(t *testing.T, m model, key string) model {
	t.Helper()

	next, _ := m.Update(keyMsg(key))
	updated, ok := next.(model)
	if !ok {
		t.Fatalf("expected model update to return app model, got %T", next)
	}
	return updated
}

func newTestModel() model {
	m := New()
	m.messages = testMessages()
	m.loading = false
	m.status = ""
	return m
}

func testMessages() []message {
	return []message{
		{
			ID:      "1",
			From:    "Alice",
			Email:   "alice@example.com",
			Subject: "Re: Design notes",
			Date:    "10:34 AM",
			Preview: "I looked at the prototype.",
			Body:    "Hey Freddy,\n\nI looked at the prototype and left notes.",
			Unread:  true,
		},
		{
			ID:      "2",
			From:    "GitHub",
			Email:   "notifications@github.com",
			Subject: "New issue assigned",
			Date:    "Yesterday",
			Preview: "You were assigned issue #42.",
			Body:    "You were assigned issue #42 in Freddster16/clibox.",
			Unread:  true,
		},
	}
}

type configurableBackend struct {
	account string
	saved   accountSetup
}

func (b configurableBackend) ListEnvelopes(context.Context) ([]message, error) {
	return testMessages(), nil
}

func (b configurableBackend) Label() string {
	return "fake " + b.account
}

func (b *configurableBackend) SaveAccountSetup(setup accountSetup) error {
	b.saved = setup
	return nil
}

func (b *configurableBackend) WithAccount(account string) inboxBackend {
	b.account = account
	return b
}

func keyMsg(key string) tea.KeyMsg {
	switch key {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case "delete":
		return tea.KeyMsg{Type: tea.KeyDelete}
	case "ctrl+o":
		return tea.KeyMsg{Type: tea.KeyCtrlO}
	case "ctrl+u":
		return tea.KeyMsg{Type: tea.KeyCtrlU}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}
