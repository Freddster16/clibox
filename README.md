# clibox

`clibox` is a fast terminal inbox for people who do not want to leave their
coding flow to read and reply to email.

The goal is a real keyboard-first email TUI, not a web wrapper. It should feel
at home next to Neovim, Codex, OpenCode, Hermes, tmux, and a shell: open the
inbox, move with `j/k`, read with `Enter`, reply in `$EDITOR`, archive with
`a`, search with `/`, change themes with `t`, and quit with `q`.

Status: Phase 1 is implemented. `clibox` currently opens a fake inbox TUI so
the keyboard flow and layout can be tested before real email is connected.

## Install

Install the latest version from GitHub:

```sh
curl -fsSL https://raw.githubusercontent.com/Freddster16/clibox/main/install.sh | sh
```

The installer requires Go 1.24 or newer and installs `clibox` with `go install`.
If your shell cannot find `clibox` after installation, add Go's bin directory to
your `PATH`; the installer prints the exact path.

For local development:

```sh
go run .
```

Phase 1 does not require an email account. It uses fake messages so the TUI can
be tested immediately.

If you already installed `clibox` and want the latest UI changes:

```sh
go install github.com/Freddster16/clibox@latest
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

j/k move  enter read  r reply  c compose  a archive  / search  t themes  ? help  q quit
```

On wide terminals, `clibox` should show a mailbox rail, inbox list, and reader
preview in one screen. On narrow terminals, it should collapse into focused
screens: mailbox/list, reader, compose/review.

The first run should be boring in the best way:

1. Run `clibox`.
2. Pick an account if more than one account exists.
3. Land in the inbox.
4. Press `Enter` to read, `b` to go back, `r` to reply, `a` to archive, `/` to
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

## Planned real-email quick start

`clibox` should rely on [Himalaya](https://github.com/pimalaya/himalaya) for
email protocols at first. Himalaya already handles the hard parts: accounts,
IMAP/JMAP/SMTP, message envelopes, folders, authentication, and sending.

Planned flow:

```sh
# 1. Install and configure Himalaya first.
himalaya

# 2. Start the TUI.
clibox

# 3. Optionally choose account or mailbox at launch.
clibox --account personal
clibox --mailbox INBOX

# 4. Check local setup.
clibox doctor
```

`clibox` should not store email credentials. It should read account metadata
from the existing Himalaya setup and keep only UI preferences in its own config.

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

Connect the inbox list to Himalaya:

- Run the configured Himalaya list command through the adapter.
- Parse JSON into internal envelope structs.
- Show sender, subject, flags, and date.
- Display clear setup errors when Himalaya is missing or incompatible.

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
