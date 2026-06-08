# clibox

`clibox` is a fast terminal inbox for people who do not want to leave their
coding flow to read and reply to email.

The goal is a real keyboard-first email TUI, not a web wrapper. It should feel
at home next to Neovim, Codex, OpenCode, Hermes, tmux, and a shell: open the
inbox, move with `j/k`, read with `Enter`, reply in `$EDITOR`, archive with
`a`, search with `/`, change themes with `t`, and quit with `q`.

Status: Phase 2 is implemented. `clibox` now loads the envelope list from
Himalaya after Himalaya is installed and configured. The reader still shows
envelope-level content only until Phase 3 wires full message bodies. First-run
setup now happens inside `clibox` for common providers instead of sending you
through Himalaya's duplicate account wizard.

## Current implementation

- Starts a Bubble Tea inbox TUI with keyboard navigation and theme selection.
- Loads real envelope lists through Himalaya instead of shipping fake messages.
  The newest page appears first, then older pages continue loading in the
  background.
- Starts setup with one email address, detects common providers, writes the
  Himalaya account config in the background, and saves the password/app password
  to macOS Keychain.
- Supports `--account`, `--mailbox`, `--himalaya`, and `--page-size`.
- Refreshes the envelope list with `R`.
- Provides `clibox doctor` for setup checks before opening the TUI.
- Keeps full message body reading, compose/reply, archive/delete, and search in
  later phases.

## Install

Install or update the latest `main` build from GitHub:

```sh
curl -fsSL https://raw.githubusercontent.com/Freddster16/clibox/main/install.sh | sh
```

The installer checks for Homebrew, Himalaya, and Go before installing `clibox`.
If Homebrew is missing on macOS or Linux, it installs Homebrew using the official
Homebrew installer. It then installs Himalaya with `brew install himalaya`, and
installs Go with Homebrew if Go 1.24 or newer is not available. Homebrew may ask
for your password while setting up system directories.

`clibox` itself is installed with `go install` directly from the `main` branch.
If your shell cannot find
`clibox` after installation, add Go's bin directory to your `PATH`; the
installer prints the exact path.

For local development:

```sh
go run .
```

Phase 2 requires Himalaya for real inbox data. If Himalaya is missing or not yet
configured, `clibox` shows a setup error in the footer instead of crashing.
If Himalaya needs an account, `clibox` asks for your email address once, detects
the provider, chooses known IMAP/SMTP settings, asks for the required password or
app password, writes `~/.config/himalaya/config.toml`, stores the secret in
macOS Keychain, and reloads your inbox.

Automatic secret storage is macOS-first right now. On other platforms, manual
Himalaya setup is still available until `clibox` grows a portable secret-store
adapter.

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
5. Press `Enter` to read, `b` to go back, `r` to reply, `a` to archive, `/` to
   search, `t` to open the theme picker, and `q` to leave.

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

`clibox` relies on [Himalaya](https://github.com/pimalaya/himalaya) for email
protocols at first. Himalaya already handles the hard parts: accounts,
IMAP/JMAP/SMTP, message envelopes, folders, authentication, and sending.

Flow:

```sh
# 1. Start the TUI.
clibox

# 2. If clibox asks for setup, type your email address and press Enter.
# clibox detects providers like Gmail, iCloud, Outlook, Yahoo, Fastmail,
# and Proton Mail.

# 3. Click the visible provider link in the TUI, or press o on the review
# screen / Ctrl+O on the password screen to open it in your browser.

# 4. Press Enter on the review screen, then paste the password/app password.
# clibox writes Himalaya's IMAP/SMTP config and saves the secret to Keychain.

# 5. Optionally choose account or mailbox at launch after setup exists.
clibox --account personal
clibox --mailbox INBOX

# 6. Check local setup without opening the TUI.
clibox doctor --account personal
```

Manual Himalaya setup is still available:

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

Full Gmail browser OAuth is a planned upgrade. Google supports OAuth for
Gmail IMAP/SMTP through XOAUTH2, but that flow requires a registered Google
OAuth desktop client and the restricted `https://mail.google.com/` scope. Until
`clibox` has its own verified Google OAuth client and token storage, the Gmail
path opens Google's setup page and uses Himalaya's app-password-compatible IMAP
setup.

Useful launch flags:

```sh
clibox --account personal --mailbox INBOX
clibox --himalaya /path/to/himalaya
clibox --page-size 50
clibox doctor --account personal --mailbox INBOX
```

By default, `clibox` shows the newest page first and keeps loading older pages
in the background until the mailbox is complete. `--page-size` is only an
advanced tuning knob for how many envelopes Himalaya should return per request;
it is not an inbox limit.

`clibox` does not write email credentials into its own config. On macOS it saves
the password/app password to Keychain and writes a Himalaya `auth.cmd` entry that
reads the secret from Keychain when Himalaya connects. It reads envelope data
through the existing Himalaya setup and keeps command details inside the backend
adapter.
The adapter currently tries the stable Himalaya v1 command first
(`himalaya envelope list --output json`) and falls back to the in-development
v2 shape (`himalaya envelopes list --json`) only when the command shape is
incompatible. It paginates through the mailbox in the background instead of
capping the inbox at the first page or blocking startup on every old message.
Runtime/setup errors, such as authentication or unknown-account failures, are
shown directly instead of being hidden behind another fallback.

## Development

Run the project locally with:

```sh
go run .
```

Check the Himalaya setup without opening the full-screen TUI:

```sh
go run . doctor
```

Run the verification suite:

```sh
go test ./...
go build ./...
```

Production code no longer carries a fake inbox. Test fixtures live in the test
suite, while the app itself starts by loading envelopes from the configured
Himalaya backend.

## Planned interface

Config path:

```text
~/.config/clibox/config.toml
```

Minimal config:

```toml
account = "personal"
mailbox = "INBOX"
archive_folder = "Archive"
editor = "nvim"
confirm_delete = true
```

Default keymap:

| Key | Action |
| --- | --- |
| `j` / `k` | Move down / up |
| `Enter` | Open selected email |
| `b` | Back to the previous view |
| `r` | Reply in `$EDITOR` |
| `c` | Compose in `$EDITOR` |
| `a` | Archive selected email |
| `d` | Delete selected email, with confirmation |
| `/` | Search current mailbox |
| `R` | Refresh inbox |
| `A` | Add or update an email account inside the TUI |
| `t` | Cycle color theme |
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
- Backend: a Himalaya adapter that calls the CLI and parses JSON where
  available.
- Account setup: provider detection plus generated Himalaya config for common
  IMAP/SMTP providers, with secrets stored in macOS Keychain.
- Drafts: create temporary draft files, open `$EDITOR` with `nvim` as the
  fallback, then send through Himalaya after the editor exits.
- Config: TOML at `~/.config/clibox/config.toml`.

The Himalaya command surface differs between released and in-development
versions, so exact invocations should stay inside the adapter. The rest of the
app should ask for operations like `ListEnvelopes`, `ReadMessage`, `Reply`,
`Archive`, `Delete`, and `Search`, not build shell commands directly.

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

The inbox list is connected to Himalaya:

- Runs the configured Himalaya list command through the adapter.
- Shows the first page quickly, then loads older pages in the background.
- Parses JSON into internal envelope structs.
- Shows sender, subject, read/unread flags, and date.
- Displays clear setup errors when Himalaya is missing or incompatible.
- Refreshes the current envelope list with `R`.

### Phase 3: Real message reader

Read the selected email:

- Fetch body content through the adapter.
- Show headers and plain text content in a scrollable reader.
- Mark messages read only when the backend does so naturally or when the user
  explicitly requests that behavior later.

### Phase 4: Reply and compose

Keep writing email inside the user's editor:

- Generate reply and compose drafts.
- Open `$EDITOR`, falling back to `nvim`.
- Send only after the editor exits cleanly.
- Show a review/confirmation step before sending in v0.1.

### Phase 5: Inbox actions

Round out the daily workflow:

- `a` archives the selected email.
- `d` deletes with confirmation.
- `/` searches the current mailbox.
- `R` refreshes the current mailbox.
- Errors appear inline without crashing the TUI.

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

## Non-goals for v0.1

- Native IMAP/SMTP implementation.
- Full HTML email rendering.
- Calendar support.
- AI features.
- Attachment-heavy workflows.
- PGP, S/MIME, or advanced security workflows.
- A plugin system.

Those can come later if the core inbox loop feels excellent.
