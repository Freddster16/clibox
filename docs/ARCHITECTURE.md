# clibox architecture

`clibox` is a terminal email client with a small Bubble Tea app core and two
mail backends. The production goal is to keep the interactive TUI light while
the mail transport, storage, parsing, and setup details stay behind narrow
contracts.

## Layers

```text
main.go
  CLI flags, config loading, command dispatch, Bubble Tea startup

internal/app/model.go
  Bubble Tea model construction, refresh tick, top-level Update routing, and
  keyboard dispatch

internal/app/model_inbox_update.go + model_mail.go + model_labels.go
  Inbox/page result handling, reader body loading, message dedupe, mailbox
  labels, and user-facing state text

internal/app/action_flow.go + setup_flow.go + draft.go
  Message actions, account setup interaction, and draft editing/sending state

internal/app/render*.go + ui_text.go + theme.go
  Terminal rendering by screen, width/height fitting, ANSI sanitization,
  themes, overlays, and inline image presentation

internal/app/backend.go
  App-owned backend contracts and doctor dispatch

internal/app/himalaya.go + himalaya_config.go
  Himalaya compatibility adapter and Himalaya config generation

internal/app/native_backend.go + native_mime.go + native_address.go
  Native IMAP/SMTP adapter, MIME extraction, inline image limits, and address
  parsing

internal/app/native_store.go + oauth.go + secret_store.go
  Local cache, OAuth helpers, and OS credential lookup
```

## Current boundaries

- `model` owns UI state and only talks to mail through backend interfaces.
- `backend.go` owns the app-facing backend contracts. Adapters implement those
  contracts; they do not define the contract for the rest of the app.
- `message.go` holds the shared mail shape used by rendering, state updates,
  and backends.
- `model.go` routes events. Inbox/page result handling lives in
  `model_inbox_update.go`; mail commands and message body hydration live in
  `model_mail.go`.
- `render*.go` files render from already-normalized app state. They should not
  fetch mail or mutate backend state.
- `ui_text.go` centralizes terminal text safety and frame fitting, which is
  important because email content is untrusted terminal input.
- `native_mime.go` owns MIME/body/image extraction so parser behavior can be
  tested without dragging in the whole IMAP backend.
- `native_store.go` caches local metadata and bodies, while credentials stay in
  the OS credential store.

## Production direction

The largest responsibility splits are now in place. The next production-level
steps are about hardening contracts and making runtime behavior more observable:

- Split `native_backend.go` further by protocol job: IMAP envelopes, IMAP body
  fetch, SMTP sending, account setup, and OAuth auth glue.
- Split `model.go` further once behavior changes require it: theme/help
  routing, reader key handling, and top-level app shell can become separate
  event handlers.
- Keep backend interfaces small. Add a new interface only when the UI needs a
  new capability and both backends can gracefully report whether they support
  it.
- Add focused tests at each boundary: state transition tests for `model`,
  rendering frame tests for `render`, parser tests for mail/MIME handling, and
  backend contract tests with fakes.
- Add lightweight structured debug logging around backend calls, page loads,
  stale serial drops, and render dimensions. Keep it opt-in so the TUI remains
  quiet by default.

## Guardrails

- No remote image fetching inside rendering. Inline images should come from
  already-loaded message content and stay bounded in size.
- No raw email text should reach the terminal before passing through the text
  safety helpers.
- Refresh and pagination commands should carry serials so stale async work
  cannot overwrite newer UI state.
- Backends should return normalized messages and plain errors; rendering decides
  how to present status text.
