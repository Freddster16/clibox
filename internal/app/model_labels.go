package app

import (
	"fmt"
	"os"
	"strings"
)

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
