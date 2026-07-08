package app

import (
	"context"
	"fmt"
)

type inboxBackend interface {
	ListEnvelopes(context.Context) ([]message, error)
	Label() string
}

type pagedInboxBackend interface {
	ListEnvelopePage(context.Context, int) ([]message, bool, error)
}

type searchablePagedInboxBackend interface {
	SearchEnvelopePage(context.Context, int, string) ([]message, bool, error)
}

type mailboxSwitchBackend interface {
	WithMailbox(mailbox string) inboxBackend
}

type messageBodyBackend interface {
	ReadMessage(context.Context, message) (string, error)
}

type messageContentBackend interface {
	ReadMessageContent(context.Context, message) (messageContent, error)
}

type messageActionBackend interface {
	ArchiveMessage(context.Context, message) error
	DeleteMessage(context.Context, message) error
}

type messageFlagBackend interface {
	MarkMessageRead(context.Context, message) error
	MarkMessageUnread(context.Context, message) error
	SetMessageFlagged(context.Context, message, bool) error
}

type draftBackend interface {
	PrepareDraft(context.Context, draftRequest) (string, error)
	SendDraft(context.Context, string) error
}

type accountSetupBackend interface {
	SaveAccountSetup(accountSetup) error
	WithAccount(account string) inboxBackend
}

type oauthAccountSetupBackend interface {
	SaveOAuthAccountSetup(context.Context, accountSetup) error
	WithAccount(account string) inboxBackend
}

func Doctor(ctx context.Context, options Options) (string, error) {
	if options.backend != nil {
		messages, err := options.backend.ListEnvelopes(ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Email connection OK: loaded %d emails from %s", len(messages), options.backend.Label()), nil
	}

	if normalizeBackendMode(options.BackendMode) == backendModeNative {
		backend := newNativeBackend(options)
		return backend.Diagnose(ctx, options.Verbose)
	}
	backend := newHimalayaBackend(options)
	return backend.Diagnose(ctx)
}
