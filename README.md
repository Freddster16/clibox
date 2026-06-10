# clibox

`clibox` is a keyboard-first email app for your terminal.

It gives you a familiar mail workflow in a TUI: inbox, unread, archive, sent,
drafts, preview, full reader, compose, reply, search, refresh, archive, and
delete. It is built for people who live in a shell, tmux session, or editor and
do not want to switch apps just to handle mail.

```text
clibox                                                50 emails

Mailboxes          Inbox                         Reader
* Inbox 50       > Alice        Re: Design notes      From: Alice <alice@example.com>
  Unread 2         GitHub       New issue assigned    Subject: Re: Design notes
  Archive          Vercel       Deployment failed     Date: Today
  Sent
  Drafts                                           Hey Freddy,

                                                    I looked at the prototype...

tab mailboxes  j/k move  enter full reader  R refresh  r reply  c compose  a archive  ? help
```

## Features

- Terminal mail UI powered by Bubble Tea.
- Mailbox rail for `Inbox`, `Unread`, `Archive`, `Sent`, `Drafts`, and `Trash`.
- Wide-screen preview pane that follows the selected email.
- Full reader for longer messages.
- Compose and reply in your editor.
- Review screen before sending.
- Keyboard search, refresh, archive, and delete.
- Newest mail loads first; older pages load only when you ask for them.
- Idle inbox refresh checks for new mail every 30 seconds.
- Credentials are stored in the OS credential store, not in the config file.
- Themes: Nocturne, Ember, and Lagoon.

## Install

Install or update the latest `main` build:

```sh
curl -fsSL https://raw.githubusercontent.com/Freddster16/clibox/main/install.sh | sh
```

Then start the app:

```sh
clibox
```

The installer checks for Go and the default Himalaya compatibility backend. If
Homebrew is already installed, the installer can use it for missing
dependencies. It will not install Homebrew unless you opt in:

```sh
curl -fsSL https://raw.githubusercontent.com/Freddster16/clibox/main/install.sh | CLIBOX_INSTALL_HOMEBREW=1 sh
```

If you only want the native backend and do not want the installer to install
Himalaya:

```sh
curl -fsSL https://raw.githubusercontent.com/Freddster16/clibox/main/install.sh | CLIBOX_SKIP_HIMALAYA=1 sh
```

Developer install from source:

```sh
git clone https://github.com/Freddster16/clibox.git
cd clibox
go install .
```

## First Run

1. Run `clibox`.
2. If setup is needed, enter your email address.
3. Follow the provider guidance in the TUI.
4. Paste your app password or complete browser login when prompted.
5. Land in your inbox.

`clibox` detects common providers such as Gmail, iCloud, Outlook, Yahoo,
Fastmail, Proton Mail, and custom IMAP/SMTP domains.

## How To Use The TUI

### Message List

| Key | Action |
| --- | --- |
| `j` / `k` | Move down or up |
| `Enter` | Open selected email in the full reader |
| `R` | Refresh now |
| `/` | Search the current mailbox |
| `a` | Archive selected email |
| `d` | Move selected email to Trash, with confirmation |
| `r` | Reply in your editor |
| `c` | Compose in your editor |
| `t` | Choose a theme |
| `?` | Show help |
| `q` | Quit |

On wide terminals, the right pane previews the selected email automatically.
Press `Enter` only when you want the full reader.

### Mailboxes

Use the left mailbox rail to move between folders:

1. Press `Tab` to focus the mailbox rail.
2. Press `Tab`, `j`, or `k` to choose a mailbox.
3. Press `Enter` to open it.
4. Press `Esc`, `b`, or the right arrow to return to the message list.

`Unread` is an unread-only view of your inbox. From `Unread`, press `Esc` to
return to all inbox mail.

### Reader

| Key | Action |
| --- | --- |
| `j` / `k` | Scroll |
| `PgUp` / `PgDn` | Jump by page |
| `Home` / `End` | Jump to top or bottom |
| `b` / `Esc` | Back to inbox |
| `r` | Reply |
| `a` | Archive |
| `d` | Delete |

### Large Inboxes

`clibox` loads the newest page first so the inbox opens quickly. When you reach
the bottom of the loaded list, press `j` again to load older mail. The
`--page-size` option changes how many messages each request asks for; it is not
an inbox limit.

The app checks the newest page for new mail every 30 seconds while the inbox is
idle. Press `R` any time to refresh immediately.

## Common Commands

```sh
# Open the TUI
clibox

# Choose an account or mailbox
clibox --account personal
clibox --mailbox INBOX

# Pick the native backend
clibox --mail-backend native --account gmail

# Check the connection without opening the TUI
clibox doctor
clibox doctor --account personal
clibox doctor --mail-backend native --verbose --account gmail

# Native account helpers
clibox auth add --email you@gmail.com --account gmail
clibox auth login --account gmail
clibox accounts
clibox sync --account gmail --mailbox INBOX

# Editor and behavior options
clibox --editor "nvim"
clibox --page-size 50
clibox --confirm-delete=false
clibox --archive-folder "[Gmail]/All Mail"

# Themes
clibox --themes
```

Editor selection also works through environment variables:

```sh
EDITOR=nvim clibox
VISUAL="code --wait" clibox
CLIBOX_EDITOR="vim -n" clibox
```

## Backends

`clibox` has two mail backends:

- `himalaya`: default compatibility backend using
  [Himalaya](https://github.com/pimalaya/himalaya).
- `native`: built-in IMAP/SMTP backend with SQLite envelope/body cache, OS
  keychain secret storage, and OAuth plumbing for Gmail and Outlook.

Use native mode with:

```sh
clibox --mail-backend native --account gmail
```

Native Gmail and Outlook OAuth currently require your own desktop/native OAuth
client ID:

```sh
clibox auth add --email you@gmail.com --account gmail

export CLIBOX_GMAIL_CLIENT_ID="your-google-desktop-client-id"
export CLIBOX_OUTLOOK_CLIENT_ID="your-microsoft-public-client-id"

clibox auth login --account gmail
clibox sync --account gmail --mailbox INBOX
clibox --mail-backend native --account gmail
```

## Configuration

Config lives at:

```text
~/.config/clibox/config.toml
```

Minimal example:

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

Command-line flags override config values for that launch.

For compatibility with older configs, `backend = "/path/to/himalaya"` is treated
as a Himalaya binary path. New configs should prefer:

```toml
backend = "himalaya"
himalaya_binary = "/opt/homebrew/bin/himalaya"
```

Credential-like config keys are rejected on purpose, including `password`,
`access_token`, `refresh_token`, `id_token`, and `client_secret`.

## Security

- Email credentials are not written to `config.toml`.
- Native mode stores app passwords and refresh tokens in the OS credential
  store.
- Native mode stores account metadata and cached mail data in SQLite.
- Draft files are temporary owner-only files.
- Draft content is sent to the backend over stdin, not as command-line
  arguments.

## Development

Run locally:

```sh
go run .
```

Run checks:

```sh
go test ./...
go build ./...
go vet ./...
```

Useful development commands:

```sh
go run . doctor
go run . doctor --mail-backend native --verbose
go run . --themes
```

The app is written in Go with:

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) for the TUI loop.
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) for terminal styling.
- [Himalaya](https://github.com/pimalaya/himalaya) for the compatibility
  backend.
- `github.com/emersion/go-imap`, `go-smtp`, `go-message`, and `go-sasl` for
  native mail.
- SQLite for the native cache.
- OS keychain storage through `github.com/zalando/go-keyring`.

## Current Limits

- Public one-click Gmail/Outlook OAuth is not enabled yet.
- HTML email is reduced to readable text.
- Attachments are not a primary workflow yet.
- Calendar, AI, PGP, S/MIME, and plugin support are out of scope for the current
  pass.

## License

MIT. See [LICENSE](LICENSE).
