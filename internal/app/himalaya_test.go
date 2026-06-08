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
	want := "himalaya message read --no-headers --account personal --folder INBOX 42"
	if runner.calls[0] != want {
		t.Fatalf("unexpected read command:\nwant %q\ngot  %q", want, runner.calls[0])
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
	want := "himalaya messages read --no-headers -a personal -m INBOX 99"
	if runner.calls[1] != want {
		t.Fatalf("unexpected fallback read command:\nwant %q\ngot  %q", want, runner.calls[1])
	}
}

type fakeCommandRunner struct {
	results []fakeCommandResult
	calls   []string
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
