# clibox

`clibox` is a keyboard-first email app for your terminal.

It gives you a fast TUI for reading, searching, replying, composing, archiving,
and deleting mail without leaving your shell.

## Features

- Inbox, unread, archive, sent, drafts, and trash views.
- Preview pane on wide terminals and a full reader for longer messages.
- Compose and reply inside the TUI, with optional external editor support.
- Search, refresh, archive, delete, and theme controls from the keyboard.
- Native IMAP/SMTP support plus the default Himalaya compatibility backend.
- Secrets stored in the OS credential store, not in the config file.

## Install

Install or update the latest `main` build:

```sh
curl -fsSL https://raw.githubusercontent.com/Freddster16/clibox/main/install.sh | sh
```

Start the app:

```sh
clibox
```

The installer checks for Go and the default Himalaya backend. To use only the
native backend and skip Himalaya:

```sh
curl -fsSL https://raw.githubusercontent.com/Freddster16/clibox/main/install.sh | CLIBOX_SKIP_HIMALAYA=1 sh
```

Install from source:

```sh
git clone https://github.com/Freddster16/clibox.git
cd clibox
go install .
```

## First Run

1. Run `clibox`.
2. Enter your email address if setup is needed.
3. Follow the provider instructions in the TUI.
4. Paste an app password or complete browser login when prompted.

`clibox` supports common providers such as Gmail, iCloud, Outlook, Yahoo,
Fastmail, Proton Mail, and custom IMAP/SMTP domains.

## Keyboard Basics

| Key | Action |
| --- | --- |
| `j` / `k` | Move down or up |
| `Enter` | Open the selected email |
| `Tab` | Switch to the mailbox rail |
| `/` | Search the current mailbox |
| `R` | Refresh now |
| `r` | Reply |
| `c` | Compose |
| `a` | Archive |
| `d` | Move to Trash |
| `t` | Choose a theme |
| `?` | Show help |
| `q` | Quit |

In compose and reply, press `Tab` to move between fields, `Ctrl+S` to send, and
`Ctrl+O` to open the draft in your external editor.

## Useful Commands

```sh
clibox                         # open the TUI
clibox doctor                  # check mail setup
clibox accounts                # list native accounts
clibox sync                    # sync native mail cache
clibox --account personal      # use a named account
clibox --mailbox INBOX         # open a mailbox
clibox --page-size 50          # tune large inbox loading
clibox --themes                # list themes
```

Native account helpers:

```sh
clibox auth add --email you@gmail.com --account gmail
clibox auth login --account gmail
clibox --mail-backend native --account gmail
```

Native Gmail and Outlook OAuth currently require your own desktop/native OAuth
client ID.

Config lives at `~/.config/clibox/config.toml`. Credentials are stored in the OS
credential store.

## Development

```sh
go run .
go test ./...
go build ./...
go vet ./...
```

Architecture notes live in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## License

MIT
