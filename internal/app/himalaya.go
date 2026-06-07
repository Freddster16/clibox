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
	binary := firstNonEmpty(options.Himalaya, os.Getenv("CLIBOX_HIMALAYA"), "himalaya")
	mailbox := firstNonEmpty(options.Mailbox, "INBOX")
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
	return strings.Join(nonEmpty("Himalaya", h.account, h.mailbox), " ")
}

func (h himalayaBackend) ListEnvelopes(ctx context.Context) ([]message, error) {
	if h.runner == nil {
		h.runner = osCommandRunner{}
	}

	var last commandFailure
	for _, args := range h.listCandidates() {
		stdout, stderr, err := h.runner.Run(ctx, h.binary, args)
		if err == nil {
			messages, parseErr := parseHimalayaMessages(stdout)
			if parseErr != nil {
				return nil, fmt.Errorf("Himalaya returned unreadable JSON for %s: %w", shellCommand(h.binary, args), parseErr)
			}
			return messages, nil
		}

		last = commandFailure{program: h.binary, args: args, stdout: stdout, stderr: stderr, err: err}
		if isMissingExecutable(err) || !looksLikeCommandShapeError(last.output()) {
			return nil, describeHimalayaFailure(last)
		}
	}
	return nil, describeHimalayaFailure(last)
}

func (h himalayaBackend) listCandidates() [][]string {
	size := strconv.Itoa(h.pageSize)
	v1 := appendFlags([]string{"envelope", "list", "--output", "json", "--page-size", size}, "--account", h.account, "--folder", h.mailbox)
	v2 := appendFlags([]string{"envelopes", "list", "--json", "--page-size", size}, "--account", h.account, "--mailbox", h.mailbox)
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
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, errors.New("empty output")
	}

	var raw any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return nil, err
	}

	envelopes := envelopeObjects(raw)
	messages := make([]message, 0, len(envelopes))
	for _, envelope := range envelopes {
		messages = append(messages, messageFromEnvelope(envelope, len(messages)+1))
	}
	return messages, nil
}

func messageFromEnvelope(envelope map[string]any, fallbackID int) message {
	flags := flagList(value(envelope, "flags"))
	fromName, fromEmail := address(value(envelope, "from", "sender"))
	fromName = firstNonEmpty(fromName, text(envelope, "from_name", "fromName", "sender_name", "senderName"), fromEmail, "Unknown")
	fromEmail = firstNonEmpty(fromEmail, text(envelope, "from_email", "fromEmail", "sender_email", "senderEmail"))

	preview := text(envelope, "preview", "snippet", "body_preview", "bodyPreview")
	if preview == "" && len(flags) > 0 {
		preview = "Flags: " + strings.Join(flags, ", ")
	}

	return message{
		ID:      firstNonEmpty(text(envelope, "id", "uid", "message_id", "messageId", "message-id"), strconv.Itoa(fallbackID)),
		From:    fromName,
		Email:   fromEmail,
		Subject: firstNonEmpty(text(envelope, "subject"), "(no subject)"),
		Date:    text(envelope, "date", "sent_at", "sentAt", "received_at", "receivedAt"),
		Preview: preview,
		Body:    "Message body loading arrives in Phase 3.",
		Unread:  isUnread(flags),
	}
}

func envelopeObjects(raw any) []map[string]any {
	switch value := raw.(type) {
	case []any:
		var out []map[string]any
		for _, item := range value {
			if obj, ok := item.(map[string]any); ok {
				out = append(out, obj)
			}
		}
		return out
	case map[string]any:
		if _, hasID := lookup(value, "id"); hasID {
			return []map[string]any{value}
		}
		if _, hasSubject := lookup(value, "subject"); hasSubject {
			return []map[string]any{value}
		}
		for _, key := range []string{"envelopes", "messages", "items", "results", "data", "result", "response"} {
			if nested, ok := lookup(value, key); ok {
				if found := envelopeObjects(nested); len(found) > 0 {
					return found
				}
			}
		}
	}
	return nil
}

func address(raw any) (string, string) {
	switch typed := raw.(type) {
	case []any:
		for _, item := range typed {
			if name, email := address(item); name != "" || email != "" {
				return name, email
			}
		}
	case map[string]any:
		name := text(typed, "name", "display_name", "displayName")
		email := text(typed, "address", "addr", "email", "mail")
		if email == "" {
			_, email = address(value(typed, "mailbox", "raw"))
		}
		return name, email
	case string:
		parsed, err := mail.ParseAddress(typed)
		if err == nil {
			return parsed.Name, parsed.Address
		}
		return strings.TrimSpace(typed), ""
	}
	return "", ""
}

func flagList(raw any) []string {
	add := func(flags []string, flag string) []string {
		if flag = strings.TrimSpace(flag); flag != "" {
			return append(flags, flag)
		}
		return flags
	}

	switch value := raw.(type) {
	case []any:
		flags := make([]string, 0, len(value))
		for _, item := range value {
			flags = add(flags, stringValue(item))
		}
		return flags
	case string:
		var flags []string
		for _, part := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' || r == '|' }) {
			flags = add(flags, part)
		}
		return flags
	}
	return nil
}

func isUnread(flags []string) bool {
	for _, flag := range flags {
		switch strings.TrimLeft(strings.ToLower(flag), "\\") {
		case "seen", "read":
			return false
		}
	}
	return len(flags) > 0
}

func text(obj map[string]any, keys ...string) string {
	return strings.TrimSpace(stringValue(value(obj, keys...)))
}

func value(obj map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := lookup(obj, key); ok {
			return value
		}
	}
	return nil
}

func lookup(obj map[string]any, key string) (any, bool) {
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

func stringValue(raw any) string {
	switch value := raw.(type) {
	case nil:
		return ""
	case string:
		return value
	case json.Number:
		return value.String()
	default:
		return fmt.Sprint(value)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func nonEmpty(values ...string) []string {
	var out []string
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func appendFlags(args []string, flagValues ...string) []string {
	for i := 0; i+1 < len(flagValues); i += 2 {
		if value := strings.TrimSpace(flagValues[i+1]); value != "" {
			args = append(args, flagValues[i], value)
		}
	}
	return args
}

func isMissingExecutable(err error) bool {
	return errors.Is(err, exec.ErrNotFound) || strings.Contains(err.Error(), "executable file not found")
}

func describeHimalayaFailure(failure commandFailure) error {
	if isMissingExecutable(failure.err) {
		return errors.New("Himalaya is not installed or not on PATH. Install and configure Himalaya, then run clibox again")
	}
	output := firstNonEmpty(failure.output(), failure.err.Error())
	return fmt.Errorf("%s failed: %s", shellCommand(failure.program, failure.args), oneLine(output))
}

func (f commandFailure) output() string {
	return strings.TrimSpace(strings.TrimSpace(string(f.stderr)) + "\n" + strings.TrimSpace(string(f.stdout)))
}

func looksLikeCommandShapeError(output string) bool {
	output = strings.ToLower(output)
	for _, needle := range []string{"unrecognized subcommand", "unrecognized option", "unexpected argument", "unknown command", "invalid subcommand", "did you mean"} {
		if strings.Contains(output, needle) {
			return true
		}
	}
	return false
}

func shellCommand(program string, args []string) string {
	return strings.Join(append([]string{program}, args...), " ")
}

func oneLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
