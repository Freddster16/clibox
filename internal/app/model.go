package app

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type viewMode int

const (
	inboxView viewMode = iota
	readerView
	setupView
)

type Options struct {
	Theme    string
	Account  string
	Mailbox  string
	Himalaya string
	PageSize int
	backend  inboxBackend
}

type inboxLoadedMsg struct {
	messages []message
	err      error
}

type inboxPageLoadedMsg struct {
	messages []message
	page     int
	serial   int
	done     bool
	err      error
}

type accountConfiguredMsg struct {
	account string
	err     error
}

type providerHelpOpenedMsg struct {
	provider string
	err      error
}

type model struct {
	messages          []message
	backend           inboxBackend
	cursor            int
	mode              viewMode
	showHelp          bool
	showThemes        bool
	loading           bool
	loadingMore       bool
	loadedAll         bool
	loadSerial        int
	status            string
	account           string
	mailbox           string
	setupEmail        string
	setupAccount      string
	setupSecret       string
	setupProvider     providerInfo
	setupStep         setupStep
	configuring       bool
	theme             int
	themeCursor       int
	themeBeforePicker int
	width             int
	height            int
}

type styles struct {
	screen       lipgloss.Style
	header       lipgloss.Style
	row          lipgloss.Style
	rowAlt       lipgloss.Style
	title        lipgloss.Style
	subtitle     lipgloss.Style
	panelTitle   lipgloss.Style
	selected     lipgloss.Style
	unread       lipgloss.Style
	muted        lipgloss.Style
	footer       lipgloss.Style
	helpPanel    lipgloss.Style
	readerHeader lipgloss.Style
	readerBody   lipgloss.Style
	themeBadge   lipgloss.Style
}

type appTheme struct {
	name        string
	description string
	palette     palette
	styles      styles
}

type palette struct {
	background     string
	header         string
	surface        string
	surfaceAlt     string
	accent         string
	accentText     string
	accentSoft     string
	text           string
	muted          string
	selected       string
	selectedText   string
	unread         string
	border         string
	footer         string
	footerText     string
	readerHeader   string
	readerHeaderFg string
}

var appThemes = []appTheme{
	newTheme("Nocturne", "violet header, cyan accents, dark blue surfaces", palette{
		background:     "#09090f",
		header:         "#312e81",
		surface:        "#111827",
		surfaceAlt:     "#0f172a",
		accent:         "#d946ef",
		accentText:     "#fdf4ff",
		accentSoft:     "#67e8f9",
		text:           "#f8fafc",
		muted:          "#94a3b8",
		selected:       "#7c3aed",
		selectedText:   "#ffffff",
		unread:         "#a7f3d0",
		border:         "#22d3ee",
		footer:         "#1e1b4b",
		footerText:     "#e0e7ff",
		readerHeader:   "#3730a3",
		readerHeaderFg: "#eef2ff",
	}),
	newTheme("Ember", "orange header, gold accents, warm dark surfaces", palette{
		background:     "#120b07",
		header:         "#7c2d12",
		surface:        "#26150d",
		surfaceAlt:     "#321a0f",
		accent:         "#fb923c",
		accentText:     "#1c0a00",
		accentSoft:     "#facc15",
		text:           "#fff7ed",
		muted:          "#d6a57e",
		selected:       "#f97316",
		selectedText:   "#1c0a00",
		unread:         "#fdba74",
		border:         "#ea580c",
		footer:         "#431407",
		footerText:     "#ffedd5",
		readerHeader:   "#9a3412",
		readerHeaderFg: "#fff7ed",
	}),
	newTheme("Lagoon", "teal header, seafoam accents, deep green surfaces", palette{
		background:     "#031b1f",
		header:         "#155e75",
		surface:        "#07343a",
		surfaceAlt:     "#092d33",
		accent:         "#2dd4bf",
		accentText:     "#042f2e",
		accentSoft:     "#a7f3d0",
		text:           "#ecfeff",
		muted:          "#8fc8d1",
		selected:       "#14b8a6",
		selectedText:   "#042f2e",
		unread:         "#5eead4",
		border:         "#06b6d4",
		footer:         "#083344",
		footerText:     "#cffafe",
		readerHeader:   "#0f766e",
		readerHeaderFg: "#ecfeff",
	}),
}

func newTheme(name, description string, p palette) appTheme {
	return appTheme{
		name:        name,
		description: description,
		palette:     p,
		styles: styles{
			screen: lipgloss.NewStyle().
				Foreground(lipgloss.Color(p.text)).
				Background(lipgloss.Color(p.background)),
			header: lipgloss.NewStyle().
				Foreground(lipgloss.Color(p.text)).
				Background(lipgloss.Color(p.header)),
			row: lipgloss.NewStyle().
				Foreground(lipgloss.Color(p.text)).
				Background(lipgloss.Color(p.surface)),
			rowAlt: lipgloss.NewStyle().
				Foreground(lipgloss.Color(p.text)).
				Background(lipgloss.Color(p.surfaceAlt)),
			title: lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(p.accentText)).
				Background(lipgloss.Color(p.accent)).
				Padding(0, 1),
			subtitle: lipgloss.NewStyle().
				Foreground(lipgloss.Color(p.muted)),
			panelTitle: lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(p.accentSoft)),
			selected: lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(p.selectedText)).
				Background(lipgloss.Color(p.selected)),
			unread: lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(p.unread)).
				Background(lipgloss.Color(p.surfaceAlt)),
			muted: lipgloss.NewStyle().
				Foreground(lipgloss.Color(p.muted)),
			footer: lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(p.footerText)).
				Background(lipgloss.Color(p.footer)).
				Padding(0, 1),
			helpPanel: lipgloss.NewStyle().
				Foreground(lipgloss.Color(p.text)).
				Background(lipgloss.Color(p.surface)).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(p.border)).
				Padding(1, 2),
			readerHeader: lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(p.readerHeaderFg)).
				Background(lipgloss.Color(p.readerHeader)).
				Padding(0, 1),
			readerBody: lipgloss.NewStyle().
				Foreground(lipgloss.Color(p.text)).
				Background(lipgloss.Color(p.surface)),
			themeBadge: lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(p.accentText)).
				Background(lipgloss.Color(p.accent)).
				Padding(0, 1),
		},
	}
}

func New() model {
	return NewWithOptions(Options{})
}

func NewWithTheme(name string) model {
	return NewWithOptions(Options{Theme: name})
}

func NewWithOptions(options Options) model {
	selected := strings.TrimSpace(options.Theme)
	if selected == "" {
		selected = os.Getenv("CLIBOX_THEME")
	}

	index, ok := themeIndex(selected)
	status := fmt.Sprintf("theme %s active; press t to choose", appThemes[index].name)
	if strings.TrimSpace(selected) != "" && !ok {
		status = fmt.Sprintf("unknown theme %q; using %s", selected, appThemes[index].name)
	}

	backend := options.backend
	var hint accountSetup
	if backend == nil {
		himalaya := newHimalayaBackend(options)
		backend = himalaya
		if options.Mailbox == "" {
			options.Mailbox = himalaya.mailbox
		}
		hint, _ = himalayaAccountHint(himalaya.account)
	}
	setupEmail := strings.TrimSpace(hint.Email)
	setupAccount := firstNonEmpty(options.Account, hint.Account, "personal")
	setupProvider := hint.Provider

	return model{
		backend:           backend,
		loading:           true,
		status:            status,
		account:           strings.TrimSpace(options.Account),
		mailbox:           strings.TrimSpace(options.Mailbox),
		setupEmail:        setupEmail,
		setupAccount:      setupAccount,
		setupProvider:     setupProvider,
		setupStep:         setupEmailStep,
		theme:             index,
		themeCursor:       index,
		themeBeforePicker: index,
	}
}

func ThemeHelp() string {
	var lines []string
	for _, theme := range appThemes {
		lines = append(lines, fmt.Sprintf("  %-9s %s", strings.ToLower(theme.name), theme.description))
	}

	return fmt.Sprintf(`Available clibox themes:
%s

Start with a theme:
  clibox --theme lagoon

Inside clibox:
  press t to open the theme picker
`, strings.Join(lines, "\n"))
}

func (m model) Init() tea.Cmd {
	return m.loadInbox()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case inboxPageLoadedMsg:
		if msg.serial != m.loadSerial {
			return m, nil
		}
		m.loading = false
		m.loadingMore = false
		if msg.err != nil {
			if msg.page == 1 && isSetupRequiredError(msg.err) {
				m.mode = setupView
				m.messages = nil
				m.cursor = 0
				m.setupStep = setupEmailStep
				if strings.TrimSpace(m.setupEmail) != "" && m.setupProvider.canAutoConfigure() {
					m.setupStep = setupSecretStep
				}
				if strings.TrimSpace(m.setupAccount) == "" {
					m.setupAccount = firstNonEmpty(m.account, "personal")
				}
				if strings.TrimSpace(m.setupProvider.Name) == "" && validEmailAddress(m.setupEmail) {
					m.setupProvider = detectProvider(m.setupEmail)
				}
				status := msg.err.Error()
				if m.setupStep == setupSecretStep {
					status = "paste your " + strings.ToLower(m.setupProvider.secretLabel()) + ", not your email address"
				}
				return m.withStatus(status), nil
			}
			if msg.page > 1 && len(m.messages) > 0 {
				m.loadedAll = true
				return m.withStatus(fmt.Sprintf("loaded %d emails; stopped loading older mail: %s", len(m.messages), oneLine(msg.err.Error()))), nil
			}
			m.status = msg.err.Error()
			m.messages = nil
			m.cursor = 0
			return m, nil
		}

		if msg.page == 1 {
			m.messages = msg.messages
			m.cursor = 0
		} else {
			m.messages = append(m.messages, msg.messages...)
		}
		if m.cursor >= len(m.messages) {
			m.cursor = max(0, len(m.messages)-1)
		}

		if msg.done {
			m.loadedAll = true
			if len(m.messages) == 0 {
				return m.withStatus("No emails found in " + m.mailboxLabel()), nil
			}
			return m.withStatus(fmt.Sprintf("loaded %d emails from %s", len(m.messages), m.mailboxLabel())), nil
		}

		m.loadingMore = true
		return m.withStatus(fmt.Sprintf("loaded %d emails; loading older mail in the background...", len(m.messages))), m.loadInboxPage(msg.page+1, msg.serial)
	case inboxLoadedMsg:
		m.loading = false
		m.loadingMore = false
		m.loadedAll = true
		if msg.err != nil {
			if isSetupRequiredError(msg.err) {
				m.mode = setupView
				m.messages = nil
				m.cursor = 0
				m.setupStep = setupEmailStep
				if strings.TrimSpace(m.setupEmail) != "" && m.setupProvider.canAutoConfigure() {
					m.setupStep = setupSecretStep
				}
				if strings.TrimSpace(m.setupAccount) == "" {
					m.setupAccount = firstNonEmpty(m.account, "personal")
				}
				if strings.TrimSpace(m.setupProvider.Name) == "" && validEmailAddress(m.setupEmail) {
					m.setupProvider = detectProvider(m.setupEmail)
				}
				status := msg.err.Error()
				if m.setupStep == setupSecretStep {
					status = "paste your " + strings.ToLower(m.setupProvider.secretLabel()) + ", not your email address"
				}
				return m.withStatus(status), nil
			}
			m.status = msg.err.Error()
			m.messages = nil
			m.cursor = 0
			return m, nil
		}
		m.messages = msg.messages
		if len(m.messages) == 0 {
			m.cursor = 0
			return m.withStatus("No emails found in " + m.mailboxLabel()), nil
		}
		if m.cursor >= len(m.messages) {
			m.cursor = len(m.messages) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		return m.withStatus(fmt.Sprintf("loaded %d emails from %s", len(m.messages), m.mailboxLabel())), nil
	case accountConfiguredMsg:
		m.configuring = false
		if msg.err != nil {
			return m.withStatus("Account setup failed: " + oneLine(msg.err.Error())), nil
		}
		m.account = strings.TrimSpace(msg.account)
		m.setupAccount = m.account
		m.setupSecret = ""
		if backend, ok := m.backend.(accountSetupBackend); ok {
			m.backend = backend.WithAccount(m.account)
		}
		m.mode = inboxView
		m.loading = true
		m.loadingMore = false
		m.loadedAll = false
		m.messages = nil
		m.cursor = 0
		m.loadSerial++
		return m.withStatus("Account setup finished; loading " + m.mailboxLabel() + "..."), m.loadInbox()
	case providerHelpOpenedMsg:
		if msg.err != nil {
			return m.withStatus("could not open browser: " + oneLine(msg.err.Error())), nil
		}
		return m.withStatus(msg.provider + " setup opened in your browser; return here when ready"), nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		key := msg.String()

		if m.mode == setupView {
			return m.updateSetup(msg)
		}

		if m.showThemes {
			switch key {
			case "ctrl+c":
				return m, tea.Quit
			case "up", "k":
				return m.previewTheme(m.themeCursor - 1), nil
			case "down", "j":
				return m.previewTheme(m.themeCursor + 1), nil
			case "enter", "t":
				m.theme = m.themeCursor
				m.themeBeforePicker = m.theme
				m.showThemes = false
				return m.withStatus("theme " + m.activeTheme().name + " applied"), nil
			case "esc", "q":
				m.theme = m.themeBeforePicker
				m.themeCursor = m.theme
				m.showThemes = false
				return m.withStatus("theme " + m.activeTheme().name + " kept"), nil
			case "1", "2", "3":
				index := int([]rune(key)[0] - '1')
				if index >= 0 && index < len(appThemes) {
					m.themeCursor = index
					m.theme = index
					m.themeBeforePicker = index
					m.showThemes = false
					return m.withStatus("theme " + m.activeTheme().name + " applied"), nil
				}
			}
			return m, nil
		}

		if m.showHelp {
			switch key {
			case "?", "esc", "q", "enter":
				m.showHelp = false
			}
			return m, nil
		}

		switch key {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if m.mode == readerView {
				m.mode = inboxView
				return m, nil
			}
			return m, tea.Quit
		case "?":
			m.showHelp = true
		case "up", "k":
			if m.mode == inboxView && m.cursor > 0 {
				m.cursor--
				m.status = ""
			}
		case "down", "j":
			if m.mode == inboxView && m.cursor < len(m.messages)-1 {
				m.cursor++
				m.status = ""
			}
		case "enter":
			if m.mode == inboxView && len(m.messages) > 0 {
				m.mode = readerView
				m.messages[m.cursor].Unread = false
				m.status = ""
			}
		case "b", "esc":
			if m.mode == readerView {
				m.mode = inboxView
				m.status = ""
			}
		case "r":
			if m.mode == readerView {
				return m.withStatus("reply will open $EDITOR in Phase 4"), nil
			}
		case "c":
			return m.withStatus("compose will open $EDITOR in Phase 4"), nil
		case "a":
			return m.withStatus("archive arrives in Phase 5"), nil
		case "d":
			return m.withStatus("delete confirmation arrives in Phase 5"), nil
		case "/":
			return m.withStatus("search arrives in Phase 5"), nil
		case "R":
			m.loading = true
			m.loadingMore = false
			m.loadedAll = false
			m.loadSerial++
			m.status = "refreshing " + m.mailboxLabel() + "..."
			return m, m.loadInbox()
		case "A":
			m.mode = setupView
			m.setupStep = setupEmailStep
			m.setupAccount = firstNonEmpty(m.account, m.setupAccount, "personal")
			m.status = "type your email address, then press Enter"
		case "t":
			m.showThemes = true
			m.themeCursor = m.theme
			m.themeBeforePicker = m.theme
			m.status = ""
		}
	}

	return m, nil
}

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

func (m model) loadInbox() tea.Cmd {
	if m.backend == nil {
		return nil
	}

	backend := m.backend
	if _, ok := backend.(pagedInboxBackend); ok {
		return m.loadInboxPage(1, m.loadSerial)
	}
	return func() tea.Msg {
		messages, err := backend.ListEnvelopes(context.Background())
		return inboxLoadedMsg{messages: messages, err: err}
	}
}

func (m model) loadInboxPage(page, serial int) tea.Cmd {
	backend, ok := m.backend.(pagedInboxBackend)
	if !ok {
		return m.loadInbox()
	}
	return func() tea.Msg {
		messages, done, err := backend.ListEnvelopePage(context.Background(), page)
		return inboxPageLoadedMsg{messages: messages, page: page, serial: serial, done: done, err: err}
	}
}

func (m model) View() string {
	if m.width == 0 {
		return "Starting clibox..."
	}

	content := m.renderCurrentView()
	if m.showHelp {
		content = m.overlayHelp(content)
	}
	if m.showThemes {
		content = m.overlayThemes(content)
	}

	return content
}

func (m model) renderCurrentView() string {
	header := m.renderHeader()
	footer := m.renderFooter()
	bodyHeight := max(1, m.height-lipgloss.Height(header)-lipgloss.Height(footer))

	var body string
	if m.mode == setupView {
		body = m.renderSetup(bodyHeight)
	} else if m.mode == readerView {
		body = m.renderReader(bodyHeight)
	} else {
		body = m.renderInbox(bodyHeight)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
	return m.activeTheme().styles.screen.Width(m.width).Height(max(1, m.height)).Render(content)
}

func (m model) renderHeader() string {
	theme := m.activeTheme()
	styles := theme.styles
	if m.width < 46 {
		return styles.header.Width(m.width).Render(truncate(fmt.Sprintf("clibox theme:%s", theme.name), m.width))
	}

	account := styles.subtitle.Render(m.accountLabel())
	count := styles.subtitle.Render(fmt.Sprintf("%d emails", len(m.messages)))
	title := styles.title.Render("clibox")
	badge := styles.themeBadge.Render("theme " + theme.name)
	left := title + " " + badge
	right := count
	if m.width >= 78 {
		left += " " + account
	}
	gap := max(1, m.width-lipgloss.Width(left)-lipgloss.Width(right))
	return styles.header.Width(m.width).Render(left + strings.Repeat(" ", gap) + right)
}

func (m model) renderInbox(height int) string {
	styles := m.activeTheme().styles
	if m.width >= 96 {
		return m.renderWideInbox(height)
	}

	lines := []string{styles.panelTitle.Render("Inbox")}
	lines = append(lines, m.renderRows(m.width, height-3)...)
	lines = append(lines, "")
	lines = append(lines, styles.muted.Render(fmt.Sprintf("Theme %s. Press t to choose, ? for help.", m.activeTheme().name)))
	return fitHeight(strings.Join(lines, "\n"), height)
}

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
		styles.readerBody.Width(width).Render("Examples: freddy@gmail.com, you@icloud.com, work@company.com"),
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
	lines = append(lines,
		"",
	)
	if provider.HelpURL != "" {
		lines = append(lines,
			styles.readerHeader.Width(width).Render(provider.HelpLabel),
			styles.readerBody.Width(width).Render(provider.HelpURL),
			styles.readerBody.Width(width).Render("Click the link if your terminal supports it, or press o to open it."),
		)
	}
	if provider.canAutoConfigure() {
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
	prompt := "Paste your " + provider.secretLabel() + ". clibox will save it to macOS Keychain, configure your mail connection, and reload your inbox."
	if provider.Name == "Gmail" {
		prompt = "Paste the 16-character Google app password, not your Gmail address or normal Google password. clibox will save it to macOS Keychain, configure your mail connection, and reload your inbox."
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

func (m model) renderWideInbox(height int) string {
	styles := m.activeTheme().styles
	railWidth := 18
	listWidth := min(52, max(38, m.width/2))
	readerWidth := max(24, m.width-railWidth-listWidth-4)

	rail := m.renderMailboxRail(railWidth, height)
	list := strings.Join(append(
		[]string{styles.panelTitle.Render("Inbox")},
		m.renderRows(listWidth, height-1)...,
	), "\n")
	preview := m.renderPreview(readerWidth, height)

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Width(railWidth).Render(rail),
		"  ",
		lipgloss.NewStyle().Width(listWidth).Render(list),
		"  ",
		lipgloss.NewStyle().Width(readerWidth).Render(preview),
	)
}

func (m model) renderMailboxRail(width, height int) string {
	styles := m.activeTheme().styles
	unread := 0
	for _, msg := range m.messages {
		if msg.Unread {
			unread++
		}
	}

	lines := []string{
		styles.panelTitle.Render("Mailboxes"),
		styles.selected.Width(width).Render(fmt.Sprintf("> %-8s %4d", truncate(m.mailboxLabel(), 8), len(m.messages))),
		styles.rowAlt.Width(width).Render(fmt.Sprintf("  Unread %6d", unread)),
		styles.row.Width(width).Render("  Archive"),
		styles.rowAlt.Width(width).Render("  Sent"),
		styles.row.Width(width).Render("  Drafts"),
		styles.screen.Width(width).Render(""),
		styles.panelTitle.Render("Theme"),
		styles.themeBadge.Width(width).Render(m.activeTheme().name),
		styles.screen.Width(width).Render(""),
		styles.panelTitle.Render("Accounts"),
		styles.row.Width(width).Render("  " + truncate(m.accountLabel(), max(1, width-2))),
	}
	return fitHeight(strings.Join(lines, "\n"), height)
}

func (m model) renderRows(width, height int) []string {
	styles := m.activeTheme().styles
	if m.loading && len(m.messages) == 0 {
		return []string{styles.row.Width(width).Render(truncate("  Loading inbox...", width))}
	}
	if len(m.messages) == 0 {
		return []string{styles.row.Width(width).Render(truncate("  No messages loaded. Press R to retry.", width))}
	}

	rows := make([]string, 0, len(m.messages))
	visible := max(1, height)
	start := scrollStart(m.cursor, visible, len(m.messages))
	end := min(len(m.messages), start+visible)

	for i := start; i < end; i++ {
		msg := m.messages[i]
		prefix := " "
		if i == m.cursor {
			prefix = ">"
		}
		unread := " "
		if msg.Unread {
			unread = "*"
		}

		fromWidth := 12
		dateWidth := 10
		subjectWidth := max(8, width-fromWidth-dateWidth-8)
		line := fmt.Sprintf(
			"%s %s %-*s %-*s %*s",
			prefix,
			unread,
			fromWidth,
			truncate(msg.From, fromWidth),
			subjectWidth,
			truncate(msg.Subject, subjectWidth),
			dateWidth,
			truncate(msg.Date, dateWidth),
		)

		style := styles.row
		if i%2 == 1 {
			style = styles.rowAlt
		}
		if msg.Unread {
			style = styles.unread
		}
		if i == m.cursor {
			style = styles.selected.Width(width)
		}
		rows = append(rows, style.Width(width).Render(truncate(line, width)))
	}
	if m.loadingMore && end == len(m.messages) && len(rows) < visible {
		rows = append(rows, styles.muted.Width(width).Render(truncate("  Loading older mail...", width)))
	}

	return rows
}

func (m model) renderPreview(width, height int) string {
	return m.renderMessage(width, height, true)
}

func (m model) renderReader(height int) string {
	return m.renderMessage(max(32, m.width), height, false)
}

func (m model) renderMessage(width, height int, includePreview bool) string {
	styles := m.activeTheme().styles
	if len(m.messages) == 0 {
		return fitHeight(strings.Join([]string{
			styles.panelTitle.Render("Reader"),
			styles.readerBody.Width(width).Render("No message selected."),
			styles.readerBody.Width(width).Render("Finish account setup, then press R to load your inbox."),
		}, "\n"), height)
	}

	msg := m.selectedMessage()
	body := msg.Body
	if includePreview && strings.TrimSpace(msg.Preview) != "" {
		body = msg.Preview + "\n\n" + body
	}
	lines := []string{
		styles.panelTitle.Render("Reader"),
		styles.readerHeader.Width(width).Render("From: " + msg.From + " <" + msg.Email + ">"),
		styles.readerHeader.Width(width).Render("Subject: " + msg.Subject),
		styles.readerHeader.Width(width).Render("Date: " + msg.Date),
		styles.readerBody.Width(width).Render(""),
	}
	lines = append(lines, styledLines(wrapText(body, width-2), styles.readerBody, width)...)
	return fitHeight(strings.Join(lines, "\n"), height)
}

func (m model) renderFooter() string {
	styles := m.activeTheme().styles
	themeHint := fmt.Sprintf("theme %s: t themes", m.activeTheme().name)
	hints := themeHint + "  |  j/k move  enter read  R refresh  A account  r reply  c compose  a archive  / search  ? help  q quit"
	if m.mode == readerView {
		hints = themeHint + "  |  b back  r reply  a archive  d delete  ? help  q back"
	} else if m.mode == setupView {
		hints = m.setupFooterHints()
	}
	if m.status != "" {
		hints = m.status + "  |  " + hints
	}
	return styles.footer.Width(m.width).Render(truncate(hints, max(1, m.width-2)))
}

func (m model) setupFooterHints() string {
	if m.configuring {
		return "finish account setup"
	}
	switch m.setupStep {
	case setupReviewStep:
		return "o open provider page  enter setup  e edit email  n edit account name  q back"
	case setupAccountStep:
		return "type account name  enter review  backspace edit  q back"
	default:
		return "type email address  enter continue  backspace edit  q quit"
	}
}

func (m model) overlayHelp(content string) string {
	styles := m.activeTheme().styles
	help := strings.Join([]string{
		styles.panelTitle.Render("Help"),
		"",
		"Theme      " + m.activeTheme().name,
		"",
		"j / k      move down / up",
		"Enter      open selected email",
		"b / Esc    back to inbox",
		"r          reply in $EDITOR (planned)",
		"c          compose in $EDITOR (planned)",
		"a          archive selected email (planned)",
		"d          delete selected email (planned)",
		"/          search current mailbox (planned)",
		"R          refresh inbox",
		"A          add or update an email account",
		"t          open theme picker",
		"?          close this help",
		"q          quit or close current view",
	}, "\n")

	panel := styles.helpPanel.Width(min(58, max(30, m.width-8))).Render(help)
	return placeOverlay(content, panel, m.width, m.height)
}

func (m model) overlayThemes(content string) string {
	styles := m.activeTheme().styles
	panelWidth := min(72, max(38, m.width-8))
	lines := []string{
		styles.panelTitle.Render("Themes"),
		"",
		styles.muted.Render("j/k previews colors. Enter applies. Esc cancels."),
		"",
	}

	for i, theme := range appThemes {
		prefix := " "
		if i == m.themeCursor {
			prefix = ">"
		}
		active := " "
		if i == m.themeBeforePicker {
			active = "*"
		}
		label := fmt.Sprintf("%s %s %d. %-9s %s", prefix, active, i+1, theme.name, theme.description)
		style := styles.row
		if i%2 == 1 {
			style = styles.rowAlt
		}
		if i == m.themeCursor {
			style = theme.styles.selected
		}
		lines = append(lines, style.Width(panelWidth).Render(truncate(label, panelWidth)))
		lines = append(lines, "  "+renderThemeSwatches(theme, max(10, panelWidth-2)))
	}

	lines = append(lines,
		"",
		styles.muted.Render("* original theme.  1-3 jumps directly."),
	)

	panel := styles.helpPanel.Width(panelWidth).Render(strings.Join(lines, "\n"))
	return placeOverlay(content, panel, m.width, m.height)
}

func placeOverlay(content, panel string, width, height int) string {
	topPad := max(0, (height-lipgloss.Height(panel))/3)
	leftPad := max(0, (width-lipgloss.Width(panel))/2)
	overlay := strings.Repeat("\n", topPad) + lipgloss.NewStyle().MarginLeft(leftPad).Render(panel)

	contentLines := strings.Split(content, "\n")
	overlayLines := strings.Split(overlay, "\n")
	for i, line := range overlayLines {
		if i >= len(contentLines) {
			break
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		contentLines[i] = line
	}

	return strings.Join(contentLines, "\n")
}

func renderThemeSwatches(theme appTheme, width int) string {
	swatches := []string{
		theme.styles.header.Render(" header "),
		theme.styles.themeBadge.Render(" accent "),
		theme.styles.selected.Render(" selected "),
		theme.styles.unread.Render(" unread "),
	}

	out := swatches[0]
	for _, swatch := range swatches[1:] {
		next := out + " " + swatch
		if lipgloss.Width(next) > width {
			break
		}
		out = next
	}
	return out
}

func (m model) selectedMessage() message {
	if len(m.messages) == 0 {
		return message{}
	}
	return m.messages[min(m.cursor, len(m.messages)-1)]
}

func (m model) withStatus(text string) model {
	m.status = text
	return m
}

func (m model) previewTheme(index int) model {
	if len(appThemes) == 0 {
		return m
	}
	index %= len(appThemes)
	if index < 0 {
		index += len(appThemes)
	}
	m.themeCursor = index
	m.theme = index
	return m
}

func (m model) activeTheme() appTheme {
	if len(appThemes) == 0 {
		return appTheme{name: "Default"}
	}
	index := m.theme % len(appThemes)
	if index < 0 {
		index += len(appThemes)
	}
	return appThemes[index]
}

func (m model) accountLabel() string {
	if strings.TrimSpace(m.account) != "" {
		return m.account
	}
	return "default account"
}

func (m model) mailboxLabel() string {
	if strings.TrimSpace(m.mailbox) != "" {
		return m.mailbox
	}
	return "INBOX"
}

func themeIndex(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, true
	}
	if strings.EqualFold(value, "copper") {
		value = "Ember"
	}
	for i, theme := range appThemes {
		if strings.EqualFold(theme.name, value) {
			return i, true
		}
	}
	return 0, false
}

func scrollStart(cursor, visible, total int) int {
	if total <= visible {
		return 0
	}
	half := visible / 2
	start := cursor - half
	if start < 0 {
		return 0
	}
	if start+visible > total {
		return total - visible
	}
	return start
}

func wrapText(text string, width int) []string {
	width = max(16, width)
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		line := words[0]
		for _, word := range words[1:] {
			if lipgloss.Width(line)+1+lipgloss.Width(word) > width {
				lines = append(lines, line)
				line = word
				continue
			}
			line += " " + word
		}
		lines = append(lines, line)
	}
	return lines
}

func fitHeight(value string, height int) string {
	lines := strings.Split(value, "\n")
	if len(lines) > height {
		return strings.Join(lines[:height], "\n")
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func styledLines(lines []string, style lipgloss.Style, width int) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, style.Width(width).Render(truncate(line, width)))
	}
	return out
}

func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	if width <= 3 {
		runes := []rune(value)
		return string(runes[:min(width, len(runes))])
	}

	limit := max(0, width-3)
	var out strings.Builder
	for _, r := range value {
		next := out.String() + string(r)
		if lipgloss.Width(next) > limit {
			break
		}
		out.WriteRune(r)
	}
	return out.String() + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func dropLastRune(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return ""
	}
	return string(runes[:len(runes)-1])
}

func isAccountNameRune(r rune) bool {
	if r >= 'a' && r <= 'z' {
		return true
	}
	if r >= 'A' && r <= 'Z' {
		return true
	}
	if r >= '0' && r <= '9' {
		return true
	}
	return r == '-' || r == '_' || r == '.'
}
