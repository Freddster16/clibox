package app

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) updateSearchPrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.searching = false
		return m.withStatus("search canceled"), nil
	case "enter":
		input := strings.TrimSpace(m.searchInput)
		if input != "" && buildSearchQuery(input) == "" {
			return m.withStatus("try a search with letters, numbers, names, or email addresses"), nil
		}
		m.searching = false
		m.searchQuery = input
		if input == "" {
			return m.refreshInbox("clearing search...")
		}
		return m.refreshInbox("searching " + m.mailboxLabel() + " for " + input + "...")
	case "backspace", "delete":
		runes := []rune(m.searchInput)
		if len(runes) > 0 {
			m.searchInput = string(runes[:len(runes)-1])
		}
	case "ctrl+u":
		m.searchInput = ""
	case "ctrl+w":
		m.searchInput = trimLastWord(m.searchInput)
	default:
		if msg.Type == tea.KeyRunes {
			for _, r := range msg.Runes {
				if r >= 32 && r < 127 && len([]rune(m.searchInput)) < 160 {
					m.searchInput += string(r)
				}
			}
		}
	}
	return m, nil
}

func (m model) updateDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "y", "Y", "enter":
		m.confirmDelete = false
		return m.startMessageAction(deleteAction)
	case "n", "N", "esc", "b", "q":
		m.confirmDelete = false
		return m.withStatus("delete canceled"), nil
	}
	return m, nil
}

func (m model) askDeleteConfirmation() (model, tea.Cmd) {
	if m.action.Running {
		return m.withStatus("finish the current email action first"), nil
	}
	msg, index, ok := m.currentActionMessage()
	if !ok {
		return m.withStatus("No email selected"), nil
	}
	if strings.TrimSpace(msg.ID) == "" {
		return m.withStatus("selected email has no backend id"), nil
	}
	if !m.deletePrompt {
		return m.startMessageAction(deleteAction)
	}
	m.confirmDelete = true
	m.action = messageActionState{Kind: deleteAction, Message: msg, Index: index}
	return m.withStatus("Move to Trash? y confirms, n cancels: " + selectedMessageLabel(msg)), nil
}

func (m model) startMessageAction(kind messageActionKind) (model, tea.Cmd) {
	if m.action.Running {
		return m.withStatus("finish the current email action first"), nil
	}
	backend, ok := m.backend.(messageActionBackend)
	if !ok {
		return m.withStatus("this backend cannot " + kind.verb() + " email yet"), nil
	}

	msg, index, ok := m.currentActionMessage()
	if !ok {
		return m.withStatus("No email selected"), nil
	}
	if strings.TrimSpace(msg.ID) == "" {
		return m.withStatus("selected email has no backend id"), nil
	}

	m.actionSerial++
	action := messageActionState{
		Kind:    kind,
		Message: msg,
		Index:   index,
		Serial:  m.actionSerial,
		Running: true,
	}
	m.action = action
	return m.withStatus(kind.presentParticiple() + " " + selectedMessageLabel(msg) + "..."), m.runMessageAction(backend, action)
}

func (m model) runMessageAction(backend messageActionBackend, action messageActionState) tea.Cmd {
	return func() tea.Msg {
		var err error
		if action.Kind == deleteAction {
			err = backend.DeleteMessage(context.Background(), action.Message)
		} else {
			err = backend.ArchiveMessage(context.Background(), action.Message)
		}
		return messageActionDoneMsg{action: action, err: err}
	}
}

func (m model) currentActionMessage() (message, int, bool) {
	if len(m.messages) == 0 {
		return message{}, 0, false
	}
	index := min(m.cursor, len(m.messages)-1)
	return m.messages[index], index, true
}

func (m model) removeMessage(id string) model {
	if strings.TrimSpace(id) == "" {
		return m
	}
	for i := range m.messages {
		if m.messages[i].ID == id {
			m.messages = append(m.messages[:i], m.messages[i+1:]...)
			if len(m.messages) == 0 {
				m.cursor = 0
				m.readerOffset = 0
				return m
			}
			if m.cursor >= len(m.messages) {
				m.cursor = len(m.messages) - 1
			}
			m.readerOffset = 0
			return m
		}
	}
	return m
}

func (m model) clearSearch() (model, tea.Cmd) {
	m.searching = false
	m.searchInput = ""
	m.searchQuery = ""
	return m.refreshInbox("clearing search...")
}

func (m model) clearMailboxFilter() (model, tea.Cmd) {
	m.mailboxFilter = allMailFilter
	m.mailboxCursor = m.activeMailboxEntryIndex()
	return m.refreshInbox("showing all mail in " + m.mailboxLabel() + "...")
}

func (m model) refreshInbox(status string) (model, tea.Cmd) {
	return m.reloadMailbox(status)
}

func (m model) reloadMailbox(status string) (model, tea.Cmd) {
	m.loading = true
	m.loadingMore = false
	m.loadedAll = false
	m.loadingMessageID = ""
	m.readerOffset = 0
	m.messages = nil
	m.cursor = 0
	m.loadSerial++
	m.status = status
	return m, m.loadInbox()
}

func filterMessages(messages []message, query string) []message {
	terms := searchTerms(query)
	if len(terms) == 0 {
		return messages
	}
	var filtered []message
	for _, msg := range messages {
		haystack := strings.ToLower(strings.Join([]string{msg.From, msg.Email, msg.Subject, msg.Preview, msg.Body}, " "))
		matched := true
		for _, term := range terms {
			if !strings.Contains(haystack, term) {
				matched = false
				break
			}
		}
		if matched {
			filtered = append(filtered, msg)
		}
	}
	return filtered
}

func filterUnreadMessages(messages []message) []message {
	var filtered []message
	for _, msg := range messages {
		if msg.Unread {
			filtered = append(filtered, msg)
		}
	}
	return filtered
}

func trimLastWord(value string) string {
	value = strings.TrimRight(value, " \t")
	index := strings.LastIndexAny(value, " \t")
	if index < 0 {
		return ""
	}
	return strings.TrimRight(value[:index], " \t")
}
