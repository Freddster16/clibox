package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const defaultHimalayaPageSize = 25

type inboxBackend interface {
	ListEnvelopes(context.Context) ([]message, error)
	Label() string
}

type himalayaBackend struct {
	binary   string
	account  string
	mailbox  string
	pageSize int
	runner   commandRunner
}

type commandRunner interface {
	Run(context.Context, string, []string) ([]byte, []byte, error)
}

type osCommandRunner struct{}

type commandFailure struct {
	program string
	args    []string
	stdout  []byte
	stderr  []byte
	err     error
}

func Doctor(ctx context.Context, options Options) (string, error) {
	backend := newHimalayaBackend(options)
	messages, err := backend.ListEnvelopes(ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Himalaya OK: loaded %d envelopes from %s", len(messages), backend.Label()), nil
}

func newHimalayaBackend(options Options) himalayaBackend {
	binary := strings.TrimSpace(options.Himalaya)
	if binary == "" {
		binary = strings.TrimSpace(os.Getenv("CLIBOX_HIMALAYA"))
	}
	if binary == "" {
		binary = "himalaya"
	}

	mailbox := strings.TrimSpace(options.Mailbox)
	if mailbox == "" {
		mailbox = "INBOX"
	}

	pageSize := options.PageSize
	if pageSize <= 0 {
		pageSize = defaultHimalayaPageSize
	}

	return himalayaBackend{
		binary:   binary,
		account:  strings.TrimSpace(options.Account),
		mailbox:  mailbox,
		pageSize: pageSize,
		runner:   osCommandRunner{},
	}
}

func (h himalayaBackend) Label() string {
	parts := []string{"Himalaya"}
	if h.account != "" {
		parts = append(parts, h.account)
	}
	if h.mailbox != "" {
		parts = append(parts, h.mailbox)
	}
	return strings.Join(parts, " ")
}

func (h himalayaBackend) ListEnvelopes(ctx context.Context) ([]message, error) {
	if h.runner == nil {
		h.runner = osCommandRunner{}
	}

	var failures []commandFailure
	for _, args := range h.listCandidates() {
		stdout, stderr, err := h.runner.Run(ctx, h.binary, args)
		if err != nil {
			failure := commandFailure{
				program: h.binary,
				args:    args,
				stdout:  stdout,
				stderr:  stderr,
				err:     err,
			}
			failures = append(failures, failure)
			if isMissingExecutable(err) || !looksLikeCommandShapeError(failure.output()) {
				return nil, describeHimalayaFailure(failure)
			}
			continue
		}

		messages, parseErr := parseHimalayaMessages(stdout)
		if parseErr != nil {
			return nil, fmt.Errorf("Himalaya returned unreadable JSON for %s: %w", shellCommand(h.binary, args), parseErr)
		}
		return messages, nil
	}

	if len(failures) == 0 {
		return nil, errors.New("no Himalaya envelope command candidates were configured")
	}
	return nil, describeHimalayaFailure(failures[len(failures)-1])
}

func (h himalayaBackend) listCandidates() [][]string {
	account := h.account
	mailbox := h.mailbox
	pageSize := strconv.Itoa(h.pageSize)

	v1 := []string{"envelope", "list", "--output", "json", "--page-size", pageSize}
	if account != "" {
		v1 = append(v1, "--account", account)
	}
	if mailbox != "" {
		v1 = append(v1, "--folder", mailbox)
	}

	v2 := []string{"envelopes", "list", "--json", "--page-size", pageSize}
	if account != "" {
		v2 = append(v2, "--account", account)
	}
	if mailbox != "" {
		v2 = append(v2, "--mailbox", mailbox)
	}

	return [][]string{v1, v2}
}

func (r osCommandRunner) Run(ctx context.Context, program string, args []string) ([]byte, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	return stdout.Bytes(), stderr.Bytes(), err
}

func parseHimalayaMessages(data []byte) ([]message, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, errors.New("empty output")
	}

	var decoded any
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return nil, err
	}

	items := extractEnvelopeItems(decoded)
	messages := make([]message, 0, len(items))
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}

		flags := parseFlags(firstValue(obj, "flags"))
		fromName, fromEmail := parseAddressValue(firstValue(obj, "from", "sender"))
		if fromName == "" {
			fromName = firstString(obj, "from_name", "fromName", "sender_name", "senderName")
		}
		if fromEmail == "" {
			fromEmail = firstString(obj, "from_email", "fromEmail", "sender_email", "senderEmail")
		}
		if fromName == "" && fromEmail != "" {
			fromName = fromEmail
		}
		if fromName == "" {
			fromName = "Unknown"
		}

		id := firstString(obj, "id", "uid", "message_id", "messageId", "message-id")
		if id == "" {
			id = strconv.Itoa(len(messages) + 1)
		}

		subject := firstString(obj, "subject")
		if subject == "" {
			subject = "(no subject)"
		}

		preview := firstString(obj, "preview", "snippet", "body_preview", "bodyPreview")
		if preview == "" && len(flags) > 0 {
			preview = "Flags: " + strings.Join(flags, ", ")
		}

		messages = append(messages, message{
			ID:      id,
			From:    fromName,
			Email:   fromEmail,
			Subject: subject,
			Date:    firstString(obj, "date", "sent_at", "sentAt", "received_at", "receivedAt"),
			Preview: preview,
			Body:    "Message body loading arrives in Phase 3.",
			Unread:  flagsKnownAsUnread(flags),
		})
	}

	return messages, nil
}

func extractEnvelopeItems(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case map[string]any:
		if looksLikeEnvelope(typed) {
			return []any{typed}
		}
		for _, key := range []string{"envelopes", "messages", "items", "results", "data", "result", "response"} {
			if nested, ok := getCaseInsensitive(typed, key); ok {
				if items := extractEnvelopeItems(nested); len(items) > 0 {
					return items
				}
			}
		}
	}
	return nil
}

func looksLikeEnvelope(obj map[string]any) bool {
	_, hasID := getCaseInsensitive(obj, "id")
	_, hasSubject := getCaseInsensitive(obj, "subject")
	return hasID || hasSubject
}

func parseFlags(value any) []string {
	switch typed := value.(type) {
	case []any:
		flags := make([]string, 0, len(typed))
		for _, item := range typed {
			if flag := strings.TrimSpace(valueToString(item)); flag != "" {
				flags = append(flags, flag)
			}
		}
		return flags
	case string:
		parts := strings.FieldsFunc(typed, func(r rune) bool {
			return r == ',' || r == ' ' || r == '|'
		})
		flags := make([]string, 0, len(parts))
		for _, part := range parts {
			if flag := strings.TrimSpace(part); flag != "" {
				flags = append(flags, flag)
			}
		}
		return flags
	default:
		return nil
	}
}

func flagsKnownAsUnread(flags []string) bool {
	for _, flag := range flags {
		normalized := strings.TrimLeft(strings.ToLower(flag), "\\")
		switch normalized {
		case "seen", "read":
			return false
		}
	}
	return len(flags) > 0
}

func parseAddressValue(value any) (string, string) {
	switch typed := value.(type) {
	case []any:
		if len(typed) == 0 {
			return "", ""
		}
		return parseAddressValue(typed[0])
	case map[string]any:
		name := firstString(typed, "name", "display_name", "displayName")
		email := firstString(typed, "address", "addr", "email", "mail")
		if email == "" {
			_, email = parseAddressValue(firstValue(typed, "mailbox", "raw"))
		}
		return name, email
	case string:
		parsed, err := mail.ParseAddress(typed)
		if err == nil {
			return parsed.Name, parsed.Address
		}
		return strings.TrimSpace(typed), ""
	default:
		return "", ""
	}
}

func firstString(obj map[string]any, keys ...string) string {
	value := firstValue(obj, keys...)
	return strings.TrimSpace(valueToString(value))
}

func firstValue(obj map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := getCaseInsensitive(obj, key); ok {
			return value
		}
	}
	return nil
}

func getCaseInsensitive(obj map[string]any, key string) (any, bool) {
	if value, ok := obj[key]; ok {
		return value, true
	}
	for existing, value := range obj {
		if strings.EqualFold(existing, key) {
			return value, true
		}
	}
	return nil, false
}

func valueToString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(typed)
	}
}

func isMissingExecutable(err error) bool {
	return errors.Is(err, exec.ErrNotFound) || strings.Contains(err.Error(), "executable file not found")
}

func describeHimalayaFailure(failure commandFailure) error {
	if isMissingExecutable(failure.err) {
		return fmt.Errorf("Himalaya is not installed or not on PATH. Install and configure Himalaya, then run clibox again")
	}

	output := strings.TrimSpace(failure.output())
	if output == "" {
		output = strings.TrimSpace(failure.err.Error())
	}
	return fmt.Errorf("%s failed: %s", shellCommand(failure.program, failure.args), oneLine(output))
}

func (f commandFailure) output() string {
	parts := []string{
		strings.TrimSpace(string(f.stderr)),
		strings.TrimSpace(string(f.stdout)),
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func looksLikeCommandShapeError(output string) bool {
	lower := strings.ToLower(output)
	needles := []string{
		"unrecognized subcommand",
		"unrecognized option",
		"unexpected argument",
		"unknown command",
		"invalid subcommand",
		"the subcommand",
		"wasn't recognized",
		"did you mean",
	}
	for _, needle := range needles {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func shellCommand(program string, args []string) string {
	parts := append([]string{program}, args...)
	return strings.Join(parts, " ")
}

func oneLine(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	for strings.Contains(value, "  ") {
		value = strings.ReplaceAll(value, "  ", " ")
	}
	return strings.TrimSpace(value)
}
