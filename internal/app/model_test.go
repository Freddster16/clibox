package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestInboxNavigation(t *testing.T) {
	m := New()

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
	m := New()
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
	m := New()

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
	m := New()

	_, cmd := m.Update(keyMsg("q"))
	if cmd == nil {
		t.Fatal("expected q from inbox to return a quit command")
	}
}

func TestPlannedActionsShowStatus(t *testing.T) {
	m := New()

	m = pressKey(t, m, "a")
	if m.status == "" {
		t.Fatal("expected planned archive action to show status")
	}

	m = pressKey(t, m, "j")
	if m.status != "" {
		t.Fatal("expected navigation to clear status")
	}
}

func TestThemeCycleShowsStatus(t *testing.T) {
	t.Setenv("CLIBOX_THEME", "")
	m := New()
	nextTheme := appThemes[(m.theme+1)%len(appThemes)].name

	m = pressKey(t, m, "t")
	if got := m.activeTheme().name; got != nextTheme {
		t.Fatalf("expected active theme %q, got %q", nextTheme, got)
	}
	if want := "theme switched to " + nextTheme; m.status != want {
		t.Fatalf("expected theme status %q, got %q", want, m.status)
	}
}

func TestThemeCanBeSelectedFromEnvironment(t *testing.T) {
	t.Setenv("CLIBOX_THEME", "lagoon")

	m := New()
	if got := m.activeTheme().name; got != "Lagoon" {
		t.Fatalf("expected CLIBOX_THEME to select Lagoon, got %q", got)
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

func keyMsg(key string) tea.KeyMsg {
	switch key {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}
