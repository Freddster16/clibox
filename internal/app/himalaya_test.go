package app

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestParseHimalayaEnvelopeListV1(t *testing.T) {
	raw := []byte(`[
		{
			"id": 42,
			"flags": ["Seen"],
			"subject": "Deployment failed",
			"from": {"name": "Vercel", "address": "noreply@vercel.com"},
			"date": "2026-06-07 10:34+00:00"
		},
		{
			"id": "43",
			"flags": ["Answered"],
			"subject": "Dinner",
			"from": "Mom <mom@example.com>",
			"date": "Yesterday"
		}
	]`)

	messages, err := parseHimalayaMessages(raw)
	if err != nil {
		t.Fatalf("expected v1 JSON to parse: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].ID != "42" || messages[0].From != "Vercel" || messages[0].Email != "noreply@vercel.com" {
		t.Fatalf("unexpected first message: %+v", messages[0])
	}
	if messages[0].Unread {
		t.Fatal("expected Seen flag to produce a read message")
	}
	if !messages[1].Unread {
		t.Fatal("expected missing Seen flag to produce an unread message")
	}
}

func TestParseHimalayaEnvelopeListV2Wrapper(t *testing.T) {
	raw := []byte(`{
		"envelopes": [
			{
				"id": "abc-123",
				"flags": ["\\Seen"],
				"subject": "Re: Design notes",
				"sender": [{"displayName": "Alice", "email": "alice@example.com"}],
				"sentAt": "Sun, 07 Jun 2026 10:34:00 -0400",
				"snippet": "I looked at the prototype..."
			}
		]
	}`)

	messages, err := parseHimalayaMessages(raw)
	if err != nil {
		t.Fatalf("expected v2 JSON to parse: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	msg := messages[0]
	if msg.ID != "abc-123" || msg.From != "Alice" || msg.Email != "alice@example.com" {
		t.Fatalf("unexpected parsed message: %+v", msg)
	}
	if msg.Subject != "Re: Design notes" || msg.Preview != "I looked at the prototype..." {
		t.Fatalf("expected subject and preview to parse, got %+v", msg)
	}
	if msg.Unread {
		t.Fatal("expected \\Seen flag to produce a read message")
	}
}

func TestParseHimalayaEnvelopeListResponseWrapper(t *testing.T) {
	raw := []byte(`{
		"response": [
			{"id": 7, "flags": ["Recent"], "subject": "Wrapped", "sender": "Himalaya <mail@example.org>"}
		]
	}`)

	messages, err := parseHimalayaMessages(raw)
	if err != nil {
		t.Fatalf("expected response wrapper to parse: %v", err)
	}
	if len(messages) != 1 || messages[0].ID != "7" || messages[0].From != "Himalaya" {
		t.Fatalf("unexpected wrapped message: %+v", messages)
	}
}

func TestParseAddressStringHandlesCommonShapes(t *testing.T) {
	cases := []struct {
		raw   string
		name  string
		email string
	}{
		{raw: "Alice <alice@example.com>", name: "Alice", email: "alice@example.com"},
		{raw: `"Alice Example" <alice@example.com>`, name: "Alice Example", email: "alice@example.com"},
		{raw: "alice@example.com", name: "", email: "alice@example.com"},
	}

	for _, tc := range cases {
		name, email := parseAddressString(tc.raw)
		if name != tc.name || email != tc.email {
			t.Fatalf("parseAddressString(%q) = (%q, %q), want (%q, %q)", tc.raw, name, email, tc.name, tc.email)
		}
	}
}

func TestHimalayaBackendFallsBackToV2Shape(t *testing.T) {
	runner := &fakeCommandRunner{results: []fakeCommandResult{
		{
			stderr: []byte("error: unrecognized subcommand 'envelope'\n"),
			err:    errors.New("exit status 2"),
		},
		{
			stdout: []byte(`[{"id":"99","subject":"Hello","from":"Alice <alice@example.com>","flags":["Answered"]}]`),
		},
	}}
	backend := himalayaBackend{
		binary:   "himalaya",
		mailbox:  "INBOX",
		pageSize: 10,
		runner:   runner,
	}

	messages, err := backend.ListEnvelopes(context.Background())
	if err != nil {
		t.Fatalf("expected fallback command to succeed: %v", err)
	}
	if len(messages) != 1 || messages[0].ID != "99" {
		t.Fatalf("unexpected messages: %+v", messages)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("expected 2 command attempts, got %d", len(runner.calls))
	}
	if runner.calls[0] != "himalaya envelope list --output json --page 1 --page-size 10 --folder INBOX" {
		t.Fatalf("unexpected v1 command: %q", runner.calls[0])
	}
	if !strings.HasPrefix(runner.calls[1], "himalaya envelopes list --json --page 1 --page-size 10") {
		t.Fatalf("unexpected v2 command: %q", runner.calls[1])
	}
}

func TestHimalayaBackendLoadsAllPages(t *testing.T) {
	runner := &fakeCommandRunner{results: []fakeCommandResult{
		{
			stdout: []byte(`[
				{"id":"1","subject":"One","from":"Alice <alice@example.com>"},
				{"id":"2","subject":"Two","from":"Bob <bob@example.com>"}
			]`),
		},
		{
			stdout: []byte(`[
				{"id":"3","subject":"Three","from":"Cora <cora@example.com>"}
			]`),
		},
	}}
	backend := himalayaBackend{
		binary:   "himalaya",
		mailbox:  "INBOX",
		pageSize: 2,
		runner:   runner,
	}

	messages, err := backend.ListEnvelopes(context.Background())
	if err != nil {
		t.Fatalf("expected paginated list to succeed: %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("expected all 3 messages, got %+v", messages)
	}
	if got := strings.Join(runner.calls, "\n"); !strings.Contains(got, "--page 1") || !strings.Contains(got, "--page 2") {
		t.Fatalf("expected page 1 and page 2 calls, got:\n%s", got)
	}
}

func TestHimalayaFallbackIDsDoNotCollideAcrossPages(t *testing.T) {
	runner := &fakeCommandRunner{results: []fakeCommandResult{
		{
			stdout: []byte(`[
				{"subject":"One","from":"Alice <alice@example.com>"},
				{"subject":"Two","from":"Bob <bob@example.com>"}
			]`),
		},
		{
			stdout: []byte(`[
				{"subject":"Three","from":"Cora <cora@example.com>"}
			]`),
		},
	}}
	backend := himalayaBackend{
		binary:   "himalaya",
		mailbox:  "INBOX",
		pageSize: 2,
		runner:   runner,
	}

	first, done, err := backend.ListEnvelopePage(context.Background(), 1)
	if err != nil {
		t.Fatalf("expected first page to load: %v", err)
	}
	if done {
		t.Fatal("expected full first page to leave pagination open")
	}
	second, done, err := backend.ListEnvelopePage(context.Background(), 2)
	if err != nil {
		t.Fatalf("expected second page to load: %v", err)
	}
	if !done {
		t.Fatal("expected partial second page to finish pagination")
	}

	merged := mergeMessagePage(first, second)
	if len(merged) != 3 {
		t.Fatalf("expected fallback ids to keep all pages, got %+v", merged)
	}
	if merged[0].ID != "1" || merged[1].ID != "2" || merged[2].ID != "3" {
		t.Fatalf("unexpected fallback ids: %+v", merged)
	}
}

func TestHimalayaBackendOmitsPageSizeWhenUnset(t *testing.T) {
	runner := &fakeCommandRunner{results: []fakeCommandResult{
		{
			stdout: []byte(`[]`),
		},
	}}
	backend := himalayaBackend{
		binary:  "himalaya",
		mailbox: "INBOX",
		runner:  runner,
	}

	messages, err := backend.ListEnvelopes(context.Background())
	if err != nil {
		t.Fatalf("expected empty mailbox to load: %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("expected empty mailbox, got %+v", messages)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected one page call, got %v", runner.calls)
	}
	if strings.Contains(runner.calls[0], "--page-size") {
		t.Fatalf("expected no page-size flag when unset, got %q", runner.calls[0])
	}
	if runner.calls[0] != "himalaya envelope list --output json --page 1 --folder INBOX" {
		t.Fatalf("unexpected command: %q", runner.calls[0])
	}
}

func TestNewHimalayaBackendDefaultsPageSize(t *testing.T) {
	backend := newHimalayaBackend(Options{})
	if backend.pageSize != 50 {
		t.Fatalf("expected default page size 50, got %d", backend.pageSize)
	}
}

func TestHimalayaDoctorChecksOnePage(t *testing.T) {
	runner := &fakeCommandRunner{results: []fakeCommandResult{
		{
			stdout: []byte("himalaya 1.2.0\n"),
		},
		{
			stdout: []byte(`[
				{"id":"1","subject":"One","from":"Alice <alice@example.com>"},
				{"id":"2","subject":"Two","from":"Bob <bob@example.com>"}
			]`),
		},
	}}
	backend := himalayaBackend{
		binary:   "himalaya",
		account:  "personal",
		mailbox:  "INBOX",
		pageSize: 2,
		runner:   runner,
	}

	report, err := backend.Diagnose(context.Background())
	if err != nil {
		t.Fatalf("expected doctor to succeed: %v", err)
	}
	for _, want := range []string{"Email connection OK", "Backend: himalaya 1.2.0", "Account: personal", "Mailbox: INBOX", "First page: 2 emails"} {
		if !strings.Contains(report, want) {
			t.Fatalf("expected doctor report to contain %q, got:\n%s", want, report)
		}
	}
	if len(runner.calls) != 2 {
		t.Fatalf("expected version plus one page call, got %v", runner.calls)
	}
	if strings.Contains(strings.Join(runner.calls, "\n"), "--page 2") {
		t.Fatalf("doctor should not load older pages, got calls: %v", runner.calls)
	}
}

func TestHimalayaBackendSearchesEnvelopePages(t *testing.T) {
	runner := &fakeCommandRunner{results: []fakeCommandResult{
		{
			stdout: []byte(`[{"id":"7","subject":"Deploy notes","from":"Alice <alice@example.com>"}]`),
		},
	}}
	backend := himalayaBackend{
		binary:   "himalaya",
		account:  "personal",
		mailbox:  "INBOX",
		pageSize: 1,
		runner:   runner,
	}

	messages, done, err := backend.SearchEnvelopePage(context.Background(), 1, "Alice deploy")
	if err != nil {
		t.Fatalf("expected search page to load: %v", err)
	}
	if done {
		t.Fatal("expected full page to keep pagination open")
	}
	if len(messages) != 1 || messages[0].ID != "7" {
		t.Fatalf("unexpected search messages: %+v", messages)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected one search command, got %v", runner.calls)
	}
	for _, want := range []string{"himalaya envelope list --output json --page 1 --page-size 1 --account personal --folder INBOX", "subject alice", "body deploy", "order by date desc"} {
		if !strings.Contains(runner.calls[0], want) {
			t.Fatalf("expected search command to contain %q, got %q", want, runner.calls[0])
		}
	}
}

func TestHimalayaBackendTreatsOutOfBoundsPageAsEnd(t *testing.T) {
	runner := &fakeCommandRunner{results: []fakeCommandResult{
		{
			stdout: []byte(`[
				{"id":"1","subject":"One","from":"Alice <alice@example.com>"},
				{"id":"2","subject":"Two","from":"Bob <bob@example.com>"}
			]`),
		},
		{
			stderr: []byte("page number out of bound"),
			err:    errors.New("exit status 1"),
		},
	}}
	backend := himalayaBackend{
		binary:   "himalaya",
		mailbox:  "INBOX",
		pageSize: 2,
		runner:   runner,
	}

	messages, err := backend.ListEnvelopes(context.Background())
	if err != nil {
		t.Fatalf("expected out-of-bounds page to end list: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected first page messages to be kept, got %+v", messages)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("expected one successful page and one end page, got %v", runner.calls)
	}
}

func TestHimalayaBackendArchivesMessage(t *testing.T) {
	runner := &fakeCommandRunner{results: []fakeCommandResult{{stdout: []byte("moved")}}}
	backend := himalayaBackend{
		binary:        "himalaya",
		account:       "personal",
		mailbox:       "INBOX",
		archiveFolder: "Archive",
		runner:        runner,
	}

	if err := backend.ArchiveMessage(context.Background(), message{ID: "42"}); err != nil {
		t.Fatalf("expected archive to succeed: %v", err)
	}
	want := "himalaya message move Archive --account personal --folder INBOX 42"
	if len(runner.calls) != 1 || runner.calls[0] != want {
		t.Fatalf("unexpected archive command:\nwant %q\ngot  %v", want, runner.calls)
	}
}

func TestHimalayaBackendDeletesMessage(t *testing.T) {
	runner := &fakeCommandRunner{results: []fakeCommandResult{{stdout: []byte("deleted")}}}
	backend := himalayaBackend{
		binary:  "himalaya",
		account: "personal",
		mailbox: "INBOX",
		runner:  runner,
	}

	if err := backend.DeleteMessage(context.Background(), message{ID: "42"}); err != nil {
		t.Fatalf("expected delete to succeed: %v", err)
	}
	want := "himalaya message delete --account personal --folder INBOX 42"
	if len(runner.calls) != 1 || runner.calls[0] != want {
		t.Fatalf("unexpected delete command:\nwant %q\ngot  %v", want, runner.calls)
	}
}

func TestHimalayaBackendArchiveFallsBackToV2Shape(t *testing.T) {
	runner := &fakeCommandRunner{results: []fakeCommandResult{
		{
			stderr: []byte("error: unrecognized subcommand 'message'"),
			err:    errors.New("exit status 2"),
		},
		{
			stdout: []byte("moved"),
		},
	}}
	backend := himalayaBackend{
		binary:        "himalaya",
		account:       "personal",
		mailbox:       "INBOX",
		archiveFolder: "Archive",
		runner:        runner,
	}

	if err := backend.ArchiveMessage(context.Background(), message{ID: "42"}); err != nil {
		t.Fatalf("expected fallback archive to succeed: %v", err)
	}
	want := "himalaya messages move 42 --account personal --from INBOX --to Archive"
	if len(runner.calls) != 2 || runner.calls[1] != want {
		t.Fatalf("unexpected fallback archive command:\nwant %q\ngot  %v", want, runner.calls)
	}
}

func TestHimalayaBackendDoesNotFallbackOnRuntimeFailure(t *testing.T) {
	runner := &fakeCommandRunner{results: []fakeCommandResult{
		{
			stderr: []byte("unrecognized account personal"),
			err:    errors.New("exit status 1"),
		},
		{
			stdout: []byte(`[{"id":"should-not-run"}]`),
		},
	}}
	backend := himalayaBackend{
		binary:   "himalaya",
		mailbox:  "INBOX",
		pageSize: 10,
		runner:   runner,
	}

	_, err := backend.ListEnvelopes(context.Background())
	if err == nil {
		t.Fatal("expected runtime failure to return an error")
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected runtime failure not to fallback, got calls: %v", runner.calls)
	}
	if !strings.Contains(err.Error(), "unrecognized account personal") {
		t.Fatalf("expected error to explain runtime failure, got %q", err)
	}
}

func TestHimalayaBackendExplainsSetupPromptFailure(t *testing.T) {
	runner := &fakeCommandRunner{results: []fakeCommandResult{
		{
			stderr: []byte("? Would you like to create one with the wizard? Error: 0: cannot prompt boolean 1: IO error"),
			err:    errors.New("exit status 1"),
		},
	}}
	backend := himalayaBackend{
		binary:   "himalaya",
		mailbox:  "INBOX",
		pageSize: 10,
		runner:   runner,
	}

	_, err := backend.ListEnvelopes(context.Background())
	if err == nil {
		t.Fatal("expected setup prompt failure")
	}
	for _, want := range []string{"setup is not finished", "finish provider setup", "password step"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected setup guidance to contain %q, got %q", want, err)
		}
	}
}

func TestHimalayaBackendReadsMessageBody(t *testing.T) {
	runner := &fakeCommandRunner{results: []fakeCommandResult{
		{
			stdout: []byte("Hey Freddy,\r\n\r\nThe build passed.\r\n"),
		},
	}}
	backend := himalayaBackend{
		binary:  "himalaya",
		account: "personal",
		mailbox: "INBOX",
		runner:  runner,
	}

	body, err := backend.ReadMessage(context.Background(), message{ID: "42"})
	if err != nil {
		t.Fatalf("expected message body to load: %v", err)
	}
	if body != "Hey Freddy,\n\nThe build passed." {
		t.Fatalf("unexpected normalized body: %q", body)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected one read command, got %v", runner.calls)
	}
	want := "himalaya message read --preview --no-headers --account personal --folder INBOX 42"
	if runner.calls[0] != want {
		t.Fatalf("unexpected read command:\nwant %q\ngot  %q", want, runner.calls[0])
	}
}

func TestNormalizeMessageBodyStripsReadHeadersAndPartMarkers(t *testing.T) {
	raw := []byte(strings.Join([]string{
		"From: ATTACK SHARK <support@attackshark.com>",
		"From: ATTACK SHARK <support@attackshark.com>",
		"From: LinkedIn Job Alerts <jobalerts-noreply@linkedin.com>",
		"Subject: junior software engineer: Orchia - Junior Software Development Engineer and more",
		"Date: 2026-06-10 19:47+00:00",
		"<#part type=text/html>",
		"Your job alert for junior software engineer in United States",
		"",
		"<#part type=text/plain>",
		"View jobs",
	}, "\n"))

	body := normalizeMessageBody(raw)
	if strings.Contains(body, "From: ATTACK SHARK") || strings.Contains(body, "Subject:") || strings.Contains(body, "<#part") {
		t.Fatalf("expected read decorations to be stripped, got %q", body)
	}
	want := "Your job alert for junior software engineer in United States\n\nView jobs"
	if body != want {
		t.Fatalf("unexpected normalized body:\nwant %q\ngot  %q", want, body)
	}
}

func TestNormalizeMessageBodyPreservesBodyStartingWithHeaderLikeLine(t *testing.T) {
	raw := []byte("From: the recruiting desk\n\nThanks for applying.")

	body := normalizeMessageBody(raw)
	if body != "From: the recruiting desk\n\nThanks for applying." {
		t.Fatalf("expected header-like body text to be preserved, got %q", body)
	}
}

func TestHimalayaBackendReadFallsBackToV2Shape(t *testing.T) {
	runner := &fakeCommandRunner{results: []fakeCommandResult{
		{
			stderr: []byte("error: unrecognized subcommand 'message'"),
			err:    errors.New("exit status 2"),
		},
		{
			stdout: []byte("Plain body"),
		},
	}}
	backend := himalayaBackend{
		binary:  "himalaya",
		account: "personal",
		mailbox: "INBOX",
		runner:  runner,
	}

	body, err := backend.ReadMessage(context.Background(), message{ID: "99"})
	if err != nil {
		t.Fatalf("expected fallback read command to succeed: %v", err)
	}
	if body != "Plain body" {
		t.Fatalf("unexpected body: %q", body)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("expected two command attempts, got %v", runner.calls)
	}
	want := "himalaya messages read --preview --no-headers -a personal -m INBOX 99"
	if runner.calls[1] != want {
		t.Fatalf("unexpected fallback read command:\nwant %q\ngot  %q", want, runner.calls[1])
	}
}

func TestHimalayaBackendPreparesComposeDraft(t *testing.T) {
	runner := &fakeCommandRunner{results: []fakeCommandResult{
		{
			stdout: []byte("From: Freddy <freddy@example.com>\nTo: \nSubject: \n\n"),
		},
	}}
	backend := himalayaBackend{
		binary:  "himalaya",
		account: "personal",
		runner:  runner,
	}

	draft, err := backend.PrepareDraft(context.Background(), draftRequest{Kind: composeDraft})
	if err != nil {
		t.Fatalf("expected compose draft to prepare: %v", err)
	}
	if !strings.Contains(draft, "To:") || !strings.Contains(draft, "Subject:") {
		t.Fatalf("expected friendly compose headers, got %q", draft)
	}
	want := "himalaya template write -H To: -H Subject: --account personal"
	if len(runner.calls) != 1 || runner.calls[0] != want {
		t.Fatalf("unexpected compose command:\nwant %q\ngot  %v", want, runner.calls)
	}
}

func TestHimalayaBackendPreparesReplyDraft(t *testing.T) {
	runner := &fakeCommandRunner{results: []fakeCommandResult{
		{
			stdout: []byte("From: Freddy <freddy@example.com>\nTo: Alice <alice@example.com>\nSubject: Re: Design notes\n\n> hello\n"),
		},
	}}
	backend := himalayaBackend{
		binary:  "himalaya",
		account: "personal",
		mailbox: "INBOX",
		runner:  runner,
	}

	draft, err := backend.PrepareDraft(context.Background(), draftRequest{
		Kind:    replyDraft,
		Message: message{ID: "42", Email: "alice@example.com", Subject: "Design notes"},
	})
	if err != nil {
		t.Fatalf("expected reply draft to prepare: %v", err)
	}
	if !strings.Contains(draft, "Subject: Re: Design notes") || !strings.Contains(draft, "> hello") {
		t.Fatalf("unexpected reply draft: %q", draft)
	}
	want := "himalaya template reply --account personal --folder INBOX 42"
	if len(runner.calls) != 1 || runner.calls[0] != want {
		t.Fatalf("unexpected reply command:\nwant %q\ngot  %v", want, runner.calls)
	}
}

func TestHimalayaBackendSendsDraftThroughStdin(t *testing.T) {
	runner := &fakeCommandRunner{results: []fakeCommandResult{{stdout: []byte("Message successfully sent!")}}}
	backend := himalayaBackend{
		binary:  "himalaya",
		account: "personal",
		runner:  runner,
	}
	content := "To: alice@example.com\nSubject: private\n\nThis body must not be an argv value.\n"

	if err := backend.SendDraft(context.Background(), content); err != nil {
		t.Fatalf("expected draft send to succeed: %v", err)
	}
	want := "himalaya template send --account personal"
	if len(runner.calls) != 1 || runner.calls[0] != want {
		t.Fatalf("unexpected send command:\nwant %q\ngot  %v", want, runner.calls)
	}
	if len(runner.inputs) != 1 || runner.inputs[0] != content {
		t.Fatalf("expected draft content on stdin, got %#v", runner.inputs)
	}
	if strings.Contains(runner.calls[0], "This body") || strings.Contains(runner.calls[0], "private") {
		t.Fatalf("draft content leaked into argv: %q", runner.calls[0])
	}
}

func TestHimalayaBackendSendDraftFallsBackToMessagesSend(t *testing.T) {
	runner := &fakeCommandRunner{results: []fakeCommandResult{
		{
			stderr: []byte("error: unrecognized subcommand 'template'"),
			err:    errors.New("exit status 2"),
		},
		{
			stdout: []byte("Message successfully sent!"),
		},
	}}
	backend := himalayaBackend{
		binary:  "himalaya",
		account: "personal",
		runner:  runner,
	}

	if err := backend.SendDraft(context.Background(), "To: alice@example.com\n\nHi\n"); err != nil {
		t.Fatalf("expected fallback send to succeed: %v", err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("expected two send attempts, got %v", runner.calls)
	}
	if runner.calls[1] != "himalaya messages send -a personal" {
		t.Fatalf("unexpected fallback send command: %q", runner.calls[1])
	}
}

type fakeCommandRunner struct {
	results []fakeCommandResult
	calls   []string
	inputs  []string
}

type fakeCommandResult struct {
	stdout []byte
	stderr []byte
	err    error
}

func (r *fakeCommandRunner) Run(_ context.Context, program string, args []string) ([]byte, []byte, error) {
	r.calls = append(r.calls, shellCommand(program, args))
	if len(r.results) == 0 {
		return nil, nil, errors.New("unexpected command")
	}
	result := r.results[0]
	r.results = r.results[1:]
	return result.stdout, result.stderr, result.err
}

func (r *fakeCommandRunner) RunInput(_ context.Context, program string, args []string, input string) ([]byte, []byte, error) {
	r.calls = append(r.calls, shellCommand(program, args))
	r.inputs = append(r.inputs, input)
	if len(r.results) == 0 {
		return nil, nil, errors.New("unexpected command")
	}
	result := r.results[0]
	r.results = r.results[1:]
	return result.stdout, result.stderr, result.err
}
