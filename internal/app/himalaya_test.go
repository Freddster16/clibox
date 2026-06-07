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
	if runner.calls[0] != "himalaya envelope list --output json --page-size 10 --folder INBOX" {
		t.Fatalf("unexpected v1 command: %q", runner.calls[0])
	}
	if !strings.HasPrefix(runner.calls[1], "himalaya envelopes list --json --page-size 10") {
		t.Fatalf("unexpected v2 command: %q", runner.calls[1])
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
	for _, want := range []string{"not configured", "himalaya account configure", "clibox doctor"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected setup guidance to contain %q, got %q", want, err)
		}
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
