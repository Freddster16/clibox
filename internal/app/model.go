package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const mailboxRefreshInterval = 30 * time.Second

type viewMode int

const (
	inboxView viewMode = iota
	readerView
	setupView
	draftReviewView
)

type Options struct {
	Theme         string
	Account       string
	Mailbox       string
	ArchiveFolder string
	BackendMode   string
	Himalaya      string
	Editor        string
	PageSize      int
	ConfirmDelete *bool
	ConfigPath    string
	StatePath     string
	Accounts      map[string]AccountConfig
	Verbose       bool
	backend       inboxBackend
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

type messageBodyLoadedMsg struct {
	id     string
	body   string
	serial int
	err    error
}

type mailboxRefreshTickMsg struct{}

type accountConfiguredMsg struct {
	account string
	err     error
}

type providerHelpOpenedMsg struct {
	provider string
	err      error
}

type draftPreparedMsg struct {
	request draftRequest
	content string
	path    string
	serial  int
	err     error
}

type draftEditorFinishedMsg struct {
	path   string
	serial int
	err    error
}

type draftSentMsg struct {
	serial int
	err    error
}

type messageActionDoneMsg struct {
	action messageActionState
	err    error
}

type mailboxViewFilter int

const (
	allMailFilter mailboxViewFilter = iota
	unreadMailFilter
)

type mailboxRailEntry struct {
	Label   string
	Mailbox string
	Filter  mailboxViewFilter
	Count   string
}

type model struct {
	messages          []message
	backend           inboxBackend
	cursor            int
	mailboxCursor     int
	mailboxFocused    bool
	mailboxFilter     mailboxViewFilter
	mailboxFolders    map[string]string
	mode              viewMode
	showHelp          bool
	showThemes        bool
	loading           bool
	loadingMore       bool
	loadedAll         bool
	nextPage          int
	loadSerial        int
	loadingMessageID  string
	messageLoadSerial int
	readerOffset      int
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
	draft             draftState
	draftSerial       int
	searching         bool
	searchInput       string
	searchQuery       string
	confirmDelete     bool
	action            messageActionState
	actionSerial      int
	deletePrompt      bool
	editor            string
	width             int
	height            int
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
		backend, hint = newConfiguredBackend(options)
		if options.Mailbox == "" {
			options.Mailbox = "INBOX"
		}
	}
	setupEmail := strings.TrimSpace(hint.Email)
	setupAccount := firstNonEmpty(options.Account, hint.Account, "personal")
	setupProvider := hint.Provider
	deletePrompt := true
	if options.ConfirmDelete != nil {
		deletePrompt = *options.ConfirmDelete
	}
	mailboxFolders := mergeFolders(setupProvider.Folders)
	if strings.TrimSpace(options.ArchiveFolder) != "" {
		mailboxFolders["archive"] = strings.TrimSpace(options.ArchiveFolder)
	}

	m := model{
		backend:           backend,
		loading:           true,
		status:            status,
		account:           strings.TrimSpace(options.Account),
		mailbox:           strings.TrimSpace(options.Mailbox),
		mailboxFolders:    mailboxFolders,
		setupEmail:        setupEmail,
		setupAccount:      setupAccount,
		setupProvider:     setupProvider,
		setupStep:         setupEmailStep,
		theme:             index,
		themeCursor:       index,
		themeBeforePicker: index,
		deletePrompt:      deletePrompt,
		editor:            strings.TrimSpace(options.Editor),
	}
	m.mailboxCursor = m.activeMailboxEntryIndex()
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.loadInbox(), mailboxRefreshTick())
}

func mailboxRefreshTick() tea.Cmd {
	return tea.Tick(mailboxRefreshInterval, func(time.Time) tea.Msg {
		return mailboxRefreshTickMsg{}
	})
}

func (m model) setupStepAfterSetupRequired() setupStep {
	if strings.TrimSpace(m.setupEmail) == "" {
		return setupEmailStep
	}
	provider := m.setupProvider
	if provider.Name == "" && validEmailAddress(m.setupEmail) {
		provider = detectProvider(m.setupEmail)
	}
	if _, ok := m.backend.(oauthAccountSetupBackend); ok && providerNeedsOAuth(provider) {
		return setupReviewStep
	}
	if provider.canAutoConfigure() {
		return setupSecretStep
	}
	return setupEmailStep
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case mailboxRefreshTickMsg:
		cmd := mailboxRefreshTick()
		if m.mode != inboxView || m.mailboxFocused || m.loading || m.loadingMore || m.searching || m.confirmDelete || m.action.Running {
			return m, cmd
		}
		m.loadSerial++
		m.loading = len(m.messages) == 0
		m.loadingMore = false
		m.nextPage = 1
		m.status = "checking for new mail..."
		return m, tea.Batch(cmd, m.loadInbox())
	case inboxPageLoadedMsg:
		if msg.serial != m.loadSerial {
			return m, nil
		}
		wasLoadingMore := m.loadingMore
		wasAtEnd := len(m.messages) == 0 || m.cursor >= len(m.messages)-1
		beforeCount := len(m.messages)
		m.loading = false
		m.loadingMore = false
		if msg.err != nil {
			if msg.page == 1 && isSetupRequiredError(msg.err) {
				m.mode = setupView
				m.messages = nil
				m.cursor = 0
				if strings.TrimSpace(m.setupAccount) == "" {
					m.setupAccount = firstNonEmpty(m.account, "personal")
				}
				if strings.TrimSpace(m.setupProvider.Name) == "" && validEmailAddress(m.setupEmail) {
					m.setupProvider = detectProvider(m.setupEmail)
				}
				m.setupStep = m.setupStepAfterSetupRequired()
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
			selectedID := ""
			if len(m.messages) > 0 && !m.loading {
				selectedID = m.selectedMessage().ID
			}
			m.messages = mergeMessagePage(nil, msg.messages)
			if selectedID != "" {
				m.cursor = indexMessageByID(m.messages, selectedID)
			} else {
				m.cursor = 0
			}
		} else {
			m.messages = mergeMessagePage(m.messages, msg.messages)
		}
		addedCount := len(m.messages) - beforeCount
		if m.cursor >= len(m.messages) {
			m.cursor = max(0, len(m.messages)-1)
		}
		if wasLoadingMore && wasAtEnd && addedCount > 0 {
			m.cursor = len(m.messages) - 1
		}

		if msg.done {
			m.loadedAll = true
			m.nextPage = 0
			if len(m.messages) == 0 {
				if m.mailboxFilter == unreadMailFilter {
					return m.withStatus("No unread emails found in " + m.mailboxLabel()), nil
				}
				if strings.TrimSpace(m.searchQuery) != "" {
					return m.withStatus("No emails matched " + m.searchQuery + " in " + m.mailboxLabel()), nil
				}
				return m.withStatus("No emails found in " + m.mailboxLabel()), nil
			}
			m = m.withStatus(fmt.Sprintf("loaded %d emails from %s", len(m.messages), m.scopeLabel()))
			return m.previewSelectedMessage()
		}

		m.loadedAll = false
		m.nextPage = msg.page + 1
		if wasLoadingMore && wasAtEnd && msg.page > 1 {
			if addedCount == 0 {
				m.loadedAll = true
				m.nextPage = 0
				m = m.withStatus(fmt.Sprintf("loaded %d emails from %s; stopped because the next page had no new mail", len(m.messages), m.scopeLabel()))
				return m.previewSelectedMessage()
			}
			m.loadingMore = true
			m = m.withStatus(fmt.Sprintf("loaded %d emails from %s; loading older mail...", len(m.messages), m.scopeLabel()))
			m, previewCmd := m.previewSelectedMessage()
			nextPageCmd := m.loadInboxPage(m.nextPage, m.loadSerial)
			if previewCmd != nil {
				return m, tea.Batch(previewCmd, nextPageCmd)
			}
			return m, nextPageCmd
		}
		m = m.withStatus(fmt.Sprintf("loaded %d emails from %s; press j at the bottom for older mail", len(m.messages), m.scopeLabel()))
		return m.previewSelectedMessage()
	case inboxLoadedMsg:
		m.loading = false
		m.loadingMore = false
		m.loadedAll = true
		m.nextPage = 0
		if msg.err != nil {
			if isSetupRequiredError(msg.err) {
				m.mode = setupView
				m.messages = nil
				m.cursor = 0
				if strings.TrimSpace(m.setupAccount) == "" {
					m.setupAccount = firstNonEmpty(m.account, "personal")
				}
				if strings.TrimSpace(m.setupProvider.Name) == "" && validEmailAddress(m.setupEmail) {
					m.setupProvider = detectProvider(m.setupEmail)
				}
				m.setupStep = m.setupStepAfterSetupRequired()
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
			if m.mailboxFilter == unreadMailFilter {
				return m.withStatus("No unread emails found in " + m.mailboxLabel()), nil
			}
			if strings.TrimSpace(m.searchQuery) != "" {
				return m.withStatus("No emails matched " + m.searchQuery + " in " + m.mailboxLabel()), nil
			}
			return m.withStatus("No emails found in " + m.mailboxLabel()), nil
		}
		if m.cursor >= len(m.messages) {
			m.cursor = len(m.messages) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		m = m.withStatus(fmt.Sprintf("loaded %d emails from %s", len(m.messages), m.scopeLabel()))
		return m.previewSelectedMessage()
	case messageBodyLoadedMsg:
		if msg.serial != m.messageLoadSerial {
			return m, nil
		}
		m.loadingMessageID = ""
		if msg.err != nil {
			m = m.setMessageBodyError(msg.id, oneLine(msg.err.Error()))
			if m.mode == readerView {
				return m.withStatus("could not load email: " + oneLine(msg.err.Error())), nil
			}
			return m.withStatus("could not preview email: " + oneLine(msg.err.Error())), nil
		}
		m = m.setMessageBody(msg.id, msg.body)
		if m.mode == readerView {
			return m.withStatus("email loaded"), nil
		}
		return m.withStatus(""), nil
	case accountConfiguredMsg:
		m.configuring = false
		if msg.err != nil {
			return m.withStatus("Account setup failed: " + oneLine(msg.err.Error())), nil
		}
		m.account = strings.TrimSpace(msg.account)
		m.setupAccount = m.account
		m.setupSecret = ""
		m.mailboxFolders = mergeFolders(m.setupProvider.Folders)
		if backend, ok := m.backend.(accountSetupBackend); ok {
			m.backend = backend.WithAccount(m.account)
		}
		m.mailboxCursor = m.activeMailboxEntryIndex()
		m.mode = inboxView
		m.loading = true
		m.loadingMore = false
		m.loadedAll = false
		m.nextPage = 1
		m.messages = nil
		m.cursor = 0
		m.loadSerial++
		return m.withStatus("Account setup finished; loading " + m.mailboxLabel() + "..."), m.loadInbox()
	case providerHelpOpenedMsg:
		if msg.err != nil {
			return m.withStatus("could not open browser: " + oneLine(msg.err.Error())), nil
		}
		return m.withStatus(msg.provider + " setup opened in your browser; return here when ready"), nil
	case draftPreparedMsg:
		if msg.serial != m.draftSerial {
			removeDraftFile(msg.path)
			return m, nil
		}
		if msg.err != nil {
			return m.withStatus("could not prepare draft: " + oneLine(msg.err.Error())), nil
		}
		m.draft = draftState{
			Kind:    msg.request.Kind,
			Message: msg.request.Message,
			Path:    msg.path,
			Content: msg.content,
			Summary: parseDraftSummary(msg.content),
			Serial:  msg.serial,
		}
		if msg.request.Kind == replyDraft {
			m = m.setDraftFocus(draftFieldBody)
			if strings.HasPrefix(strings.TrimSpace(m.draft.Summary.Body), ">") {
				m.draft.Cursor = 0
			}
		} else {
			m = m.setDraftFocus(draftFieldTo)
		}
		m = m.syncDraftContent()
		m.mode = draftReviewView
		return m.withStatus("write " + msg.request.Kind.name() + "; Tab fields, Ctrl+S sends, Esc discards"), nil
	case draftEditorFinishedMsg:
		if msg.serial != m.draftSerial {
			return m, nil
		}
		if msg.err != nil {
			content, readErr := readDraftFile(msg.path)
			if readErr != nil {
				m.cleanupDraft()
				return m.withStatus("editor closed with error: " + oneLine(msg.err.Error())), nil
			}
			m.draft.Path = msg.path
			m.draft.Content = content
			m.draft.Summary = parseDraftSummary(content)
			m.draft.Sending = false
			m = m.setDraftFocus(draftFieldBody)
			m.mode = draftReviewView
			return m.withStatus("editor reported an error; review draft before sending with Ctrl+S"), nil
		}
		content, err := readDraftFile(msg.path)
		if err != nil {
			return m.withStatus("could not read draft: " + oneLine(err.Error())), nil
		}
		m.draft.Path = msg.path
		m.draft.Content = content
		m.draft.Summary = parseDraftSummary(content)
		m.draft.Sending = false
		m = m.setDraftFocus(draftFieldBody)
		m.mode = draftReviewView
		return m.withStatus("draft updated; Ctrl+S sends"), nil
	case draftSentMsg:
		if msg.serial != m.draftSerial {
			return m, nil
		}
		m.draft.Sending = false
		if msg.err != nil {
			return m.withStatus("could not send email: " + oneLine(msg.err.Error())), nil
		}
		nextMode := m.draftReturnMode()
		m.cleanupDraft()
		m.mode = nextMode
		return m.withStatus("Email sent"), nil
	case messageActionDoneMsg:
		if msg.action.Serial != m.actionSerial {
			return m, nil
		}
		m.action.Running = false
		if msg.err != nil {
			return m.withStatus("could not " + msg.action.Kind.verb() + " email: " + oneLine(msg.err.Error())), nil
		}
		m = m.removeMessage(msg.action.Message.ID)
		if msg.action.Kind == archiveAction || m.mode == readerView {
			m.mode = inboxView
		}
		return m.withStatus(msg.action.Kind.pastTense() + " " + selectedMessageLabel(msg.action.Message)), nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m.previewSelectedMessage()
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

		if m.searching {
			return m.updateSearchPrompt(msg)
		}

		if m.confirmDelete {
			return m.updateDeleteConfirm(msg)
		}

		if m.mode == draftReviewView {
			return m.updateDraftReview(msg)
		}

		if m.mode == inboxView && m.mailboxFocused {
			return m.updateMailboxRail(msg)
		}

		switch key {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if m.mode == readerView {
				return m.closeReader(), nil
			}
			return m, tea.Quit
		case "?":
			m.showHelp = true
		case "tab", "left":
			if m.mode == inboxView {
				m.mailboxFocused = true
				m.mailboxCursor = m.activeMailboxEntryIndex()
				return m.withStatus("mailboxes focused; Tab or j/k choose, Enter opens, right returns to messages"), nil
			}
		case "up", "k":
			if m.mode == readerView {
				m.readerOffset = max(0, m.readerOffset-1)
				m.status = ""
			} else if m.mode == inboxView && m.cursor > 0 {
				m.cursor--
				m.status = ""
				return m.previewSelectedMessage()
			}
		case "down", "j":
			if m.mode == readerView {
				m.readerOffset = min(m.maxReaderOffset(), m.readerOffset+1)
				m.status = ""
			} else if m.mode == inboxView && m.cursor < len(m.messages)-1 {
				m.cursor++
				m.status = ""
				return m.previewSelectedMessage()
			} else if m.mode == inboxView && len(m.messages) > 0 {
				return m.loadOlderMail()
			}
		case "pgup":
			if m.mode == readerView {
				m.readerOffset = max(0, m.readerOffset-m.readerPageSize())
				m.status = ""
			}
		case "pgdown":
			if m.mode == readerView {
				m.readerOffset = min(m.maxReaderOffset(), m.readerOffset+m.readerPageSize())
				m.status = ""
			}
		case "home":
			if m.mode == readerView {
				m.readerOffset = 0
				m.status = ""
			}
		case "end":
			if m.mode == readerView {
				m.readerOffset = m.maxReaderOffset()
				m.status = ""
			}
		case "enter":
			if m.mode == inboxView && len(m.messages) > 0 {
				return m.openSelectedMessage()
			}
		case "b", "esc":
			if m.mode == readerView {
				return m.closeReader(), nil
			} else if m.mode == inboxView && strings.TrimSpace(m.searchQuery) != "" {
				return m.clearSearch()
			} else if m.mode == inboxView && m.mailboxFilter == unreadMailFilter {
				return m.clearMailboxFilter()
			}
		case "r":
			if m.mode == readerView {
				return m.startDraft(replyDraft)
			}
		case "c":
			return m.startDraft(composeDraft)
		case "a":
			return m.startMessageAction(archiveAction)
		case "d":
			return m.askDeleteConfirmation()
		case "/":
			m.mode = inboxView
			m.searching = true
			m.searchInput = m.searchQuery
			return m.withStatus("type search text, then press Enter"), nil
		case "R":
			return m.refreshInbox("refreshing " + m.scopeLabel() + "...")
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

func (m model) updateMailboxRail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "?":
		m.showHelp = true
	case "right", "esc", "b":
		m.mailboxFocused = false
		return m.withStatus("message list focused"), nil
	case "up", "k":
		return m.moveMailboxCursor(-1), nil
	case "down", "j", "tab":
		return m.moveMailboxCursor(1), nil
	case "shift+tab", "backtab":
		return m.moveMailboxCursor(-1), nil
	case "enter":
		return m.activateMailboxEntry()
	case "R":
		m.mailboxFocused = false
		return m.refreshInbox("refreshing " + m.scopeLabel() + "...")
	case "t":
		m.showThemes = true
		m.themeCursor = m.theme
		m.themeBeforePicker = m.theme
		m.status = ""
	}
	return m, nil
}

func (m model) moveMailboxCursor(delta int) model {
	entries := m.mailboxEntries()
	if len(entries) == 0 {
		m.mailboxCursor = 0
		return m
	}
	m.mailboxCursor = (m.mailboxCursor + delta) % len(entries)
	if m.mailboxCursor < 0 {
		m.mailboxCursor += len(entries)
	}
	m.status = "mailbox: " + entries[m.mailboxCursor].Label
	return m
}

func (m model) activateMailboxEntry() (model, tea.Cmd) {
	entries := m.mailboxEntries()
	if len(entries) == 0 {
		return m.withStatus("No mailbox selected"), nil
	}
	if m.mailboxCursor < 0 || m.mailboxCursor >= len(entries) {
		m.mailboxCursor = m.activeMailboxEntryIndex()
	}
	entry := entries[m.mailboxCursor]
	return m.openMailbox(entry.Mailbox, entry.Filter, entry.Label)
}

func (m model) openMailbox(mailbox string, filter mailboxViewFilter, label string) (model, tea.Cmd) {
	if m.action.Running {
		return m.withStatus("finish the current email action first"), nil
	}
	mailbox = firstNonEmpty(mailbox, "INBOX")
	if !strings.EqualFold(m.mailboxLabel(), mailbox) {
		switcher, ok := m.backend.(mailboxSwitchBackend)
		if !ok {
			return m.withStatus("this backend cannot switch mailboxes yet"), nil
		}
		m.backend = switcher.WithMailbox(mailbox)
	}

	m.mailbox = mailbox
	m.mailboxFilter = filter
	m.mailboxFocused = false
	m.mailboxCursor = m.activeMailboxEntryIndex()
	m.searching = false
	m.searchInput = ""
	m.searchQuery = ""
	return m.reloadMailbox("loading " + label + "...")
}

func (m model) startDraft(kind draftKind) (model, tea.Cmd) {
	backend, ok := m.backend.(draftBackend)
	if !ok {
		return m.withStatus("this backend cannot send email yet"), nil
	}

	request := draftRequest{Kind: kind}
	if kind == replyDraft {
		if m.mode != readerView || len(m.messages) == 0 {
			return m.withStatus("open an email before replying"), nil
		}
		request.Message = m.selectedMessage()
		if strings.TrimSpace(request.Message.Email) == "" {
			return m.withStatus("selected email has no reply address"), nil
		}
	}

	if m.draft.Path != "" {
		removeDraftFile(m.draft.Path)
	}
	m.draftSerial++
	serial := m.draftSerial
	m.draft = draftState{Kind: kind, Message: request.Message, Serial: serial}

	return m.withStatus("preparing " + kind.name() + " draft..."), m.prepareDraft(backend, request, serial)
}

func (m model) prepareDraft(backend draftBackend, request draftRequest, serial int) tea.Cmd {
	return func() tea.Msg {
		content, err := backend.PrepareDraft(context.Background(), request)
		if err != nil {
			return draftPreparedMsg{request: request, serial: serial, err: err}
		}
		path, err := writeDraftFile(content)
		return draftPreparedMsg{request: request, content: content, path: path, serial: serial, err: err}
	}
}

func (m model) openDraftEditor() tea.Cmd {
	path := m.draft.Path
	serial := m.draft.Serial
	if err := saveDraftFile(path, m.draft.Content); err != nil {
		return func() tea.Msg {
			return draftEditorFinishedMsg{path: path, serial: serial, err: err}
		}
	}
	cmd, err := draftEditorCommand(path, m.editor)
	if err != nil {
		return func() tea.Msg {
			return draftEditorFinishedMsg{path: path, serial: serial, err: err}
		}
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return draftEditorFinishedMsg{path: path, serial: serial, err: err}
	})
}

func (m model) updateDraftReview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.draft.Sending {
		if key == "ctrl+c" {
			m.cleanupDraft()
			return m, tea.Quit
		}
		return m, nil
	}

	switch key {
	case "ctrl+c":
		m.cleanupDraft()
		return m, tea.Quit
	case "esc":
		nextMode := m.draftReturnMode()
		m.cleanupDraft()
		m.mode = nextMode
		return m.withStatus("draft discarded"), nil
	case "ctrl+o":
		return m.withStatus("opening " + m.draft.Kind.name() + " in " + m.editorLabel() + "..."), m.openDraftEditor()
	case "ctrl+s":
		return m.sendCurrentDraft()
	case "tab", "down":
		return m.setDraftFocus(m.draft.Focus + 1).withStatus(""), nil
	case "shift+tab", "backtab", "up":
		return m.setDraftFocus(m.draft.Focus - 1).withStatus(""), nil
	case "enter":
		if m.draft.Focus == draftFieldBody {
			return m.insertDraftText("\n").withStatus(""), nil
		}
		return m.setDraftFocus(m.draft.Focus + 1).withStatus(""), nil
	case "left":
		return m.moveDraftCursor(-1).withStatus(""), nil
	case "right":
		return m.moveDraftCursor(1).withStatus(""), nil
	case "home":
		m.draft.Cursor = 0
		return m.withStatus(""), nil
	case "end":
		m.draft.Cursor = draftTextLen(m.draftFieldValue())
		return m.withStatus(""), nil
	case "backspace":
		return m.deleteDraftBackward().withStatus(""), nil
	case "ctrl+u":
		return m.clearDraftField().withStatus(""), nil
	case "ctrl+w":
		return m.trimDraftFieldWord().withStatus(""), nil
	}
	if msg.Type == tea.KeyRunes {
		return m.insertDraftText(string(msg.Runes)).withStatus(""), nil
	}
	return m, nil
}

func (m model) sendCurrentDraft() (model, tea.Cmd) {
	m = m.syncDraftContent()
	summary := parseDraftSummary(m.draft.Content)
	if err := validateDraftForSend(summary); err != nil {
		m.draft.Summary = summary
		return m.withStatus(err.Error()), nil
	}
	backend, ok := m.backend.(draftBackend)
	if !ok {
		return m.withStatus("this backend cannot send email yet"), nil
	}
	m.draft.Summary = summary
	m.draft.Sending = true
	return m.withStatus("sending email..."), m.sendDraft(backend, m.draft.Content, m.draft.Serial)
}

func (m model) setDraftFocus(field draftField) model {
	for field < 0 {
		field += draftFieldCount
	}
	field = field % draftFieldCount
	m.draft.Focus = field
	m.draft.Cursor = draftTextLen(m.draftFieldValue())
	return m
}

func (m model) draftFieldValue() string {
	switch m.draft.Focus {
	case draftFieldSubject:
		return m.draft.Summary.Subject
	case draftFieldBody:
		return m.draft.Summary.Body
	default:
		return m.draft.Summary.To
	}
}

func (m model) setDraftFieldValue(value string) model {
	switch m.draft.Focus {
	case draftFieldSubject:
		m.draft.Summary.Subject = terminalSafeLine(value)
	case draftFieldBody:
		m.draft.Summary.Body = terminalSafeText(normalizeDraftContent(value))
	default:
		m.draft.Summary.To = terminalSafeLine(value)
	}
	return m.syncDraftContent()
}

func (m model) insertDraftText(text string) model {
	value := m.draftFieldValue()
	next, cursor := insertTextAt(value, text, m.draft.Cursor)
	m.draft.Cursor = cursor
	return m.setDraftFieldValue(next)
}

func (m model) deleteDraftBackward() model {
	value := m.draftFieldValue()
	next, cursor := deleteTextBefore(value, m.draft.Cursor)
	m.draft.Cursor = cursor
	return m.setDraftFieldValue(next)
}

func (m model) clearDraftField() model {
	m.draft.Cursor = 0
	return m.setDraftFieldValue("")
}

func (m model) trimDraftFieldWord() model {
	value := m.draftFieldValue()
	before, after := splitTextAt(value, m.draft.Cursor)
	before = trimLastWord(before)
	m.draft.Cursor = draftTextLen(before)
	return m.setDraftFieldValue(before + after)
}

func (m model) moveDraftCursor(delta int) model {
	m.draft.Cursor = min(max(0, m.draft.Cursor+delta), draftTextLen(m.draftFieldValue()))
	return m
}

func (m model) syncDraftContent() model {
	m.draft.Content = draftContentFromSummary(m.draft.Summary)
	return m
}

func (m model) sendDraft(backend draftBackend, content string, serial int) tea.Cmd {
	return func() tea.Msg {
		return draftSentMsg{serial: serial, err: backend.SendDraft(context.Background(), content)}
	}
}

func (m *model) cleanupDraft() {
	removeDraftFile(m.draft.Path)
	m.draft = draftState{}
}

func (m model) draftReturnMode() viewMode {
	if m.draft.Kind == replyDraft && len(m.messages) > 0 {
		return readerView
	}
	return inboxView
}

func (m model) loadInbox() tea.Cmd {
	if m.backend == nil {
		return nil
	}

	backend := m.backend
	if _, ok := backend.(searchablePagedInboxBackend); ok {
		return m.loadInboxPage(1, m.loadSerial)
	}
	if strings.TrimSpace(m.searchQuery) == "" {
		if _, ok := backend.(pagedInboxBackend); ok {
			return m.loadInboxPage(1, m.loadSerial)
		}
	}
	return func() tea.Msg {
		messages, err := backend.ListEnvelopes(context.Background())
		if strings.TrimSpace(m.searchQuery) != "" {
			messages = filterMessages(messages, m.searchQuery)
		}
		if m.mailboxFilter == unreadMailFilter {
			messages = filterUnreadMessages(messages)
		}
		return inboxLoadedMsg{messages: messages, err: err}
	}
}

func (m model) loadInboxPage(page, serial int) tea.Cmd {
	query := m.searchQuery
	filter := m.mailboxFilter
	if backend, ok := m.backend.(searchablePagedInboxBackend); ok {
		return func() tea.Msg {
			messages, done, err := backend.SearchEnvelopePage(context.Background(), page, query)
			if filter == unreadMailFilter {
				messages = filterUnreadMessages(messages)
			}
			return inboxPageLoadedMsg{messages: messages, page: page, serial: serial, done: done, err: err}
		}
	}
	if strings.TrimSpace(query) == "" {
		if backend, ok := m.backend.(pagedInboxBackend); ok {
			return func() tea.Msg {
				messages, done, err := backend.ListEnvelopePage(context.Background(), page)
				if filter == unreadMailFilter {
					messages = filterUnreadMessages(messages)
				}
				return inboxPageLoadedMsg{messages: messages, page: page, serial: serial, done: done, err: err}
			}
		}
	}
	return m.loadInbox()
}

func (m model) loadOlderMail() (model, tea.Cmd) {
	if m.loadedAll {
		return m.withStatus("all loaded from " + m.scopeLabel()), nil
	}
	if m.loadingMore {
		return m, nil
	}
	if m.nextPage <= 1 {
		m.nextPage = 2
	}
	m.loadingMore = true
	return m.withStatus("loading older mail from " + m.scopeLabel() + "..."), m.loadInboxPage(m.nextPage, m.loadSerial)
}

func mergeMessagePage(existing, page []message) []message {
	if len(page) == 0 {
		return existing
	}
	merged := make([]message, 0, len(existing)+len(page))
	seen := make(map[string]struct{}, len(existing)+len(page))
	for _, msg := range existing {
		key := messageDedupeKey(msg)
		if key != "" {
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
		}
		merged = append(merged, msg)
	}
	for _, msg := range page {
		key := messageDedupeKey(msg)
		if key != "" {
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
		}
		merged = append(merged, msg)
	}
	return merged
}

func messageDedupeKey(msg message) string {
	if id := strings.TrimSpace(msg.ID); id != "" {
		return "id:" + id
	}
	return ""
}

func indexMessageByID(messages []message, id string) int {
	id = strings.TrimSpace(id)
	if id == "" {
		return 0
	}
	for i, msg := range messages {
		if strings.TrimSpace(msg.ID) == id {
			return i
		}
	}
	return 0
}

func (m model) previewSelectedMessage() (model, tea.Cmd) {
	if m.mode != inboxView || m.width < 96 || m.mailboxFocused || len(m.messages) == 0 {
		return m, nil
	}
	msg := m.selectedMessage()
	if messageBodyReady(msg) || strings.TrimSpace(msg.BodyError) != "" || strings.TrimSpace(msg.ID) == "" {
		return m, nil
	}
	if m.loadingMessageID == msg.ID {
		return m, nil
	}
	backend, ok := m.backend.(messageBodyBackend)
	if !ok {
		return m, nil
	}
	m.messageLoadSerial++
	m.loadingMessageID = msg.ID
	return m, m.loadMessageBody(backend, msg, m.messageLoadSerial)
}

func (m model) openSelectedMessage() (model, tea.Cmd) {
	if len(m.messages) == 0 {
		return m.withStatus("No email selected"), nil
	}

	index := min(m.cursor, len(m.messages)-1)
	m.mode = readerView
	m.readerOffset = 0
	m.messages[index].Unread = false
	msg := m.messages[index]
	if messageBodyReady(msg) {
		return m.withStatus(""), nil
	}
	if m.loadingMessageID == msg.ID {
		return m.withStatus("Loading email..."), nil
	}

	backend, ok := m.backend.(messageBodyBackend)
	if !ok {
		m.messages[index].BodyError = "This backend cannot load full emails yet."
		return m.withStatus("email body is not available yet"), nil
	}

	m.messageLoadSerial++
	m.loadingMessageID = msg.ID
	m.messages[index].BodyError = ""
	return m.withStatus("Loading email..."), m.loadMessageBody(backend, msg, m.messageLoadSerial)
}

func (m model) closeReader() model {
	m.mode = inboxView
	m.status = ""
	m.readerOffset = 0
	if m.mailboxFilter == unreadMailFilter {
		m.messages = filterUnreadMessages(m.messages)
		if m.cursor >= len(m.messages) {
			m.cursor = max(0, len(m.messages)-1)
		}
	}
	return m
}

func (m model) loadMessageBody(backend messageBodyBackend, msg message, serial int) tea.Cmd {
	return func() tea.Msg {
		body, err := backend.ReadMessage(context.Background(), msg)
		return messageBodyLoadedMsg{id: msg.ID, body: body, serial: serial, err: err}
	}
}

func (m model) selectedMessage() message {
	if len(m.messages) == 0 {
		return message{}
	}
	return m.messages[min(m.cursor, len(m.messages)-1)]
}

func (m model) setMessageBody(id, body string) model {
	for i := range m.messages {
		if m.messages[i].ID == id {
			m.messages[i].Body = body
			m.messages[i].BodyLoaded = true
			m.messages[i].BodyError = ""
			break
		}
	}
	return m
}

func (m model) setMessageBodyError(id, message string) model {
	for i := range m.messages {
		if m.messages[i].ID == id {
			m.messages[i].Body = ""
			m.messages[i].BodyLoaded = false
			m.messages[i].BodyError = message
			break
		}
	}
	return m
}

func (m model) messageBodyText(msg message, includePreview bool) string {
	var parts []string
	if includePreview && strings.TrimSpace(msg.Preview) != "" {
		parts = append(parts, msg.Preview)
	}

	switch {
	case m.loadingMessageID != "" && m.loadingMessageID == msg.ID:
		parts = append(parts, "Loading email...")
	case strings.TrimSpace(msg.BodyError) != "":
		parts = append(parts, "Could not load this email: "+msg.BodyError)
	case messageBodyReady(msg):
		parts = append(parts, msg.Body)
	case includePreview:
		parts = append(parts, "Loading preview...")
	default:
		parts = append(parts, "Loading email...")
	}

	return strings.Join(parts, "\n\n")
}

func (m model) maxReaderOffset() int {
	if m.mode != readerView || len(m.messages) == 0 {
		return 0
	}
	bodyHeight := m.readerPageSize()
	lines := wrapText(m.messageBodyText(m.selectedMessage(), false), max(16, m.width-2))
	return max(0, len(lines)-bodyHeight)
}

func (m model) readerPageSize() int {
	return max(1, m.height-7)
}

func messageBodyReady(msg message) bool {
	return msg.BodyLoaded || strings.TrimSpace(msg.Body) != ""
}

func (m model) withStatus(text string) model {
	m.status = text
	return m
}

func (m model) accountLabel() string {
	if strings.TrimSpace(m.account) != "" {
		return terminalSafeLine(m.account)
	}
	return "default account"
}

func (m model) mailboxLabel() string {
	if strings.TrimSpace(m.mailbox) != "" {
		return m.mailbox
	}
	return "INBOX"
}

func (m model) mailboxEntries() []mailboxRailEntry {
	folders := mergeFolders(m.mailboxFolders)
	currentUnread := m.unreadMessageCount()
	return []mailboxRailEntry{
		{Label: "Inbox", Mailbox: firstNonEmpty(folders["inbox"], "INBOX"), Filter: allMailFilter, Count: m.mailboxEntryCount(firstNonEmpty(folders["inbox"], "INBOX"), allMailFilter)},
		{Label: "Unread", Mailbox: firstNonEmpty(folders["inbox"], "INBOX"), Filter: unreadMailFilter, Count: countLabel(currentUnread)},
		{Label: "Archive", Mailbox: firstNonEmpty(folders["archive"], "Archive"), Filter: allMailFilter, Count: m.mailboxEntryCount(firstNonEmpty(folders["archive"], "Archive"), allMailFilter)},
		{Label: "Sent", Mailbox: firstNonEmpty(folders["sent"], "Sent"), Filter: allMailFilter, Count: m.mailboxEntryCount(firstNonEmpty(folders["sent"], "Sent"), allMailFilter)},
		{Label: "Drafts", Mailbox: firstNonEmpty(folders["drafts"], "Drafts"), Filter: allMailFilter, Count: m.mailboxEntryCount(firstNonEmpty(folders["drafts"], "Drafts"), allMailFilter)},
		{Label: "Trash", Mailbox: firstNonEmpty(folders["trash"], "Trash"), Filter: allMailFilter, Count: m.mailboxEntryCount(firstNonEmpty(folders["trash"], "Trash"), allMailFilter)},
	}
}

func (m model) mailboxEntryCount(mailbox string, filter mailboxViewFilter) string {
	if !m.mailboxEntryActive(mailbox, filter) {
		return ""
	}
	return countLabel(len(m.messages))
}

func countLabel(count int) string {
	return fmt.Sprintf("%d", count)
}

func (m model) unreadMessageCount() int {
	count := 0
	for _, msg := range m.messages {
		if msg.Unread {
			count++
		}
	}
	return count
}

func (m model) activeMailboxEntryIndex() int {
	entries := m.mailboxEntries()
	for i, entry := range entries {
		if m.mailboxEntryActive(entry.Mailbox, entry.Filter) {
			return i
		}
	}
	return 0
}

func (m model) mailboxEntryActive(mailbox string, filter mailboxViewFilter) bool {
	return m.mailboxFilter == filter && sameMailboxName(m.mailboxLabel(), mailbox)
}

func sameMailboxName(left, right string) bool {
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
}

func (m model) currentMailboxTitle() string {
	for _, entry := range m.mailboxEntries() {
		if m.mailboxEntryActive(entry.Mailbox, entry.Filter) {
			return entry.Label
		}
	}
	return m.mailboxLabel()
}

func (m model) scopeLabel() string {
	scope := m.currentMailboxTitle()
	if m.mailboxFilter == unreadMailFilter && !sameMailboxName(scope, "Unread") {
		scope = "Unread"
	}
	if strings.TrimSpace(m.searchQuery) != "" {
		return fmt.Sprintf("%s search %q", scope, m.searchQuery)
	}
	return scope
}

func (m model) inboxTitle() string {
	if strings.TrimSpace(m.searchQuery) != "" && m.mailboxFilter == unreadMailFilter {
		return fmt.Sprintf("Unread search: %s", m.searchQuery)
	}
	if strings.TrimSpace(m.searchQuery) != "" {
		return fmt.Sprintf("Search: %s", m.searchQuery)
	}
	return m.currentMailboxTitle()
}

func (m model) editorLabel() string {
	return firstNonEmpty(m.editor, os.Getenv("CLIBOX_EDITOR"), os.Getenv("VISUAL"), os.Getenv("EDITOR"), "nvim")
}
