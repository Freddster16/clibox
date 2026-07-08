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
	Theme           string
	Account         string
	Mailbox         string
	ArchiveFolder   string
	BackendMode     string
	Himalaya        string
	Editor          string
	PageSize        int
	ConfirmDelete   *bool
	ComposeFormat   string
	ConfigPath      string
	StatePath       string
	RememberSession bool
	LastSession     LastSession
	Accounts        map[string]AccountConfig
	Verbose         bool
	backend         inboxBackend
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
	images []messageImage
	notice string
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

type messageMarkedReadMsg struct {
	err error
}

type messageFlagToggledMsg struct {
	flagged bool
	err     error
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
	composeFormat     string
	statePath         string
	rememberSession   bool
	restoreSession    LastSession
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

	lastSession := options.LastSession.normalized()
	if strings.TrimSpace(lastSession.Account) != "" {
		options.Account = lastSession.Account
	}
	if strings.TrimSpace(lastSession.Mailbox) != "" {
		options.Mailbox = lastSession.Mailbox
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
		composeFormat:     strings.ToLower(strings.TrimSpace(options.ComposeFormat)),
		statePath:         strings.TrimSpace(options.StatePath),
		rememberSession:   options.RememberSession,
		restoreSession:    lastSession,
	}
	m.mailboxFilter = sessionMailboxFilter(lastSession.MailboxFilter)
	if strings.TrimSpace(lastSession.SearchQuery) != "" {
		m.searchQuery = lastSession.SearchQuery
		m.searchInput = lastSession.SearchQuery
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
		return m.handleInboxPageLoaded(msg)
	case inboxLoadedMsg:
		return m.handleInboxLoaded(msg)
	case messageBodyLoadedMsg:
		return m.handleMessageBodyLoaded(msg)
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
		m = m.withStatus(msg.action.Kind.pastTense() + " " + selectedMessageLabel(msg.action.Message))
		return m.withSessionSave(nil)
	case messageMarkedReadMsg:
		if msg.err != nil {
			return m.withStatus("could not mark email read on server: " + oneLine(msg.err.Error())), nil
		}
		return m, nil
	case messageFlagToggledMsg:
		if msg.err != nil {
			action := "flag"
			if !msg.flagged {
				action = "unflag"
			}
			return m.withStatus("could not " + action + " email: " + oneLine(msg.err.Error())), nil
		}
		return m, nil
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
				return m.quitWithSession()
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
			return m.quitWithSession()
		case "q":
			if m.mode == readerView {
				m = m.closeReader()
				return m.withSessionSave(nil)
			}
			return m.quitWithSession()
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
				return m.withSessionSave(nil)
			} else if m.mode == inboxView && m.cursor > 0 {
				m.cursor--
				m.status = ""
				m, cmd := m.previewSelectedMessage()
				return m.withSessionSave(cmd)
			}
		case "down", "j":
			if m.mode == readerView {
				m.readerOffset = min(m.maxReaderOffset(), m.readerOffset+1)
				m.status = ""
				return m.withSessionSave(nil)
			} else if m.mode == inboxView && m.cursor < len(m.messages)-1 {
				m.cursor++
				m.status = ""
				m, cmd := m.previewSelectedMessage()
				return m.withSessionSave(cmd)
			} else if m.mode == inboxView && len(m.messages) > 0 {
				return m.loadOlderMail()
			}
		case "pgup":
			if m.mode == readerView {
				m.readerOffset = max(0, m.readerOffset-m.readerPageSize())
				m.status = ""
				return m.withSessionSave(nil)
			}
		case "pgdown":
			if m.mode == readerView {
				m.readerOffset = min(m.maxReaderOffset(), m.readerOffset+m.readerPageSize())
				m.status = ""
				return m.withSessionSave(nil)
			}
		case "home":
			if m.mode == readerView {
				m.readerOffset = 0
				m.status = ""
				return m.withSessionSave(nil)
			}
		case "end":
			if m.mode == readerView {
				m.readerOffset = m.maxReaderOffset()
				m.status = ""
				return m.withSessionSave(nil)
			}
		case "enter":
			if m.mode == inboxView && len(m.messages) > 0 {
				return m.openSelectedMessage()
			}
		case "b", "esc":
			if m.mode == readerView {
				m = m.closeReader()
				return m.withSessionSave(nil)
			} else if m.mode == inboxView && strings.TrimSpace(m.searchQuery) != "" {
				return m.clearSearch()
			} else if m.mode == inboxView && m.mailboxFilter == unreadMailFilter {
				return m.clearMailboxFilter()
			}
		case "r":
			if m.mode == readerView {
				return m.startDraft(replyDraft)
			}
		case "f":
			if m.mode == readerView {
				return m.startDraft(forwardDraft)
			}
		case "c":
			return m.startDraft(composeDraft)
		case "a":
			return m.startMessageAction(archiveAction)
		case "d":
			return m.askDeleteConfirmation()
		case "m":
			return m.toggleRead()
		case "s":
			return m.toggleFlagged()
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
		return m.quitWithSession()
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
	if kind == replyDraft || kind == forwardDraft {
		if m.mode != readerView || len(m.messages) == 0 {
			return m.withStatus("open an email before " + kind.name() + "ing"), nil
		}
		request.Message = m.selectedMessage()
		if kind == replyDraft && strings.TrimSpace(request.Message.Email) == "" {
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
			return m.quitWithSession()
		}
		return m, nil
	}

	switch key {
	case "ctrl+c":
		m.cleanupDraft()
		return m.quitWithSession()
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
	if (m.draft.Kind == replyDraft || m.draft.Kind == forwardDraft) && len(m.messages) > 0 {
		return readerView
	}
	return inboxView
}
