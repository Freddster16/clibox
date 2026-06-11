package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) handleInboxPageLoaded(msg inboxPageLoadedMsg) (model, tea.Cmd) {
	if msg.serial != m.loadSerial {
		return m, nil
	}

	wasLoadingMore := m.loadingMore
	wasAtEnd := len(m.messages) == 0 || m.cursor >= len(m.messages)-1
	beforeCount := len(m.messages)
	m.loading = false
	m.loadingMore = false

	if msg.err != nil {
		return m.handleInboxPageError(msg)
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
		return m.finishInboxLoad()
	}

	m.loadedAll = false
	m.nextPage = msg.page + 1
	if wasLoadingMore && wasAtEnd && msg.page > 1 {
		return m.continueOlderMailLoad(addedCount)
	}
	m = m.withStatus(fmt.Sprintf("loaded %d emails from %s; press j at the bottom for older mail", len(m.messages), m.scopeLabel()))
	return m.previewSelectedMessage()
}

func (m model) handleInboxPageError(msg inboxPageLoadedMsg) (model, tea.Cmd) {
	if msg.page == 1 && isSetupRequiredError(msg.err) {
		return m.enterSetupRequired(msg.err), nil
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

func (m model) continueOlderMailLoad(addedCount int) (model, tea.Cmd) {
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

func (m model) handleInboxLoaded(msg inboxLoadedMsg) (model, tea.Cmd) {
	m.loading = false
	m.loadingMore = false
	m.loadedAll = true
	m.nextPage = 0
	if msg.err != nil {
		if isSetupRequiredError(msg.err) {
			return m.enterSetupRequired(msg.err), nil
		}
		m.status = msg.err.Error()
		m.messages = nil
		m.cursor = 0
		return m, nil
	}

	m.messages = msg.messages
	if len(m.messages) == 0 {
		m.cursor = 0
		return m.withStatus(m.emptyInboxStatus()), nil
	}
	if m.cursor >= len(m.messages) {
		m.cursor = len(m.messages) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m = m.withStatus(fmt.Sprintf("loaded %d emails from %s", len(m.messages), m.scopeLabel()))
	return m.previewSelectedMessage()
}

func (m model) finishInboxLoad() (model, tea.Cmd) {
	m.loadedAll = true
	m.nextPage = 0
	if len(m.messages) == 0 {
		return m.withStatus(m.emptyInboxStatus()), nil
	}
	m = m.withStatus(fmt.Sprintf("loaded %d emails from %s", len(m.messages), m.scopeLabel()))
	return m.previewSelectedMessage()
}

func (m model) emptyInboxStatus() string {
	if m.mailboxFilter == unreadMailFilter {
		return "No unread emails found in " + m.mailboxLabel()
	}
	if strings.TrimSpace(m.searchQuery) != "" {
		return "No emails matched " + m.searchQuery + " in " + m.mailboxLabel()
	}
	return "No emails found in " + m.mailboxLabel()
}

func (m model) enterSetupRequired(err error) model {
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
	status := err.Error()
	if m.setupStep == setupSecretStep {
		status = "paste your " + strings.ToLower(m.setupProvider.secretLabel()) + ", not your email address"
	}
	return m.withStatus(status)
}

func (m model) handleMessageBodyLoaded(msg messageBodyLoadedMsg) (model, tea.Cmd) {
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
	m = m.setMessageContent(msg.id, messageContent{Body: msg.body, Images: msg.images})
	if m.mode == readerView {
		return m.withStatus("email loaded"), nil
	}
	return m.withStatus(""), nil
}
