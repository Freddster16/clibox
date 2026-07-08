package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type himalayaBackend struct {
	binary                string
	account               string
	mailbox               string
	archiveFolder         string
	archiveFolderExplicit bool
	trashFolder           string
	trashFolderExplicit   bool
	pageSize              int
	runner                commandRunner
}

type commandRunner interface {
	Run(context.Context, string, []string) ([]byte, []byte, error)
	RunInput(context.Context, string, []string, string) ([]byte, []byte, error)
}

type osCommandRunner struct{}

type commandFailure struct {
	program string
	args    []string
	stdout  []byte
	stderr  []byte
	err     error
}

func (h himalayaBackend) Diagnose(ctx context.Context) (string, error) {
	if h.runner == nil {
		h.runner = osCommandRunner{}
	}

	version := "unknown"
	stdout, stderr, err := h.runner.Run(ctx, h.binary, []string{"--version"})
	if err != nil {
		failure := commandFailure{program: h.binary, args: []string{"--version"}, stdout: stdout, stderr: stderr, err: err}
		if isMissingExecutable(err) {
			return "", describeHimalayaFailure(failure)
		}
		version = "unknown (" + oneLine(firstNonEmpty(failure.output(), err.Error())) + ")"
	} else {
		version = oneLine(firstNonEmpty(string(stdout), string(stderr), h.binary))
	}

	configPath, configErr := himalayaConfigPath()
	configLabel := "unknown"
	if configErr != nil {
		configLabel = "unavailable: " + oneLine(configErr.Error())
	} else {
		configLabel = configPath
	}

	account := h.account
	if hint, ok := himalayaAccountHint(h.account); ok {
		account = firstNonEmpty(account, hint.Account)
	}
	account = firstNonEmpty(account, "default")

	messages, done, err := h.ListEnvelopePage(ctx, 1)
	if err != nil {
		return "", err
	}
	older := ""
	if !done {
		older = " (older mail may still be available)"
	}

	lines := []string{
		"Email connection OK",
		"Backend: " + version,
		"Config: " + configLabel,
		"Account: " + account,
		"Mailbox: " + h.mailbox,
		fmt.Sprintf("First page: %d emails%s", len(messages), older),
	}
	return strings.Join(lines, "\n"), nil
}

func newHimalayaBackend(options Options) himalayaBackend {
	binary := firstNonEmpty(options.Himalaya, os.Getenv("CLIBOX_HIMALAYA"), "himalaya")
	mailbox := firstNonEmpty(options.Mailbox, "INBOX")
	pageSize := options.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	archiveFolder := firstNonEmpty(options.ArchiveFolder, os.Getenv("CLIBOX_ARCHIVE_FOLDER"))
	trashFolder := strings.TrimSpace(os.Getenv("CLIBOX_TRASH_FOLDER"))
	archiveExplicit := strings.TrimSpace(archiveFolder) != ""
	trashExplicit := strings.TrimSpace(trashFolder) != ""
	if !archiveExplicit || !trashExplicit {
		if hint, ok := himalayaAccountHint(options.Account); ok {
			if !archiveExplicit {
				archiveFolder = hint.Provider.Folders["archive"]
			}
			if !trashExplicit {
				trashFolder = hint.Provider.Folders["trash"]
			}
		}
	}

	return himalayaBackend{
		binary:                binary,
		account:               strings.TrimSpace(options.Account),
		mailbox:               mailbox,
		archiveFolder:         firstNonEmpty(archiveFolder, "Archive"),
		archiveFolderExplicit: archiveExplicit,
		trashFolder:           firstNonEmpty(trashFolder, "Trash"),
		trashFolderExplicit:   trashExplicit,
		pageSize:              pageSize,
		runner:                osCommandRunner{},
	}
}

func (h himalayaBackend) Label() string {
	return strings.Join(nonEmpty(h.account, h.mailbox), " ")
}

func (h himalayaBackend) WithAccount(account string) inboxBackend {
	h.account = strings.TrimSpace(account)
	if hint, ok := himalayaAccountHint(h.account); ok {
		if !h.archiveFolderExplicit {
			h.archiveFolder = firstNonEmpty(hint.Provider.Folders["archive"], "Archive")
		}
		if !h.trashFolderExplicit {
			h.trashFolder = firstNonEmpty(hint.Provider.Folders["trash"], "Trash")
		}
	}
	return h
}

func (h himalayaBackend) WithMailbox(mailbox string) inboxBackend {
	h.mailbox = firstNonEmpty(mailbox, "INBOX")
	return h
}

func (h himalayaBackend) ListEnvelopes(ctx context.Context) ([]message, error) {
	var messages []message
	for page := 1; ; page++ {
		pageMessages, done, err := h.ListEnvelopePage(ctx, page)
		if err != nil {
			return nil, err
		}
		messages = append(messages, pageMessages...)
		if done {
			return messages, nil
		}
	}
}

func (h himalayaBackend) ListEnvelopePage(ctx context.Context, page int) ([]message, bool, error) {
	return h.listEnvelopePage(ctx, page, "")
}

func (h himalayaBackend) SearchEnvelopePage(ctx context.Context, page int, query string) ([]message, bool, error) {
	return h.listEnvelopePage(ctx, page, buildSearchQuery(query))
}

func (h himalayaBackend) listEnvelopePage(ctx context.Context, page int, query string) ([]message, bool, error) {
	if h.runner == nil {
		h.runner = osCommandRunner{}
	}

	var last commandFailure
	for _, args := range h.listCandidates(page, query) {
		stdout, stderr, err := h.runner.Run(ctx, h.binary, args)
		if err == nil {
			messages, parseErr := parseHimalayaMessagesPage(stdout, page, h.pageSize)
			if parseErr != nil {
				return nil, false, fmt.Errorf("email backend returned unreadable data: %w", parseErr)
			}
			done := len(messages) == 0 || (h.pageSize > 0 && len(messages) < h.pageSize)
			return messages, done, nil
		}

		last = commandFailure{program: h.binary, args: args, stdout: stdout, stderr: stderr, err: err}
		if isMissingExecutable(err) {
			return nil, false, describeHimalayaFailure(last)
		}
		if looksLikePageEndError(last.output()) {
			return nil, true, nil
		}
		if !looksLikeCommandShapeError(last.output()) {
			return nil, false, describeHimalayaFailure(last)
		}
	}
	return nil, false, describeHimalayaFailure(last)
}

func (h himalayaBackend) ArchiveMessage(ctx context.Context, msg message) error {
	return h.runMessageAction(ctx, msg, h.archiveCandidates)
}

func (h himalayaBackend) DeleteMessage(ctx context.Context, msg message) error {
	return h.runMessageAction(ctx, msg, h.deleteCandidates)
}

func (h himalayaBackend) MarkMessageRead(ctx context.Context, msg message) error {
	return h.runMessageAction(ctx, msg, h.flagSeenCandidates)
}

func (h himalayaBackend) MarkMessageUnread(ctx context.Context, msg message) error {
	return h.runMessageAction(ctx, msg, h.flagUnseenCandidates)
}

func (h himalayaBackend) SetMessageFlagged(ctx context.Context, msg message, flagged bool) error {
	if flagged {
		return h.runMessageAction(ctx, msg, h.flagFlaggedCandidates)
	}
	return h.runMessageAction(ctx, msg, h.flagUnflaggedCandidates)
}

func (h himalayaBackend) runMessageAction(ctx context.Context, msg message, candidates func(string) [][]string) error {
	if h.runner == nil {
		h.runner = osCommandRunner{}
	}

	id := strings.TrimSpace(msg.ID)
	if id == "" {
		return errors.New("email has no readable id")
	}

	var last commandFailure
	for _, args := range candidates(id) {
		stdout, stderr, err := h.runner.Run(ctx, h.binary, args)
		if err == nil {
			return nil
		}

		last = commandFailure{program: h.binary, args: args, stdout: stdout, stderr: stderr, err: err}
		if isMissingExecutable(err) {
			return describeHimalayaFailure(last)
		}
		if !looksLikeCommandShapeError(last.output()) {
			return describeHimalayaFailure(last)
		}
	}
	return describeHimalayaFailure(last)
}

func (h himalayaBackend) ReadMessage(ctx context.Context, msg message) (string, error) {
	if h.runner == nil {
		h.runner = osCommandRunner{}
	}

	id := strings.TrimSpace(msg.ID)
	if id == "" {
		return "", errors.New("email has no readable id")
	}

	var last commandFailure
	for _, args := range h.readCandidates(id) {
		stdout, stderr, err := h.runner.Run(ctx, h.binary, args)
		if err == nil {
			body := normalizeMessageBody(stdout)
			if body == "" {
				body = "(empty message)"
			}
			return body, nil
		}

		last = commandFailure{program: h.binary, args: args, stdout: stdout, stderr: stderr, err: err}
		if isMissingExecutable(err) {
			return "", describeHimalayaFailure(last)
		}
		if !looksLikeCommandShapeError(last.output()) {
			return "", describeHimalayaFailure(last)
		}
	}
	return "", describeHimalayaFailure(last)
}

func (h himalayaBackend) PrepareDraft(ctx context.Context, req draftRequest) (string, error) {
	if h.runner == nil {
		h.runner = osCommandRunner{}
	}

	var candidates [][]string
	var fallback string
	switch req.Kind {
	case replyDraft:
		fallback = localReplyTemplate(req.Message, h.defaultFrom())
		id := strings.TrimSpace(req.Message.ID)
		if id == "" {
			return fallback, nil
		}
		candidates = h.replyDraftCandidates(id)
	case forwardDraft:
		fallback = localForwardTemplate(req.Message, h.defaultFrom())
		id := strings.TrimSpace(req.Message.ID)
		if id == "" {
			return fallback, nil
		}
		candidates = h.forwardDraftCandidates(id)
	default:
		fallback = localComposeTemplate(h.defaultFrom())
		candidates = h.composeDraftCandidates()
	}

	var last commandFailure
	for _, args := range candidates {
		stdout, stderr, err := h.runner.Run(ctx, h.binary, args)
		if err == nil {
			return normalizeDraftTemplate(stdout), nil
		}

		last = commandFailure{program: h.binary, args: args, stdout: stdout, stderr: stderr, err: err}
		if isMissingExecutable(err) {
			return "", describeHimalayaFailure(last)
		}
		if !looksLikeCommandShapeError(last.output()) {
			return "", describeHimalayaFailure(last)
		}
	}

	return fallback, nil
}

func (h himalayaBackend) SendDraft(ctx context.Context, content string) error {
	if h.runner == nil {
		h.runner = osCommandRunner{}
	}
	if strings.TrimSpace(content) == "" {
		return errors.New("draft is empty")
	}

	var last commandFailure
	for _, args := range h.sendDraftCandidates() {
		stdout, stderr, err := h.runner.RunInput(ctx, h.binary, args, ensureFinalNewline(content))
		if err == nil {
			return nil
		}

		last = commandFailure{program: h.binary, args: args, stdout: stdout, stderr: stderr, err: err}
		if isMissingExecutable(err) {
			return describeHimalayaFailure(last)
		}
		if !looksLikeCommandShapeError(last.output()) {
			return describeHimalayaFailure(last)
		}
	}
	return describeHimalayaFailure(last)
}

func (h himalayaBackend) listCandidates(page int, query string) [][]string {
	pageNumber := strconv.Itoa(page)
	v1 := []string{"envelope", "list", "--output", "json", "--page", pageNumber}
	v2Command := "list"
	if strings.TrimSpace(query) != "" {
		v2Command = "search"
	}
	v2 := []string{"envelopes", v2Command, "--json", "--page", pageNumber}
	if h.pageSize > 0 {
		size := strconv.Itoa(h.pageSize)
		v1 = append(v1, "--page-size", size)
		v2 = append(v2, "--page-size", size)
	}
	v1 = appendFlags(v1, "--account", h.account, "--folder", h.mailbox)
	v2 = appendFlags(v2, "--account", h.account, "--mailbox", h.mailbox)
	if strings.TrimSpace(query) != "" {
		v1 = append(v1, query)
		v2 = append(v2, query)
	}
	return [][]string{v1, v2}
}

func (h himalayaBackend) readCandidates(id string) [][]string {
	v1 := appendFlags([]string{"message", "read", "--preview", "--no-headers"}, "--account", h.account, "--folder", h.mailbox)
	v1 = append(v1, id)

	v2 := []string{"messages", "read", "--preview", "--no-headers"}
	if strings.TrimSpace(h.account) != "" {
		v2 = append(v2, "-a", h.account)
	}
	if strings.TrimSpace(h.mailbox) != "" {
		v2 = append(v2, "-m", h.mailbox)
	}
	v2 = append(v2, id)

	return [][]string{v1, v2}
}

func (h himalayaBackend) composeDraftCandidates() [][]string {
	v1 := appendFlags([]string{"template", "write", "-H", "To:", "-H", "Subject:"}, "--account", h.account)
	return [][]string{v1}
}

func (h himalayaBackend) replyDraftCandidates(id string) [][]string {
	v1 := appendFlags([]string{"template", "reply"}, "--account", h.account, "--folder", h.mailbox)
	v1 = append(v1, id)
	return [][]string{v1}
}

func (h himalayaBackend) forwardDraftCandidates(id string) [][]string {
	v1 := appendFlags([]string{"template", "forward"}, "--account", h.account, "--folder", h.mailbox)
	v1 = append(v1, id)
	return [][]string{v1}
}

func (h himalayaBackend) sendDraftCandidates() [][]string {
	v1 := appendFlags([]string{"template", "send"}, "--account", h.account)

	v2 := []string{"messages", "send"}
	if strings.TrimSpace(h.account) != "" {
		v2 = append(v2, "-a", h.account)
	}

	return [][]string{v1, v2}
}

func (h himalayaBackend) archiveCandidates(id string) [][]string {
	v1 := appendFlags([]string{"message", "move", h.archiveFolder}, "--account", h.account, "--folder", h.mailbox)
	v1 = append(v1, id)

	v2 := []string{"messages", "move", id}
	v2 = appendFlags(v2, "--account", h.account, "--from", h.mailbox, "--to", h.archiveFolder)

	return [][]string{v1, v2}
}

func (h himalayaBackend) deleteCandidates(id string) [][]string {
	v1 := appendFlags([]string{"message", "delete"}, "--account", h.account, "--folder", h.mailbox)
	v1 = append(v1, id)

	v2 := []string{"messages", "move", id}
	v2 = appendFlags(v2, "--account", h.account, "--from", h.mailbox, "--to", h.trashFolder)

	return [][]string{v1, v2}
}

func (h himalayaBackend) flagSeenCandidates(id string) [][]string {
	v1 := appendFlags([]string{"flag", "add", id, "--flag", "seen"}, "--account", h.account, "--folder", h.mailbox)

	v2 := appendFlags([]string{"flag", "add", "--flag", "seen", id}, "--account", h.account, "--mailbox", h.mailbox)

	return [][]string{v1, v2}
}

func (h himalayaBackend) flagUnseenCandidates(id string) [][]string {
	v1 := appendFlags([]string{"flag", "remove", id, "--flag", "seen"}, "--account", h.account, "--folder", h.mailbox)

	v2 := appendFlags([]string{"flag", "remove", "--flag", "seen", id}, "--account", h.account, "--mailbox", h.mailbox)

	return [][]string{v1, v2}
}

func (h himalayaBackend) flagFlaggedCandidates(id string) [][]string {
	v1 := appendFlags([]string{"flag", "add", id, "--flag", "flagged"}, "--account", h.account, "--folder", h.mailbox)

	v2 := appendFlags([]string{"flag", "add", "--flag", "flagged", id}, "--account", h.account, "--mailbox", h.mailbox)

	return [][]string{v1, v2}
}

func (h himalayaBackend) flagUnflaggedCandidates(id string) [][]string {
	v1 := appendFlags([]string{"flag", "remove", id, "--flag", "flagged"}, "--account", h.account, "--folder", h.mailbox)

	v2 := appendFlags([]string{"flag", "remove", "--flag", "flagged", id}, "--account", h.account, "--mailbox", h.mailbox)

	return [][]string{v1, v2}
}

func (h himalayaBackend) defaultFrom() string {
	hint, ok := himalayaAccountHint(h.account)
	if !ok || strings.TrimSpace(hint.Email) == "" {
		return ""
	}
	if strings.TrimSpace(hint.DisplayName) == "" {
		return hint.Email
	}
	return fmt.Sprintf("%s <%s>", strings.TrimSpace(hint.DisplayName), strings.TrimSpace(hint.Email))
}

func (r osCommandRunner) Run(ctx context.Context, program string, args []string) ([]byte, []byte, error) {
	return r.run(ctx, program, args, "", false, 30*time.Second)
}

func (r osCommandRunner) RunInput(ctx context.Context, program string, args []string, input string) ([]byte, []byte, error) {
	return r.run(ctx, program, args, input, true, 2*time.Minute)
}

func (r osCommandRunner) run(ctx context.Context, program string, args []string, input string, hasInput bool, timeout time.Duration) ([]byte, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, program, args...) // #nosec G204 -- backend binary is user-configurable, and args are not shell-evaluated.
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	if hasInput {
		cmd.Stdin = strings.NewReader(input)
	}
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
	return parseHimalayaMessagesWithFallback(data, func(index int) string {
		return strconv.Itoa(index + 1)
	})
}

func parseHimalayaMessagesPage(data []byte, page, pageSize int) ([]message, error) {
	page = max(1, page)
	if pageSize <= 0 {
		pageSize = 50
	}
	base := (page - 1) * pageSize
	return parseHimalayaMessagesWithFallback(data, func(index int) string {
		return strconv.Itoa(base + index + 1)
	})
}

func parseHimalayaMessagesWithFallback(data []byte, fallbackID func(int) string) ([]message, error) {
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
		messages = append(messages, messageFromEnvelope(envelope, fallbackID(len(messages))))
	}
	return messages, nil
}

func messageFromEnvelope(envelope map[string]any, fallbackID string) message {
	flags := flagList(value(envelope, "flags"))
	fromName, fromEmail := address(value(envelope, "from", "sender"))
	fromName = firstNonEmpty(fromName, text(envelope, "from_name", "fromName", "sender_name", "senderName"), fromEmail, "Unknown")
	fromEmail = firstNonEmpty(fromEmail, text(envelope, "from_email", "fromEmail", "sender_email", "senderEmail"))

	preview := text(envelope, "preview", "snippet", "body_preview", "bodyPreview")
	if preview == "" && len(flags) > 0 {
		preview = "Flags: " + strings.Join(flags, ", ")
	}

	return message{
		ID:      firstNonEmpty(text(envelope, "id", "uid", "message_id", "messageId", "message-id"), fallbackID),
		From:    fromName,
		Email:   fromEmail,
		Subject: firstNonEmpty(text(envelope, "subject"), "(no subject)"),
		Date:    text(envelope, "date", "sent_at", "sentAt", "received_at", "receivedAt"),
		Preview: preview,
		Unread:  isUnread(flags),
		Flagged: isFlagged(flags),
	}
}

func normalizeMessageBody(data []byte) string {
	body := strings.ReplaceAll(string(data), "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	lines := stripHimalayaReadDecorations(strings.Split(body, "\n"))
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func stripHimalayaReadDecorations(lines []string) []string {
	start := skipHimalayaReadMarkers(lines, 0)
	if end, ok := leadingMailHeaderBlock(lines[start:]); ok {
		start += end
		start = skipHimalayaReadMarkers(lines, start)
	}

	out := make([]string, 0, len(lines)-start)
	for _, line := range lines[start:] {
		if isHimalayaReadMarker(strings.TrimSpace(line)) {
			continue
		}
		out = append(out, line)
	}
	return trimBlankLines(out)
}

func skipHimalayaReadMarkers(lines []string, start int) int {
	for start < len(lines) {
		line := strings.TrimSpace(lines[start])
		if line == "" || isHimalayaReadMarker(line) {
			start++
			continue
		}
		break
	}
	return start
}

func leadingMailHeaderBlock(lines []string) (int, bool) {
	seen := map[string]struct{}{}
	count := 0
	lastWasHeader := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if count == 0 {
				continue
			}
			return i + 1, len(seen) >= 2
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if lastWasHeader {
				continue
			}
			return i, len(seen) >= 2
		}
		name, ok := mailHeaderName(trimmed)
		if !ok {
			return i, len(seen) >= 2
		}
		seen[name] = struct{}{}
		count++
		lastWasHeader = true
	}
	return len(lines), len(seen) >= 2
}

func mailHeaderName(line string) (string, bool) {
	name, _, ok := strings.Cut(line, ":")
	if !ok {
		return "", false
	}
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return "", false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return "", false
	}
	switch name {
	case "from", "to", "cc", "bcc", "reply-to", "sender", "subject", "date",
		"message-id", "in-reply-to", "references", "mime-version", "content-type",
		"content-transfer-encoding", "delivered-to", "return-path", "received",
		"dkim-signature", "authentication-results", "list-id", "list-unsubscribe",
		"precedence":
		return name, true
	default:
		return "", false
	}
}

func isHimalayaReadMarker(line string) bool {
	return strings.HasPrefix(line, "<#part ") && strings.HasSuffix(line, ">")
}

func trimBlankLines(lines []string) []string {
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return lines[start:end]
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
		return parseAddressString(typed)
	}
	return "", ""
}

func parseAddressString(raw string) (string, string) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", ""
	}
	if len(value) > 4096 {
		value = value[:4096]
	}

	if start := strings.LastIndex(value, "<"); start >= 0 {
		if end := strings.Index(value[start:], ">"); end > 0 {
			email := strings.TrimSpace(value[start+1 : start+end])
			name := cleanAddressName(value[:start])
			if validEmailAddress(email) {
				return name, email
			}
		}
	}

	fields := strings.Fields(value)
	for _, field := range fields {
		candidate := strings.Trim(field, "<>(),;")
		if validEmailAddress(candidate) {
			return "", candidate
		}
	}
	if validEmailAddress(value) {
		return "", value
	}
	return cleanAddressName(value), ""
}

func cleanAddressName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	return strings.TrimSpace(value)
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

func isFlagged(flags []string) bool {
	for _, flag := range flags {
		switch strings.TrimLeft(strings.ToLower(flag), "\\") {
		case "flagged", "\\flagged":
			return true
		}
	}
	return false
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
		return errors.New("email backend is not installed yet. Run the clibox installer, then open clibox again")
	}
	output := firstNonEmpty(failure.output(), failure.err.Error())
	if looksLikeSetupPromptError(output) {
		return setupRequiredError{detail: oneLine(output)}
	}
	return fmt.Errorf("email command failed: %s", oneLine(output))
}

type setupRequiredError struct {
	detail string
}

func (e setupRequiredError) Error() string {
	return "email account setup is not finished yet. Open clibox to finish provider setup; if an email is already configured, clibox will continue at the password step"
}

func isSetupRequiredError(err error) bool {
	var setupErr setupRequiredError
	return errors.As(err, &setupErr)
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

func looksLikeSetupPromptError(output string) bool {
	output = strings.ToLower(output)
	for _, needle := range []string{
		"would you like to create",
		"cannot prompt",
		"account configure",
		"configuration file",
		"no configuration",
		"cannot get imap password",
		"cannot get smtp password",
		"cannot get secret from command",
		"specified item could not be found in the keychain",
		"cannot authenticate",
		"authenticationfailed",
		"invalid credentials",
	} {
		if strings.Contains(output, needle) {
			return true
		}
	}
	return false
}

func looksLikePageEndError(output string) bool {
	output = strings.ToLower(output)
	for _, needle := range []string{
		"out of bound",
		"out-of-bound",
		"page number too big",
		"page too big",
	} {
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
