package app

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type viewMode int

const (
	inboxView viewMode = iota
	readerView
	setupView
	draftReviewView
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

type messageBodyLoadedMsg struct {
	id     string
	body   string
	serial int
	err    error
}

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
	case messageBodyLoadedMsg:
		if msg.serial != m.messageLoadSerial {
			return m, nil
		}
		m.loadingMessageID = ""
		if msg.err != nil {
			m = m.setMessageBodyError(msg.id, oneLine(msg.err.Error()))
			return m.withStatus("could not load email: " + oneLine(msg.err.Error())), nil
		}
		m = m.setMessageBody(msg.id, msg.body)
		return m.withStatus("email loaded"), nil
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
		return m.withStatus("opening " + msg.request.Kind.name() + " in " + m.editorLabel() + "..."), m.openDraftEditor()
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
			m.mode = draftReviewView
			return m.withStatus("editor reported an error; review draft before sending"), nil
		}
		content, err := readDraftFile(msg.path)
		if err != nil {
			return m.withStatus("could not read draft: " + oneLine(err.Error())), nil
		}
		m.draft.Path = msg.path
		m.draft.Content = content
		m.draft.Summary = parseDraftSummary(content)
		m.draft.Sending = false
		m.mode = draftReviewView
		return m.withStatus("review draft; press s to send or e to edit"), nil
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

		if m.mode == draftReviewView {
			return m.updateDraftReview(msg)
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
			if m.mode == readerView {
				m.readerOffset = max(0, m.readerOffset-1)
				m.status = ""
			} else if m.mode == inboxView && m.cursor > 0 {
				m.cursor--
				m.status = ""
			}
		case "down", "j":
			if m.mode == readerView {
				m.readerOffset = min(m.maxReaderOffset(), m.readerOffset+1)
				m.status = ""
			} else if m.mode == inboxView && m.cursor < len(m.messages)-1 {
				m.cursor++
				m.status = ""
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
				m.mode = inboxView
				m.status = ""
			}
		case "r":
			if m.mode == readerView {
				return m.startDraft(replyDraft)
			}
		case "c":
			return m.startDraft(composeDraft)
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
			m.loadingMessageID = ""
			m.readerOffset = 0
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
	cmd, err := draftEditorCommand(path)
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
	case "q", "esc", "b":
		nextMode := m.draftReturnMode()
		m.cleanupDraft()
		m.mode = nextMode
		return m.withStatus("draft discarded"), nil
	case "e":
		return m.withStatus("opening " + m.draft.Kind.name() + " in " + m.editorLabel() + "..."), m.openDraftEditor()
	case "s":
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
	case "?":
		m.showHelp = true
	}
	return m, nil
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
		parts = append(parts, "Press Enter to read this email.")
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

func (m model) editorLabel() string {
	return firstNonEmpty(os.Getenv("CLIBOX_EDITOR"), os.Getenv("VISUAL"), os.Getenv("EDITOR"), "nvim")
}
