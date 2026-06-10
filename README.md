# clibox

`clibox` is a fast terminal inbox for people who do not want to leave their
coding flow to read and reply to email.

The goal is a real keyboard-first email TUI, not a web wrapper. It should feel
at home next to Neovim, Codex, OpenCode, Hermes, tmux, and a shell: open the
inbox, move with `j/k`, read with `Enter`, reply in `$EDITOR`, archive with
`a`, search with `/`, change themes with `t`, and quit with `q`.

Status: Phase 6 is underway. The Himalaya-backed TUI remains the default
compatibility path, and a native backend now exists behind `backend = "native"`
or `--mail-backend native`. The native path adds browser OAuth plumbing,
native IMAP/SMTP operations, SQLite envelope/body cache, OS keychain token
storage, and new account/sync commands. Public one-click Gmail/Outlook login
still needs verified clibox OAuth client IDs before it can be the default for
everyone.

## Current implementation

- Starts a Bubble Tea inbox TUI with keyboard navigation and theme selection.
- Loads real envelope lists instead of shipping fake messages.
  The newest page appears first, then older pages continue loading in the
  background.
- Opens real plain-text message bodies on demand with `Enter`, then scrolls the
  reader with `j/k`, `PgUp/PgDn`, `Home`, and `End`.
- Starts setup with one email address, detects common providers, configures the
  mail connection in the background, and saves the password/app password to
  the OS credential store.
- Composes new email with `c`, replies from the reader with `r`, opens
  `$EDITOR`, then returns to a review screen where `s` sends and `e` edits
  again.
- Archives selected email with `a`, moves selected email to Trash after `d` + `y`,
  and searches the current mailbox with `/`.
- Supports `--account`, `--mailbox`, and advanced backend tuning flags.
- Reads optional local preferences from `~/.config/clibox/config.toml`.
- Refreshes the envelope list with `R`.
- Provides `clibox doctor` for setup checks before opening the TUI.
- Adds a native backend with OAuth PKCE loopback login for Gmail/Outlook when a
  provider OAuth client ID is configured.
- Stores native account metadata and cached envelopes in SQLite, but keeps
  passwords, access tokens, and refresh tokens out of config and cache.

## Install

Install or update the latest `main` build from GitHub:

```sh
curl -fsSL https://raw.githubusercontent.com/Freddster16/clibox/main/install.sh | sh
```

The installer checks for Homebrew, the Himalaya compatibility backend, and Go
before installing `clibox`. If Homebrew is already installed, the installer uses
it to install Himalaya and Go when Go 1.25.11 or newer is not available. For
security, `clibox` does not silently chain into Homebrew's remote installer. If
Homebrew is missing, install it yourself from [brew.sh](https://brew.sh/) or
explicitly opt in:

```sh
curl -fsSL https://raw.githubusercontent.com/Freddster16/clibox/main/install.sh | CLIBOX_INSTALL_HOMEBREW=1 sh
```

Homebrew may ask for your password while setting up system directories.

`clibox` itself is installed with `go install` directly from the `main` branch.
If your shell cannot find
`clibox` after installation, add Go's bin directory to your `PATH`; the
installer prints the exact path.

Native backend users can skip installing Himalaya:

```sh
curl -fsSL https://raw.githubusercontent.com/Freddster16/clibox/main/install.sh | CLIBOX_SKIP_HIMALAYA=1 sh
```

For local development:

```sh
go run .
```

Real email can use either the default Himalaya compatibility backend or the
opt-in native backend. If setup is not finished, `clibox` shows a friendly setup
prompt instead of crashing. In native mode, Gmail and Outlook use browser OAuth
when a provider client ID is configured; iCloud, Fastmail, Yahoo, and custom
IMAP providers continue to use app-password/manual setup through the OS
credential store.

Native secret storage uses the OS keychain through a cross-platform keyring
library: macOS Keychain, Linux Secret Service, and Windows Credential Manager.
The Himalaya compatibility path still supports macOS Keychain and Linux
`secret-tool`; its raw-password fallback exists only behind the explicit
`CLIBOX_ALLOW_RAW_PASSWORD=1` escape hatch.

If you already installed `clibox` and want the latest UI changes:

```sh
curl -fsSL https://raw.githubusercontent.com/Freddster16/clibox/main/install.sh | sh
```

## Target experience

```text
clibox - personal@example.com                                      20 emails
---------------------------------------------------------------------------
Mailboxes          Inbox                         Reader

> INBOX       20   > Alice        Re: Design notes          10:34 AM
  Archive          GitHub        New issue assigned        Yesterday
  Sent             Vercel        Deployment failed         Yesterday
  Drafts           Mom           Dinner                    Jun 6

                                                  From: Alice <alice@example.com>
                                                  Subject: Re: Design notes
                                                  Date: Sun Jun 7 10:34 AM

                                                  Hey Freddy,

                                                  I looked at the prototype...

j/k move  enter read  R refresh  A account  r reply  c compose  a archive  / search  t themes  ? help  q quit
```

On wide terminals, `clibox` should show a mailbox rail, inbox list, and reader
preview in one screen. On narrow terminals, it should collapse into focused
screens: mailbox/list, reader, compose/review.

The first run should be boring in the best way:

1. Run `clibox`.
2. If setup is needed, type your email address in the TUI and press `Enter`.
3. Paste the provider password or app password once.
4. Land in the inbox.
5. Press `Enter` to read, `b` to go back, `r` to reply, `c` to compose, `s` to
   send a reviewed draft, `a` to archive, `d` then `y` to move to Trash, `/` to
   search, `Esc` to clear an active search, `t` to open the theme picker, and
   `q` to leave.

## Themes

`clibox` ships with three vivid terminal themes: Nocturne, Ember, and Lagoon.
They color the header, rows, reader, help panel, and footer so each theme feels
distinct.

Theme selection lives inside the TUI:

- Press `t` to open the theme picker.
- Press `j/k` to preview themes live.
- Press `Enter` to apply the selected theme.
- Press `Esc` to cancel and return to the previous theme.

## Real-email quick start

`clibox` gives you a terminal-native inbox: start it, finish account setup if
needed, and read mail without leaving the shell.

Default compatibility flow:

```sh
# 1. Start the TUI.
clibox

# 2. If clibox asks for setup, type your email address and press Enter.
# clibox detects providers like Gmail, iCloud, Outlook, Yahoo, Fastmail,
# and Proton Mail.

# 3. Click the visible provider link in the TUI, or press o on the review
# screen / Ctrl+O on the password screen to open it in your browser.

# 4. Press Enter on the review screen, then paste the password/app password.
# clibox configures the mail connection and saves the secret to the OS credential store.

# 5. Optionally choose account or mailbox at launch after setup exists.
clibox --account personal
clibox --mailbox INBOX
clibox --config ~/.config/clibox/config.toml
clibox --archive-folder Archive

# 6. Reply or compose from the TUI. clibox opens CLIBOX_EDITOR, VISUAL,
# EDITOR, or nvim, then returns to a review screen before sending.

# 7. Check local setup without opening the TUI.
clibox doctor --account personal
```

Native OAuth flow:

```sh
# 1. Add safe account metadata. This writes no password or token to config.
clibox auth add --email you@gmail.com --account gmail

# 2. Configure a provider OAuth client ID.
# Gmail and Outlook require a registered desktop/native OAuth app.
export CLIBOX_GMAIL_CLIENT_ID="your-google-desktop-client-id"
export CLIBOX_OUTLOOK_CLIENT_ID="your-microsoft-public-client-id"

# 3. Start browser login. clibox listens on 127.0.0.1, validates state,
# exchanges the code with PKCE, and stores the refresh token in the OS keychain.
clibox auth login --account gmail

# 4. Sync envelopes into the local cache, or open the TUI directly.
clibox sync --account gmail --mailbox INBOX
clibox --mail-backend native --account gmail --mailbox INBOX

# 5. Inspect native setup without exposing secrets.
clibox doctor --mail-backend native --verbose --account gmail
clibox accounts
```

The native backend uses:

- IMAP via `github.com/emersion/go-imap`.
- SMTP via `github.com/emersion/go-smtp`.
- MIME parsing/building via `github.com/emersion/go-message/mail`.
- Provider-specific XOAUTH2/OAUTHBEARER-compatible SASL for OAuth mail login.
- SQLite cache at `~/.local/state/clibox/clibox.db` or `XDG_STATE_HOME`.
- OS keychain storage for refresh tokens and app passwords.

Manual backend setup is still available for advanced users. The default
compatibility implementation uses Himalaya internally:

```sh
himalaya account configure personal
himalaya envelope list --output json --page-size 5 --account personal --folder INBOX
```

Provider guidance currently covers:

- Gmail and Google Mail: Gmail IMAP/SMTP settings, Google app password, and the
  full email address as username.
- iCloud Mail: iCloud IMAP/SMTP settings and Apple app-specific password.
- Outlook, Hotmail, Live, and MSN: Outlook IMAP/SMTP settings, with a warning
  that some Microsoft accounts require Modern Auth/OAuth.
- Yahoo Mail: Yahoo IMAP/SMTP settings and Yahoo app password.
- Fastmail: Fastmail IMAP/SMTP settings and Fastmail app password.
- Proton Mail: Proton Mail Bridge warning before manual Bridge settings.
- Custom domains: manual IMAP/SMTP settings are still needed before automatic
  background setup can cover them.

Gmail and Outlook browser OAuth are implemented for the native backend, but a
publicly seamless flow still requires registered and verified clibox OAuth apps.
Until those client IDs exist, developers can test with their own desktop/native
OAuth client IDs through `CLIBOX_GMAIL_CLIENT_ID` or
`CLIBOX_OUTLOOK_CLIENT_ID`.

Useful launch flags:

```sh
clibox --account personal --mailbox INBOX
clibox --backend /path/to/backend
clibox --mail-backend native --account gmail
clibox --editor "nvim"
clibox --page-size 50
clibox --confirm-delete=false
clibox --archive-folder "[Gmail]/All Mail"
clibox doctor --account personal --mailbox INBOX
clibox doctor --mail-backend native --verbose --account gmail
clibox auth add --email you@gmail.com --account gmail
clibox auth login --account gmail
clibox accounts
clibox sync --account gmail --mailbox INBOX
```

Editor selection is environment-based today:

```sh
EDITOR=nvim clibox
VISUAL="code --wait" clibox
CLIBOX_EDITOR="vim -n" clibox
```

By default, `clibox` shows the newest page first and keeps loading older pages
in the background until the mailbox is complete. `--page-size` is only an
advanced tuning knob for how many envelopes the backend should return per request;
it is not an inbox limit.

`clibox` does not write email credentials into its own config. Native mode keeps
refresh tokens and app passwords in the OS keychain and stores only account
metadata plus cached mail data in SQLite. Draft files are temporary owner-only
files, and email content is sent to the backend over stdin rather than as
command-line arguments.
The adapter currently tries the stable Himalaya v1 command first
(`himalaya envelope list --output json`) and falls back to the in-development
v2 shape (`himalaya envelopes list --json`) only when the command shape is
incompatible. It paginates through the mailbox in the background instead of
capping the inbox at the first page or blocking startup on every old message.
Search uses the same pagination path and generates a backend query from plain
text across sender, recipient, subject, and body. Archive moves messages to the
provider archive folder, and delete uses Himalaya's safe delete behavior, which
moves mail to Trash or marks it deleted instead of expunging it immediately.
Runtime/setup errors, such as authentication or unknown-account failures, are
shown directly instead of being hidden behind another fallback.

## Development

Run the project locally with:

```sh
go run .
```

Check the email setup without opening the full-screen TUI:

```sh
go run . doctor
go run . doctor --mail-backend native --verbose
```

`clibox doctor` checks the backend version, config path, account, mailbox, and
the newest mailbox page. It does not download the full mailbox. Native doctor
also verifies that the SQLite cache schema does not contain credential-like
columns.

Run the verification suite:

```sh
go test ./...
go build ./...
go vet ./...
```

Production code no longer carries a fake inbox. Test fixtures live in the test
suite, while the app itself starts by loading envelopes from the configured
email backend.

## Configuration

Config path:

```text
~/.config/clibox/config.toml
```

Minimal config:

```toml
backend = "native" # or "himalaya"
account = "personal"
mailbox = "INBOX"
archive_folder = "Archive"
editor = "nvim"
confirm_delete = true

[accounts.gmail]
provider = "gmail"
email = "you@gmail.com"
mailbox = "INBOX"
archive_folder = "[Gmail]/All Mail"
sync_policy = "headers"
editor = "nvim"
```

The same path can be overridden with `CLIBOX_CONFIG` or `--config`.
Command-line flags override config values for that launch.

Allowed backend modes are `himalaya` and `native`. For compatibility with older
configs, `backend = "/path/to/himalaya"` is treated as a Himalaya binary path,
but new configs should use:

```toml
backend = "himalaya"
himalaya_binary = "/opt/homebrew/bin/himalaya"
```

Config files intentionally reject credential-like keys such as `password`,
`access_token`, `refresh_token`, `id_token`, and `client_secret`. Native tokens
and app passwords belong in the OS credential store, not TOML or SQLite.

Default keymap:

| Key | Action |
| --- | --- |
| `j` / `k` | Move in the inbox; scroll in the reader |
| `Enter` | Open selected email |
| `PgUp` / `PgDn` | Jump through the open email |
| `Home` / `End` | Jump to the top or bottom of the open email |
| `b` | Back to the previous view |
| `r` | Reply in `$EDITOR` |
| `c` | Compose in `$EDITOR` |
| `s` | Send the reviewed draft |
| `e` | Edit the reviewed draft again |
| `a` | Archive selected email |
| `d` | Delete selected email, with confirmation |
| `/` | Search current mailbox |
| `Esc` | Clear the active search while in the inbox |
| `R` | Refresh inbox |
| `A` | Add or update an email account inside the TUI |
| `t` | Open the theme picker |
| `?` | Show contextual help |
| `q` | Quit or close current view |

The UI should show the most relevant key hints in the footer and keep the full
keymap behind `?`, similar to terminal tools that make shortcuts discoverable
without crowding the main workflow.

## Architecture

Build `clibox` in Go:

- TUI: [Bubble Tea](https://github.com/charmbracelet/bubbletea) for the update
  loop, keyboard handling, and screen rendering.
- Components: [Bubbles table](https://charm-docs.vercel.app/docs/bubbles/components/table)
  for scrollable inbox rows, plus viewport/text input components where useful.
- Styling: Lip Gloss themes with clear focus states, readable status areas, and
  a quick `t` theme switcher.
- Backend boundary: one internal mail interface for list, read, send, archive,
  delete, search, and sync. TUI code does not build backend commands.
- Compatibility backend: a Himalaya adapter that calls the CLI and parses JSON
  where available.
- Native backend: IMAP, SMTP, MIME parsing, XOAUTH2/OAUTHBEARER SASL, SQLite
  cache, and OS keychain token/app-password storage.
- Account setup: provider detection plus either generated Himalaya config or
  native account metadata. Gmail and Outlook native setup uses browser OAuth
  when a provider client ID is configured; app-password providers use the OS
  credential store.
- OAuth: RFC 8252-style external browser login with loopback redirect, PKCE,
  and `state` validation. Device authorization is still a future fallback.
- Drafts: create temporary owner-only draft files, open `CLIBOX_EDITOR`,
  `VISUAL`, `EDITOR`, or `nvim`, show a review step after the editor exits, and
  send through the selected backend over stdin.
- Inbox actions: archive and delete through the backend adapter, plus
  plain-language search translated into the backend's query DSL.
- App config: TOML at `~/.config/clibox/config.toml`; native state lives in
  SQLite under XDG state; credentials never live in either place.

The Himalaya command surface differs between released and in-development
versions, so exact invocations stay inside the adapter. The same boundary keeps
provider-specific native IMAP/OAuth behavior away from the TUI.

## MVP roadmap

### Phase 1: Fake inbox

Done in the first implementation pass.

Build the TUI without touching real email:

- Show a fake inbox list.
- Move with `j/k`.
- Open a fake message with `Enter`.
- Return with `b`.
- Quit with `q`.
- Keep the footer hints and help overlay working from the start.

### Phase 2: Real envelope list

Done in the second implementation pass.

The inbox list is connected to the email backend:

- Runs the configured envelope-list command through the adapter.
- Shows the first page quickly, then loads older pages in the background.
- Parses JSON into internal envelope structs.
- Shows sender, subject, read/unread flags, and date.
- Displays clear setup errors when the backend is missing or incompatible.
- Refreshes the current envelope list with `R`.

### Phase 3: Real message reader

Done in the third implementation pass.

Read the selected email:

- Fetch body content through the adapter.
- Show headers and plain text content in a scrollable reader.
- Load the body only when the user presses `Enter`, so inbox startup stays fast.
- Cache opened bodies for the current session.
- Let the backend's normal read behavior mark opened mail as read.

### Phase 4: Reply and compose

Done in the fourth implementation pass.

Keep writing email inside the user's editor:

- Generate reply and compose drafts.
- Open `CLIBOX_EDITOR`, `VISUAL`, `EDITOR`, or `nvim`.
- Send only after the editor exits and the user confirms with `s`.
- Show a review/confirmation step before sending in v0.1.
- Keep draft content out of command-line arguments by sending it through stdin.

### Phase 5: Inbox actions

Done in the fifth implementation pass.

Round out the daily workflow:

- `a` archives the selected email.
- `d` deletes with confirmation.
- `/` searches the current mailbox.
- `R` refreshes the current mailbox.
- Errors appear inline without crashing the TUI.

### Phase 6: Native OAuth mail

Foundation implemented.

Move from visible backend ceremony toward a native mail client:

- `backend = "native"` and `--mail-backend native` select native mail.
- `clibox auth add`, `clibox auth login`, `clibox accounts`, and `clibox sync`
  expose native account and sync operations.
- Gmail and Outlook use external browser OAuth with loopback redirect, PKCE,
  state validation, and refresh-token storage in the OS keychain.
- Native IMAP lists and reads messages, archives/deletes with IMAP MOVE when
  available, and sends through SMTP.
- SQLite caches accounts, mailboxes, envelopes, message bodies, and sync state.
- Config and SQLite reject credential storage by design.

Remaining production gate: register and verify public clibox OAuth clients for
Gmail and Microsoft so normal users do not need to bring their own client IDs.

## UX principles

- Make the fast path obvious: read, reply, archive, search, quit.
- Prefer Vim-style motion, but keep help discoverable.
- Keep destructive actions confirmable.
- Never block the interface on slow email operations; show loading and errors in
  context.
- Optimize for plain text and developer mail first: GitHub, deploy failures,
  patch review, invoices, personal replies.
- Keep advanced email complexity behind the backend adapter until the TUI has a
  strong daily workflow.

## Research notes

`clibox` borrows proven ideas from existing terminal tools:

- [aerc](https://aerc-mail.org/) shows that a serious terminal email client
  benefits from Vim-style keybindings, editor/pager integration, multiple
  accounts, and async behavior.
- [Matcha](https://docs.matcha.email/) sets a modern expectation for a
  responsive terminal email UI with clear focus management, multi-account
  workflows, archive/delete/reply basics, and markdown-friendly composing.
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) is a good Go
  foundation for a stateful full-screen TUI.
- [Himalaya](https://github.com/pimalaya/himalaya) is a strong first backend
  because it turns email protocols into scriptable CLI operations.
- [RFC 8252](https://www.rfc-editor.org/info/rfc8252) guides native app OAuth:
  use the system browser, loopback redirects, PKCE, and state validation instead
  of embedded login.
- [RFC 8628](https://datatracker.ietf.org/doc/html/rfc8628) is the future
  fallback for device-style authorization when a browser callback is awkward.
- [Gmail XOAUTH2](https://developers.google.com/workspace/gmail/imap/xoauth2-protocol)
  and [Microsoft IMAP/SMTP OAuth](https://learn.microsoft.com/en-us/exchange/client-developer/legacy-protocols/how-to-authenticate-an-imap-pop-smtp-application-by-using-oauth)
  document OAuth access-token authentication for mail protocols.
- Email workflow research frames the product goal as triage, task handling, and
  retrieval, not just "show messages": [Whittaker/Sidner revisit](https://www.microsoft.com/en-us/research/publication/revisiting-whittaker-sidners-email-overload-ten-years-later/),
  [Supporting Email Workflow](https://www.interruptions.net/literature/Venolia-01-88.pdf),
  [Taking Email to Task](https://dl.acm.org/doi/10.1145/642611.642672),
  and [email automation need-finding](https://dl.acm.org/doi/10.1145/3290605.3300604).

## Non-goals for the next pass

- Making native OAuth the default before verified public provider client IDs
  exist.
- Full HTML email rendering beyond a plain-text fallback.
- Calendar support.
- AI features.
- Attachment-heavy workflows.
- PGP, S/MIME, or advanced security workflows.
- Release signing, Homebrew tap automation, and notarized binary distribution.
- A plugin system.

Those can come later if the core inbox loop feels excellent.
