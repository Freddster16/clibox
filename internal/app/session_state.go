package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const lastSessionStateKey = "last_session"

type LastSession struct {
	Account       string `json:"account,omitempty"`
	Mailbox       string `json:"mailbox,omitempty"`
	MailboxFilter string `json:"mailbox_filter,omitempty"`
	SearchQuery   string `json:"search_query,omitempty"`
	MessageID     string `json:"message_id,omitempty"`
	ReaderOpen    bool   `json:"reader_open,omitempty"`
	ReaderOffset  int    `json:"reader_offset,omitempty"`
	UpdatedAt     string `json:"updated_at,omitempty"`
}

func LoadLastSession(path string) (LastSession, bool, error) {
	store, err := openNativeStore(path)
	if err != nil {
		return LastSession{}, false, err
	}
	defer store.close()

	raw, ok, err := store.appState(context.Background(), lastSessionStateKey)
	if err != nil || !ok {
		return LastSession{}, ok, err
	}

	var session LastSession
	if err := json.Unmarshal(raw, &session); err != nil {
		return LastSession{}, false, fmt.Errorf("could not parse clibox last session: %w", err)
	}
	session = session.normalized()
	if !session.hasLocation() {
		return LastSession{}, false, nil
	}
	return session, true, nil
}

func saveLastSession(path string, session LastSession) error {
	session = session.normalized()
	session.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("could not encode clibox last session: %w", err)
	}

	store, err := openNativeStore(path)
	if err != nil {
		return err
	}
	defer store.close()
	return store.saveAppState(context.Background(), lastSessionStateKey, data)
}

func (s LastSession) normalized() LastSession {
	s.Account = sanitizeAccountName(s.Account, "")
	s.Mailbox = terminalSafeLine(s.Mailbox)
	s.SearchQuery = terminalSafeLine(s.SearchQuery)
	s.MessageID = strings.TrimSpace(s.MessageID)
	s.MailboxFilter = strings.ToLower(strings.TrimSpace(s.MailboxFilter))
	if s.MailboxFilter != "unread" {
		s.MailboxFilter = "all"
	}
	if s.ReaderOffset < 0 {
		s.ReaderOffset = 0
	}
	return s
}

func (s LastSession) hasLocation() bool {
	return strings.TrimSpace(s.Account) != "" ||
		strings.TrimSpace(s.Mailbox) != "" ||
		strings.TrimSpace(s.SearchQuery) != "" ||
		strings.TrimSpace(s.MessageID) != ""
}

func sessionFilterName(filter mailboxViewFilter) string {
	if filter == unreadMailFilter {
		return "unread"
	}
	return "all"
}

func sessionMailboxFilter(name string) mailboxViewFilter {
	if strings.EqualFold(strings.TrimSpace(name), "unread") {
		return unreadMailFilter
	}
	return allMailFilter
}

func (m model) currentSession() LastSession {
	state := LastSession{
		Account:       m.account,
		Mailbox:       m.mailboxLabel(),
		MailboxFilter: sessionFilterName(m.mailboxFilter),
		SearchQuery:   m.searchQuery,
		ReaderOpen:    m.mode == readerView,
		ReaderOffset:  m.readerOffset,
	}
	if len(m.messages) > 0 {
		state.MessageID = m.selectedMessage().ID
	}
	return state.normalized()
}

func (m model) saveSessionCmd() tea.Cmd {
	if !m.rememberSession {
		return nil
	}
	state := m.currentSession()
	path := m.statePath
	return func() tea.Msg {
		_ = saveLastSession(path, state)
		return nil
	}
}

func (m model) withSessionSave(cmd tea.Cmd) (model, tea.Cmd) {
	saveCmd := m.saveSessionCmd()
	if saveCmd == nil {
		return m, cmd
	}
	if cmd == nil {
		return m, saveCmd
	}
	return m, tea.Batch(cmd, saveCmd)
}

func (m model) quitWithSession() (model, tea.Cmd) {
	if m.rememberSession {
		_ = saveLastSession(m.statePath, m.currentSession())
	}
	return m, tea.Quit
}

func (m model) restoreLastSessionAfterLoad() (model, tea.Cmd, bool) {
	messageID := strings.TrimSpace(m.restoreSession.MessageID)
	if messageID == "" {
		return m, nil, false
	}

	if index, ok := findMessageIndexByID(m.messages, messageID); ok {
		readerOpen := m.restoreSession.ReaderOpen
		readerOffset := m.restoreSession.ReaderOffset
		m.cursor = index
		m.restoreSession.MessageID = ""
		if readerOpen {
			m, cmd := m.openSelectedMessageAtOffset(readerOffset)
			return m, cmd, true
		}
		m = m.withStatus("picked up where you left off")
		m, cmd := m.previewSelectedMessage()
		return m, cmd, true
	}

	if !m.loadedAll && m.nextPage > 1 && !m.loadingMore {
		m.loadingMore = true
		m.status = "looking for where you left off..."
		return m, m.loadInboxPage(m.nextPage, m.loadSerial), true
	}

	m.restoreSession.MessageID = ""
	return m, nil, false
}
