package app

import (
	"strings"
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

func TestThemeKeyOpensThemePicker(t *testing.T) {
	t.Setenv("CLIBOX_THEME", "")
	m := New()

	m = pressKey(t, m, "t")
	if !m.showThemes {
		t.Fatal("expected t to open the theme picker")
	}
	if m.themeCursor != m.theme {
		t.Fatalf("expected theme cursor to start on active theme, got cursor %d and theme %d", m.themeCursor, m.theme)
	}
}

func TestThemeCanBeSelectedFromEnvironment(t *testing.T) {
	t.Setenv("CLIBOX_THEME", "lagoon")

	m := New()
	if got := m.activeTheme().name; got != "Lagoon" {
		t.Fatalf("expected CLIBOX_THEME to select Lagoon, got %q", got)
	}
}

func TestCopperThemeAliasSelectsEmber(t *testing.T) {
	t.Setenv("CLIBOX_THEME", "copper")

	m := New()
	if got := m.activeTheme().name; got != "Ember" {
		t.Fatalf("expected copper alias to select Ember, got %q", got)
	}
}

func TestThemeCanBeSelectedFromConstructor(t *testing.T) {
	m := NewWithTheme("ember")
	if got := m.activeTheme().name; got != "Ember" {
		t.Fatalf("expected constructor theme Ember, got %q", got)
	}
}

func TestBlankConstructorThemeFallsBackToEnvironment(t *testing.T) {
	t.Setenv("CLIBOX_THEME", "lagoon")

	m := NewWithTheme("")
	if got := m.activeTheme().name; got != "Lagoon" {
		t.Fatalf("expected blank constructor theme to use environment, got %q", got)
	}
}

func TestUnknownThemeShowsFallbackStatus(t *testing.T) {
	m := NewWithTheme("banana")
	if got := m.activeTheme().name; got != "Nocturne" {
		t.Fatalf("expected unknown theme to fall back to Nocturne, got %q", got)
	}
	if !strings.Contains(m.status, `unknown theme "banana"`) {
		t.Fatalf("expected unknown theme status, got %q", m.status)
	}
}

func TestThemePickerPreviewsAndAppliesTheme(t *testing.T) {
	m := NewWithTheme("nocturne")

	m = pressKey(t, m, "t")
	m = pressKey(t, m, "j")
	if got := m.activeTheme().name; got != "Ember" {
		t.Fatalf("expected j in theme picker to preview Ember, got %q", got)
	}
	if !m.showThemes {
		t.Fatal("expected theme picker to stay open while previewing")
	}

	m = pressKey(t, m, "enter")
	if m.showThemes {
		t.Fatal("expected enter to close theme picker")
	}
	if got := m.activeTheme().name; got != "Ember" {
		t.Fatalf("expected enter to apply Ember, got %q", got)
	}
	if want := "theme Ember applied"; m.status != want {
		t.Fatalf("expected applied status %q, got %q", want, m.status)
	}
}

func TestThemePickerCanCancelPreview(t *testing.T) {
	m := NewWithTheme("nocturne")

	m = pressKey(t, m, "t")
	m = pressKey(t, m, "j")
	m = pressKey(t, m, "esc")
	if m.showThemes {
		t.Fatal("expected esc to close theme picker")
	}
	if got := m.activeTheme().name; got != "Nocturne" {
		t.Fatalf("expected esc to restore Nocturne, got %q", got)
	}
}

func TestThemePickerNumberAppliesTheme(t *testing.T) {
	m := NewWithTheme("nocturne")

	m = pressKey(t, m, "t")
	m = pressKey(t, m, "3")
	if got := m.activeTheme().name; got != "Lagoon" {
		t.Fatalf("expected number shortcut to apply Lagoon, got %q", got)
	}
	if m.showThemes {
		t.Fatal("expected number shortcut to close theme picker")
	}
}

func TestViewShowsThemeOnNarrowTerminal(t *testing.T) {
	m := NewWithTheme("lagoon")
	m.width = 34
	m.height = 10

	view := m.View()
	if !strings.Contains(view, "Lagoon") {
		t.Fatalf("expected narrow view to show active theme, got %q", view)
	}
}

func TestViewShowsThemePickerInsideTUI(t *testing.T) {
	m := NewWithTheme("lagoon")
	m.width = 72
	m.height = 22
	m = pressKey(t, m, "t")

	view := m.View()
	for _, want := range []string{"Themes", "Nocturne", "Ember", "Lagoon", "Enter applies"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected theme picker view to contain %q, got %q", want, view)
		}
	}
}

func TestThemeHelpListsCommands(t *testing.T) {
	help := ThemeHelp()
	for _, want := range []string{"nocturne", "ember", "lagoon", "clibox --theme lagoon", "theme picker"} {
		if !strings.Contains(help, want) {
			t.Fatalf("expected theme help to contain %q, got %q", want, help)
		}
	}
}

func TestThemesHaveDistinctVisibleSurfaces(t *testing.T) {
	seen := map[string]string{}
	for _, theme := range appThemes {
		signature := strings.Join([]string{
			theme.palette.background,
			theme.palette.header,
			theme.palette.surface,
			theme.palette.surfaceAlt,
			theme.palette.selected,
			theme.palette.footer,
		}, "|")
		if previous, ok := seen[signature]; ok {
			t.Fatalf("expected %s theme to differ from %s", theme.name, previous)
		}
		seen[signature] = theme.name
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
