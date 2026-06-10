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
- Lightweight inline image rendering in the full reader for native-backend
  embedded images, with a visible image label fallback.
- Compose and reply inside the TUI.
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
| `r` | Reply in clibox |
| `c` | Compose in clibox |
| `t` | Choose a theme |
| `?` | Show help |
| `q` | Quit |

On wide terminals, the right pane previews the selected email automatically.
Long subjects, headers, and message bodies are constrained to the terminal width
so moving through the list does not leave wrapped or stale rows behind. Press
`Enter` only when you want the full reader.

When the native backend loads an email with embedded image parts, the full
reader shows the image inline when the terminal supports it. The inbox preview
stays text-only, and clibox does not fetch remote images. Terminals without
inline-image support still show an `Image` label for each loaded image.

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

### Compose And Reply

Compose and reply stay inside the TUI by default:

| Key | Action |
| --- | --- |
| `Tab` | Move between `To`, `Subject`, and `Body` |
| `Enter` | Move to the next field, or add a newline in `Body` |
| `Ctrl+S` | Send |
| `Ctrl+O` | Open the draft in your external editor |
| `Esc` | Discard the draft |

The external editor is optional. It uses `CLIBOX_EDITOR`, `VISUAL`, `EDITOR`,
or `nvim` when you press `Ctrl+O`.

### Large Inboxes

`clibox` loads the newest page first so the inbox opens quickly. When you reach
the bottom of the loaded list, press `j` again to load older mail; clibox keeps
fetching older pages until the mailbox is complete. The `--page-size` option
changes how many messages each request asks for; it is not an inbox limit.

The app checks the newest page for new mail every 30 seconds while the inbox is
idle. Press `R` any time to refresh immediately.

## Useful Commands

```sh
# Open the TUI
clibox

# Check your mail setup without opening the TUI
clibox doctor

# Use a specific account or mailbox
clibox --account personal
clibox --mailbox INBOX

# Tune large-inbox loading
clibox --page-size 50

# Themes
clibox --themes
```

## Advanced

The default backend uses [Himalaya](https://github.com/pimalaya/himalaya). A
native IMAP/SMTP backend is also available:

```sh
clibox --mail-backend native --account gmail
```

The Himalaya compatibility backend normalizes read output before rendering, so
raw message headers and MIME part markers are not shown in the inbox preview or
reader.

Native Gmail and Outlook OAuth currently require your own desktop/native OAuth
client ID. Native account helpers are available when you need them:

```sh
clibox auth add --email you@gmail.com --account gmail
clibox auth login --account gmail
clibox accounts
```

Config lives at `~/.config/clibox/config.toml`. Credentials are not written to
that file; clibox stores secrets in the OS credential store.

## Development

```sh
go run .
go test ./...
go build ./...
go vet ./...
```

## License

MIT. See [LICENSE](LICENSE).
