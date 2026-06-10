# clibox

`clibox` is a keyboard-first email app for your terminal.

It gives you the familiar mail workflow - inbox, reader, compose, reply,
archive, delete, search, and refresh - without pulling you out of your shell,
editor, tmux session, or coding flow.

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

## What It Does

- Opens a real email inbox in a Bubble Tea terminal UI.
- Shows the newest mail first and lets you load older pages when you need them.
- Previews the selected email in the wide reader pane, then opens the full
  reader with `Enter`.
- Composes and replies in your editor using `CLIBOX_EDITOR`, `VISUAL`,
  `EDITOR`, or `nvim`.
- Sends only after a review screen.
- Archives, deletes, searches, and refreshes from the keyboard.
- Supports setup from inside the TUI.
- Stores secrets in the OS credential store, not in the config file.
- Supports themes: Nocturne, Ember, and Lagoon.

## Status

`clibox` is usable as a terminal inbox today.

The default backend uses [Himalaya](https://github.com/pimalaya/himalaya) for
compatibility. A native backend is also available behind `--mail-backend native`
or `backend = "native"`. Native mode includes IMAP/SMTP, SQLite envelope/body
cache, OS keychain secret storage, and OAuth plumbing for Gmail and Outlook.

Gmail and Outlook native OAuth currently require you to provide your own
desktop/native OAuth client ID. Public one-click OAuth still needs verified
clibox provider client IDs before it can be the default for everyone.

## Install

Install or update the latest `main` build:

```sh
curl -fsSL https://raw.githubusercontent.com/Freddster16/clibox/main/install.sh | sh
```

Then run:

```sh
clibox
```

The installer checks for Go and the Himalaya compatibility backend. If Homebrew
is already installed, it can use Homebrew to install missing dependencies. It
will not install Homebrew itself unless you explicitly opt in:

```sh
curl -fsSL https://raw.githubusercontent.com/Freddster16/clibox/main/install.sh | CLIBOX_INSTALL_HOMEBREW=1 sh
```

If you only want the native backend and do not want Himalaya installed:

```sh
curl -fsSL https://raw.githubusercontent.com/Freddster16/clibox/main/install.sh | CLIBOX_SKIP_HIMALAYA=1 sh
```

## First Run

1. Start the app:

   ```sh
   clibox
   ```

2. If setup is needed, enter your email address.

3. Follow the provider guidance shown in the TUI.

4. Paste your provider app password or complete browser login when prompted.

5. Land in your inbox.

`clibox` detects common providers such as Gmail, iCloud, Outlook, Yahoo,
Fastmail, Proton Mail, and custom domains. App passwords and refresh tokens are
stored in the OS credential store.

## Daily Use

| Key | Action |
| --- | --- |
| `Tab` | Focus the mailbox rail; when focused, move to the next mailbox |
| `j` / `k` | Move in the inbox, choose a mailbox, or scroll in the reader |
| `Enter` | Open the selected email in the full reader or open the focused mailbox |
| `PgUp` / `PgDn` | Jump through an open email |
| `Home` / `End` | Jump to the top or bottom of an open email |
| `b` / `Esc` | Go back |
| `r` | Reply in your editor |
| `c` | Compose in your editor |
| `s` | Send the reviewed draft |
| `e` | Edit the reviewed draft again |
| `a` | Archive the selected email |
| `d` | Move the selected email to Trash, with confirmation |
| `/` | Search the current mailbox |
| `R` | Refresh the current mailbox |
| `A` | Add or update an account |
| `t` | Choose a theme |
| `?` | Show help |
| `q` | Quit or close the current view |

### Mailbox Navigation

Use the mailbox rail on the left side of the TUI to move between folders:

1. Press `Tab` to focus the mailbox rail.
2. Press `Tab`, `j`, or `k` to choose `Inbox`, `Unread`, `Archive`, `Sent`,
   `Drafts`, or `Trash`.
3. Press `Enter` to open the selected mailbox or filter.
4. Press `Esc`, `b`, or the right arrow to return focus to the message list.

`Unread` is an unread-only view of your inbox. From `Unread`, press `Esc` to go
back to all inbox mail.

Large inboxes are handled page by page. The newest page appears first so the app
feels responsive. When you reach the bottom of the loaded list, press `j` again
to load older mail. `--page-size` changes the request size; it is not an inbox
limit.

On wide terminals, the right-hand reader pane previews the selected email
automatically. Press `Enter` only when you want to open that email in the full
reader. The app checks the newest page for new mail every 30 seconds while the
inbox is idle; press `R` to refresh immediately.

## Useful Commands

```sh
# Open the TUI
clibox

# Choose an account or mailbox
clibox --account personal
clibox --mailbox INBOX

# Use the native backend
clibox --mail-backend native --account gmail

# Check setup without opening the TUI
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

# Show available themes
clibox --themes
```

Editor selection also works through environment variables:

```sh
EDITOR=nvim clibox
VISUAL="code --wait" clibox
CLIBOX_EDITOR="vim -n" clibox
```

## Native OAuth

Native Gmail or Outlook OAuth is available for developers and testers with their
own OAuth client IDs.

```sh
clibox auth add --email you@gmail.com --account gmail

export CLIBOX_GMAIL_CLIENT_ID="your-google-desktop-client-id"
export CLIBOX_OUTLOOK_CLIENT_ID="your-microsoft-public-client-id"

clibox auth login --account gmail
clibox sync --account gmail --mailbox INBOX
clibox --mail-backend native --account gmail --mailbox INBOX
```

OAuth login uses the system browser, a local loopback callback, PKCE, and state
validation. Refresh tokens are stored in the OS keychain.

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

## Security Notes

- Email credentials are not written to `config.toml`.
- Native mode stores app passwords and refresh tokens in the OS credential store.
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
