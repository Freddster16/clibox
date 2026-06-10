package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type draftKind int

const (
	composeDraft draftKind = iota
	replyDraft
)

type draftRequest struct {
	Kind    draftKind
	Message message
}

type draftState struct {
	Kind    draftKind
	Message message
	Path    string
	Content string
	Summary draftSummary
	Serial  int
	Sending bool
	Focus   draftField
	Cursor  int
}

type draftSummary struct {
	From    string
	To      string
	Cc      string
	Bcc     string
	Subject string
	Body    string
}

type draftField int

const (
	draftFieldTo draftField = iota
	draftFieldSubject
	draftFieldBody
	draftFieldCount
)

func (k draftKind) name() string {
	if k == replyDraft {
		return "reply"
	}
	return "message"
}

func (k draftKind) title() string {
	if k == replyDraft {
		return "Reply"
	}
	return "Compose"
}

func writeDraftFile(content string) (string, error) {
	file, err := os.CreateTemp("", "clibox-draft-*.eml")
	if err != nil {
		return "", err
	}

	path := file.Name()
	if err := file.Chmod(0o600); err != nil {
		closeErr := file.Close()
		_ = os.Remove(path)
		return "", errors.Join(err, closeErr)
	}

	if _, err := file.WriteString(ensureFinalNewline(normalizeDraftContent(content))); err != nil {
		closeErr := file.Close()
		_ = os.Remove(path)
		return "", errors.Join(err, closeErr)
	}

	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func readDraftFile(path string) (string, error) {
	if !isDraftFilePath(path) {
		return "", errors.New("draft path is outside clibox temporary drafts")
	}
	data, err := os.ReadFile(path) // #nosec G304 -- draft paths are validated against clibox's temp-file naming before reading.
	if err != nil {
		return "", err
	}
	return normalizeDraftContent(string(data)), nil
}

func saveDraftFile(path, content string) error {
	if !isDraftFilePath(path) {
		return errors.New("draft path is outside clibox temporary drafts")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("draft path must not be a symlink")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if err := file.Chmod(0o600); err != nil {
		closeErr := file.Close()
		return errors.Join(err, closeErr)
	}
	if _, err := file.WriteString(ensureFinalNewline(normalizeDraftContent(content))); err != nil {
		closeErr := file.Close()
		return errors.Join(err, closeErr)
	}
	return file.Close()
}

func removeDraftFile(path string) {
	if isDraftFilePath(path) {
		_ = os.Remove(path)
	}
}

func draftEditorCommand(path, configuredEditor string) (*exec.Cmd, error) {
	editor := firstNonEmpty(configuredEditor, os.Getenv("CLIBOX_EDITOR"), os.Getenv("VISUAL"), os.Getenv("EDITOR"), "nvim")
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return nil, errors.New("no editor configured")
	}

	resolved, err := exec.LookPath(parts[0])
	if err != nil {
		if parts[0] != "nvim" {
			return nil, fmt.Errorf("editor %q is not available", parts[0])
		}
		resolved, err = exec.LookPath("vi")
		if err != nil {
			return nil, fmt.Errorf("editor %q is not available; set EDITOR, VISUAL, or CLIBOX_EDITOR", parts[0])
		}
	}

	args := append(append([]string{}, parts[1:]...), path)
	return exec.Command(resolved, args...), nil // #nosec G204,G702 -- terminal email clients intentionally run the user's editor; the binary is resolved without a shell and arguments are not shell-evaluated.
}

func isDraftFilePath(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	clean := filepath.Clean(path)
	if filepath.Dir(clean) != filepath.Clean(os.TempDir()) {
		return false
	}
	base := filepath.Base(clean)
	return strings.HasPrefix(base, "clibox-draft-") && strings.HasSuffix(base, ".eml")
}

func parseDraftSummary(content string) draftSummary {
	content = normalizeDraftContent(content)
	headers, body, _ := strings.Cut(content, "\n\n")
	summary := draftSummary{Body: strings.TrimSpace(body)}

	current := ""
	for _, line := range strings.Split(headers, "\n") {
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if current != "" {
				appendDraftHeader(&summary, current, strings.TrimSpace(line))
			}
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			current = ""
			continue
		}
		current = strings.ToLower(strings.TrimSpace(key))
		setDraftHeader(&summary, current, strings.TrimSpace(value))
	}

	return summary
}

func setDraftHeader(summary *draftSummary, key, value string) {
	switch key {
	case "from":
		summary.From = value
	case "to":
		summary.To = value
	case "cc":
		summary.Cc = value
	case "bcc":
		summary.Bcc = value
	case "subject":
		summary.Subject = value
	}
}

func appendDraftHeader(summary *draftSummary, key, value string) {
	if value == "" {
		return
	}
	switch key {
	case "from":
		summary.From = joinHeaderContinuation(summary.From, value)
	case "to":
		summary.To = joinHeaderContinuation(summary.To, value)
	case "cc":
		summary.Cc = joinHeaderContinuation(summary.Cc, value)
	case "bcc":
		summary.Bcc = joinHeaderContinuation(summary.Bcc, value)
	case "subject":
		summary.Subject = joinHeaderContinuation(summary.Subject, value)
	}
}

func joinHeaderContinuation(existing, value string) string {
	if existing == "" {
		return value
	}
	return existing + " " + value
}

func validateDraftForSend(summary draftSummary) error {
	if strings.TrimSpace(summary.To) == "" && strings.TrimSpace(summary.Cc) == "" && strings.TrimSpace(summary.Bcc) == "" {
		return errors.New("add at least one recipient before sending")
	}
	return nil
}

func draftContentFromSummary(summary draftSummary) string {
	lines := []string{}
	if strings.TrimSpace(summary.From) != "" {
		lines = append(lines, "From: "+safeHeaderValue(summary.From))
	}
	lines = append(lines, "To: "+safeHeaderValue(summary.To))
	if strings.TrimSpace(summary.Cc) != "" {
		lines = append(lines, "Cc: "+safeHeaderValue(summary.Cc))
	}
	if strings.TrimSpace(summary.Bcc) != "" {
		lines = append(lines, "Bcc: "+safeHeaderValue(summary.Bcc))
	}
	lines = append(lines, "Subject: "+safeHeaderValue(summary.Subject), "", strings.TrimRight(normalizeDraftContent(summary.Body), "\n"))
	return ensureFinalNewline(strings.Join(lines, "\n"))
}

func draftTextLen(value string) int {
	return len([]rune(value))
}

func splitTextAt(value string, cursor int) (string, string) {
	runes := []rune(value)
	cursor = min(max(0, cursor), len(runes))
	return string(runes[:cursor]), string(runes[cursor:])
}

func insertTextAt(value, text string, cursor int) (string, int) {
	before, after := splitTextAt(value, cursor)
	return before + text + after, draftTextLen(before + text)
}

func textWithCursor(value string, cursor int) string {
	before, after := splitTextAt(value, cursor)
	return before + "_" + after
}

func deleteTextBefore(value string, cursor int) (string, int) {
	runes := []rune(value)
	cursor = min(max(0, cursor), len(runes))
	if cursor == 0 {
		return value, 0
	}
	next := append(append([]rune{}, runes[:cursor-1]...), runes[cursor:]...)
	return string(next), cursor - 1
}

func normalizeDraftContent(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	return strings.ReplaceAll(content, "\r", "\n")
}

func stripDraftHeader(content, header string) string {
	content = normalizeDraftContent(content)
	header = strings.ToLower(strings.TrimSpace(header))
	if header == "" {
		return content
	}

	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	inHeaders := true
	skipping := false
	for _, line := range lines {
		if inHeaders {
			if line == "" {
				inHeaders = false
				skipping = false
				out = append(out, line)
				continue
			}
			if skipping && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) {
				continue
			}
			skipping = false
			key, _, ok := strings.Cut(line, ":")
			if ok && strings.EqualFold(strings.TrimSpace(key), header) {
				skipping = true
				continue
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func smtpDraftContent(content string) string {
	return ensureFinalNewline(stripDraftHeader(content, "bcc"))
}

func normalizeDraftTemplate(data []byte) string {
	return ensureFinalNewline(strings.TrimRight(normalizeDraftContent(string(data)), " \t\r\n"))
}

func ensureFinalNewline(content string) string {
	if content == "" || strings.HasSuffix(content, "\n") {
		return content
	}
	return content + "\n"
}

func localComposeTemplate(from string) string {
	lines := []string{}
	if strings.TrimSpace(from) != "" {
		lines = append(lines, "From: "+safeHeaderValue(from))
	}
	lines = append(lines, "To: ", "Subject: ", "", "")
	return strings.Join(lines, "\n")
}

func localReplyTemplate(msg message, from string) string {
	subject := strings.TrimSpace(msg.Subject)
	if subject == "" {
		subject = "(no subject)"
	}
	if !strings.HasPrefix(strings.ToLower(subject), "re:") {
		subject = "Re: " + subject
	}

	lines := []string{}
	if strings.TrimSpace(from) != "" {
		lines = append(lines, "From: "+safeHeaderValue(from))
	}
	lines = append(lines, "To: "+safeHeaderValue(msg.Email), "Subject: "+safeHeaderValue(subject), "", "")

	quoted := terminalSafeText(firstNonEmpty(msg.Body, msg.Preview))
	if strings.TrimSpace(quoted) != "" {
		lines = append(lines, "")
		for _, line := range strings.Split(normalizeDraftContent(quoted), "\n") {
			lines = append(lines, "> "+line)
		}
	}

	return strings.Join(lines, "\n")
}

func safeHeaderValue(value string) string {
	value = strings.ReplaceAll(normalizeDraftContent(value), "\n", " ")
	return strings.Join(strings.Fields(value), " ")
}
