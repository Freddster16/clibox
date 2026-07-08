package app

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

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
	if index, ok := findMessageIndexByID(messages, id); ok {
		return index
	}
	return 0
}

func findMessageIndexByID(messages []message, id string) (int, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return 0, false
	}
	for i, msg := range messages {
		if strings.TrimSpace(msg.ID) == id {
			return i, true
		}
	}
	return 0, false
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
	return m.openSelectedMessageAtOffset(0)
}

func (m model) openSelectedMessageAtOffset(offset int) (model, tea.Cmd) {
	if len(m.messages) == 0 {
		return m.withStatus("No email selected"), nil
	}

	index := min(m.cursor, len(m.messages)-1)
	m.mode = readerView
	m.readerOffset = max(0, offset)
	wasUnread := m.messages[index].Unread
	m.messages[index].Unread = false
	msg := m.messages[index]
	markRead := m.markReadCmd(msg, wasUnread)
	if messageBodyReady(msg) {
		return m.withStatus("").withSessionSave(markRead)
	}
	if m.loadingMessageID == msg.ID {
		return m.withStatus("Loading email...").withSessionSave(markRead)
	}

	backend, ok := m.backend.(messageBodyBackend)
	if !ok {
		m.messages[index].BodyError = "This backend cannot load full emails yet."
		return m.withStatus("email body is not available yet").withSessionSave(markRead)
	}

	m.messageLoadSerial++
	m.loadingMessageID = msg.ID
	m.messages[index].BodyError = ""
	loadCmd := m.loadMessageBody(backend, msg, m.messageLoadSerial)
	if markRead == nil {
		return m.withStatus("Loading email...").withSessionSave(loadCmd)
	}
	return m.withStatus("Loading email...").withSessionSave(tea.Batch(loadCmd, markRead))
}

func (m model) markReadCmd(msg message, wasUnread bool) tea.Cmd {
	if !wasUnread {
		return nil
	}
	backend, ok := m.backend.(messageFlagBackend)
	if !ok {
		return nil
	}
	return func() tea.Msg {
		return messageMarkedReadMsg{err: backend.MarkMessageRead(context.Background(), msg)}
	}
}

func (m model) toggleRead() (model, tea.Cmd) {
	if len(m.messages) == 0 {
		return m.withStatus("No email selected"), nil
	}
	backend, ok := m.backend.(messageFlagBackend)
	if !ok {
		return m.withStatus("this backend cannot toggle read state yet"), nil
	}
	index := min(m.cursor, len(m.messages)-1)
	msg := m.messages[index]
	m.messages[index].Unread = !msg.Unread
	wantUnread := m.messages[index].Unread
	var cmd tea.Cmd
	if wantUnread {
		cmd = func() tea.Msg {
			return messageMarkedReadMsg{err: backend.MarkMessageUnread(context.Background(), msg)}
		}
	} else {
		cmd = func() tea.Msg {
			return messageMarkedReadMsg{err: backend.MarkMessageRead(context.Background(), msg)}
		}
	}
	return m.withStatus(""), cmd
}

func (m model) toggleFlagged() (model, tea.Cmd) {
	if len(m.messages) == 0 {
		return m.withStatus("No email selected"), nil
	}
	backend, ok := m.backend.(messageFlagBackend)
	if !ok {
		return m.withStatus("this backend cannot toggle flags yet"), nil
	}
	index := min(m.cursor, len(m.messages)-1)
	msg := m.messages[index]
	m.messages[index].Flagged = !msg.Flagged
	flagged := m.messages[index].Flagged
	cmd := func() tea.Msg {
		return messageFlagToggledMsg{flagged: flagged, err: backend.SetMessageFlagged(context.Background(), msg, flagged)}
	}
	return m.withStatus(""), cmd
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
		if contentBackend, ok := backend.(messageContentBackend); ok {
			content, err := contentBackend.ReadMessageContent(context.Background(), msg)
			return messageBodyLoadedMsg{id: msg.ID, body: content.Body, images: content.Images, notice: content.Notice, serial: serial, err: err}
		}
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
	return m.setMessageContent(id, messageContent{Body: body})
}

func (m model) setMessageContent(id string, content messageContent) model {
	for i := range m.messages {
		if m.messages[i].ID == id {
			m.messages[i].Body = content.Body
			m.messages[i].Images = content.Images
			m.messages[i].Notice = content.Notice
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
			m.messages[i].Images = nil
			m.messages[i].BodyLoaded = false
			m.messages[i].BodyError = message
			break
		}
	}
	return m
}

func (m model) messageBodyText(msg message, includePreview bool) string {
	var parts []string
	if includePreview && !messageBodyReady(msg) && strings.TrimSpace(msg.Preview) != "" {
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
	lines := m.renderMessageBodyLines(m.selectedMessage(), false, max(32, m.width))
	return max(0, len(lines)-bodyHeight)
}

func (m model) readerPageSize() int {
	return max(1, m.height-7)
}

func messageBodyReady(msg message) bool {
	return msg.BodyLoaded || strings.TrimSpace(msg.Body) != ""
}
