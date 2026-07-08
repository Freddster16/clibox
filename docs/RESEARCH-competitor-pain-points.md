# Terminal/TUI Email Client Competitor Research — Pain Points & Wishlist

Research goal: identify what users dislike about existing terminal/TUI email clients so **clibox** (a new Go-based terminal email client) can avoid those mistakes.

## Methodology & sources

Sources fetched (webfetch):
- GitHub issue lists sorted by 👍 reactions: `pimalaya/himalaya`, `neomutt/neomutt`, `pazz/alot`. (aerc's GitHub repo `aerc/aerc` 404s — the project moved to sourcehut; bugs fetched from `todo.sr.ht/~rjarry/aerc`.)
- himalaya GitHub Discussions (`/discussions`).
- Hacker News: Algolia JSON API for `himalaya`, `aerc`, `neomutt`; then server-rendered comment threads for the highest-signal posts:
  - himalaya "CLI to Manage Emails" (349 pts, 97 comments) — https://news.ycombinator.com/item?id=42366025
  - aerc launch "An email client that runs in the terminal" (722 pts, 264 comments) — https://news.ycombinator.com/item?id=20090950
  - aerc blog "A well-crafted TUI for email" (298 pts, 151 comments) — https://news.ycombinator.com/item?id=41321981
  - aerc "A pretty good (terminal) email client" (205 pts, 50 comments) — https://news.ycombinator.com/item?id=33166054 (rate-limited/429 on direct fetch; signals captured via the other aerc threads)
  - "Ask HN: Is the state of mail user agents that sad?" (19 pts) — https://news.ycombinator.com/item?id=30275629 (body in Algolia JSON)
- Blog reviews / setup write-ups:
  - SeniorMars, "A Terminal Email Client As An Alternative To Gmail: The Old Dog Neomutt" — https://seniormars.com/posts/neomutt/
  - Ben Swift, "The great 2025 email yak-shave: O365 + mbsync + mu + neomutt + msmtp" — https://benswift.me/blog/2025/09/12/the-great-2025-email-yak-shave-o365-mbsync-mu-neomutt-msmtp
  - SergeantBiggs, "Aerc: A well-crafted TUI for email" — https://blog.sergeantbiggs.net/posts/aerc-a-well-crafted-tui-for-email/

Note on fetch failures: Reddit (`r/commandline`, `r/emailprivacy`, `r/neovim`, `r/emacs`, `r/unixporn`) blocks unauthenticated scraping (returns login walls / JS-only pages) and `mutt`'s tracker is on GitLab with a JS-rendered issue list; these were not directly harvestable. The HN + GitHub + sourcehut + blog sources above already provided strong, overlapping signal across every requested category, so the conclusions are well-supported. Specific Reddit-style complaints (vim keybindings, theming, mutt-wizard politics, Fastmail praise) still surfaced indirectly via HN commenters quoting/linking Reddit.

---

## 1. himalaya (pimalaya/himalaya) — Rust CLI email client

himalaya is a **CLI** (not a TUI) "building block" tool with JSON output, prized for scriptability. Its issues tracker is small (the project deliberately scopes features tightly), which itself is a complaint: power users want a TUI and OAuth and were told "not planned."

### Top complaints / pain points
1. **No real TUI.** The most-reacted feature issue is "Implement the TUI" (#24), closed as *Not planned (skipped)*. Users repeatedly ask "is this different than PINE / is the screenshot interactive?" and are told "Himalaya is not interactive: while Pine is a TUI, this is a CLI" (HN 42366025). People want a curses UI, not just subcommands.
2. **OAuth 2.0 was declined.** #398 "OAuth 2.0 support" closed *Not planned (skipped)*. With Gmail killing "less secure apps" and O365 disabling password IMAP, this is a blocker for many. Users instead must cobble `cyrus-sasl-xoauth2` + `mutt_oauth2.py` themselves.
3. **No Markdown→multipart/alternative HTML composition.** The `MML` MIME markup is "neither markdown based nor does it automatically build the plain text part for you" (HN gorgoiler). People "end up writing markdown anyway" and want it auto-converted.
4. **Config/password friction.** #108 "Add raw passwd in config" wontfix (good instinct), but the supported alternatives (PassCmd to gpg, etc.) require setup. "email is a pain to configure, hesitant to pull the trigger before a 1.0" (HN jarbus).
5. **JSON output inconsistency across commands** (#547, open) — hurts the very scriptability use case it targets.
6. **Robustness:** #104 "Panic on special char (non-ASCII)", #175 "IMAP server error".

### Features people wish existed
- A first-class TUI (PINE-like) on top of the CLI. (#24)
- OAuth2 / Office 365 / 2FA (SMS, Yubikey) support. (#398, HN hk1337, jxf)
- JMAP support (HN pydry: "No JMAP though :/")
- Programmatic Gmail-filter management / filter import-export (HN aynawn)
- "From" header rewriting for plus-addresses/aliases (HN xyst, twice across threads)
- Notmuch-style local indexing/search (#57)
- Maildir local folder reading (#43), end-to-end encryption (#54), attachments (#47) — all now implemented but were top requests.

### Category breakdown
- **Setup/onboarding:** App-password/OAuth pain (Gmail "less secure apps" sunset, O365 password IMAP off). macOS `isync` requires building `cyrus-sasl-xoauth2` from source and patching the Homebrew formula (SeniorMars). Credential storage: gpg-encrypted file via `PassCmd`; no native keychain/secret-service integration.
- **HTML rendering:** Out of scope for a CLI — handed to external `w3m`/`lynx`/`pandoc`. Users want inline HTML→text and inline images (iTerm/Sixel).
- **Attachments:** #47 was the top attachment request (add attachments); now done.
- **Search:** No built-in full-text index; relies on external notmuch. #57 was the wishlist.
- **Multi-account:** Works via config but per-alias "From" rewriting is manual.
- **Threading:** N/A (CLI returns envelopes).
- **Notifications/new mail:** None — users run external `goimapnotify` + cron/systemd timers (HN 0fflineuser).
- **PGP/encryption:** #54 E2EE was requested; PGP today is via external tooling.
- **Compose/editor:** Delegates to `$EDITOR`; no built-in Markdown→HTML; MML is awkward.
- **Performance:** Good (Rust, stateless CLI); IMAP round-trips on every invocation.
- **Stability:** Occasional panics on non-ASCII (#104); IMAP server errors (#175).

Sources: https://github.com/pimalaya/himalaya/issues?q=is%3Aissue+sort%3Areactions-%2B1-desc ; https://github.com/pimalaya/himalaya/discussions/118 ; https://news.ycombinator.com/item?id=42366025

---

## 2. aerc (sourcehut ~rjarry/aerc, formerly aerc/aerc) — Go TUI email client

aerc is the most-praised modern TUI ("well-crafted," "email client of my dreams") but also attracts detailed criticism. The GitHub mirror is gone (404); the live tracker is `todo.sr.ht/~rjarry/aerc`.

### Top complaints / pain points
1. **IMAP disconnects / hangs requiring restart.** "Given the above I'm really surprised that IMAP doesn't work better than it does. Often gets disconnected and I have to restart to get back to it." Workaround: "I just started running aerc in a `while` loop and I hit q to quit if it's acting up." (HN 41321981, tjoff + opan)
2. **HTML email rendering is the #1 reason people leave.** "I miss using a TUI for mail but it looks like they haven't really solved the most important reason I don't, which is rendering HTML emails." (kemiller). Default is w3m pipe — no CSS, no images, inconsistent. "In most multipart emails the plain text version is much less readable than the HTML version… sometimes just a garbled mess." Authoring/replying to HTML is even worse.
3. **No/late Windows support + mailcap hell on Windows.** "Mutt/aerc doesn't support windows, an OS many use by preference or requirement… hard getting anything to work in mailcap on windows." (djha-skin) — a common reason to "give up and use Betterbird."
4. **IMAP performance & non-cancellable fetches.** "100-200ms to load a single message… from the localhost. Maybe naggle?" "I can't cancel an IMAP request once it goes out… I have to wait for ones that scrolled offscreen long ago." Scrolling fetches headers one-at-a-time. (HN 20090950)
5. **Setup is a yak-shave** when combined with sync tooling: "took maybe a dozen hours to get it set up" (abound, aerc+notmuch+lieer/mbsync). Early aerc required `scdoc` to build and shipped no prebuilt binaries ("not designed to be installed from pre-built binaries").

### Features people wish existed
- **Unified inbox** across accounts (aerc #348 "Unified Inbox/Sent"; HN "a way to have all accounts in the same tab, with a unified inbox").
- **"Reply to list"** (aerc #276).
- **IMAP4rev2 / SMTPUTF8** (aerc #328), **Memory Hole / PGP-encrypted Subject** (#347), **custom PGP recipient key** (#349).
- **Inline image preview of attachments** (HN rafo built it manually via Sixel/iTerm).
- **Markdown→multipart/alternative** authoring (repeated HN wishlist).
- **Per-recipient auto "From"** for plus-addresses (HN xyst: "doesn't automatically fill it in based on the recipient in a reply").
- **Better message forwarding** incl. forwarding a thread as `message/rfc822` attachments (aerc #344; HN).
- **Robust MIME/UTF-7** (aerc #340 utf-7; #323 "malformed MIME header key: From").

### Category breakdown
- **Setup/onboarding:** Account wizard is *praised* ("I wish Mutt had a mail account configurator like the one in Aerc") — a real differentiator. But passwords stored plaintext post-wizard raised questions (HN "Why account passwords are stored on plaintext after the wizard?"); `PassCmd` (gpg) is the secure option. OAuth2 *is* supported and has been for years (HN tristan957), unlike himalaya — a plus.
- **HTML rendering:** w3m by default; pipe-to-browser on a key for complex mail. No images, no CSS, links need `urlview`-style extractor. Sixel/iTerm inline images only via custom config.
- **Attachments:** "Is there any way I can see the name of attached files?" (HN). Bug #345 "Attach file with `[` in filename". Image preview requires manual setup.
- **Search:** notmuch backend exists but buggy: #160 "incorrectly handles notmuch queries", #341 "dirtree notmuch regex queries create wrong subfolders", #106 "wrong message file paths when using notmuch backend". Full-text search was a top wishlist before notmuch landed.
- **Multi-account:** Multiple accounts supported, but unified inbox missing (#348); per-alias From is manual.
- **Threading:** `threading-enabled=true` *ignores sort* (#265) — a real bug. Early aerc "doesn't support threads, which is a must for MUA" (HN sulfastor) — since added.
- **Notifications/new mail:** `check-mail-cmd` exists; bug #350 "check-mail-cmd spawned processes cannot use pinentry-curses — aerc holds TTY" breaks GPG. No first-class IDLE push.
- **PGP/encryption:** "First-class PGP support is planned and a blocker for 1.0" (2019). Today: bug #306 "aerc is unable to decrypt mail"; WKD (Web Key Directory) requested but absent; Memory Hole (#347) missing.
- **Compose/editor:** Embedded terminal for `$EDITOR` is the headline feature but buggy: #343 "emacs as embedded editor has display issues", #346 "Ctrl+Backspace does not work correctly with micro", #350 pinentry TTY conflict. Passthrough mode helps shortcut overlap with neovim (HN).
- **Performance:** Lazy header fetch helps cold open vs mutt, but per-message IMAP latency (100-200ms even on localhost) and non-cancellable requests hurt scrolling on big folders.
- **Stability:** IMAP disconnect/restart loop; malformed-MIME parse failures (#323); notmuch backend correctness bugs.

Sources: https://todo.sr.ht/~rjarry/aerc ; https://news.ycombinator.com/item?id=20090950 ; https://news.ycombinator.com/item?id=41321981

---

## 3. neomutt / mutt (neomutt/neomutt; mutt on GitLab) — C TUI, the incumbent

neomutt is the mature powerhouse ("the best email user agent") but configuration is legendary in its difficulty and it shows its age.

### Top complaints / pain points
1. **Configuration is a nightmare / pile of hacks.** "I've spent a lot of time configuring mutt just the way I like it, but I'd throw it all away in a heartbeat for a good, modern alternative." "On Mutt any non-trivial extension was just a pile of hacks. For example, multiple accounts." (HN nextos) The standard setup chains *five+ external tools*: `mbsync`/`offlineimap` + `notmuch`/`mu` + `msmtp` + `gpg` + `lynx`/`w3m` (+ `lbdbq`/`goobook` for contacts, `urlscan` for links, `imapfilter`/`afew` for tagging).
2. **OAuth2 / O365 is brutal.** Ben Swift's 2025 write-up: had to *build mbsync from source with SASL*, build `cyrus-sasl-xoauth2` from source, use `mutt_oauth2.py` with *Thunderbird's* client ID and the device-code flow, and wrap token storage in macOS Keychain by hand. SeniorMars's macOS guide requires patching the Homebrew `isync.rb` formula. "mbsync/offlineimap don't support OAuth well." (djha-skin)
3. **HTML mail via external `mailcap`/`w3m`** — "Rendering HTML emails is fine, IMO, unless you want images… What's more difficult is authoring/replying to HTML emails." (HN) Plaintext purists fight the 80-column war; `format=flowed` is "format=flawed" (mangles mails, breaks signatures, lost when non-f=f clients reply).
4. **No Markdown→multipart/alternative out of the box.** Top wishlist across two HN threads; neomutt #587 "Better support for multipart/alternative" was the implementation tracker. Users hack it with `pandoc` macros.
5. **Contacts/autocomplete pain.** SeniorMars "things left desired": "I would like to auto complete email addresses from my contacts." Requires `lbdbq`/`goobook`/`vdirsyncer` + manual alias caching scripts. "no good way to search/auto-complete addresses" (HN).

### Features people wish existed
- Markdown→multipart/alternative HTML sending (#587; HN rlue/mrzool/masukomi).
- `format=flowed` done right, or its replacement (#206 in himalaya; FastMail blog on why f=f is broken).
- Vim keybindings by default (#56 "Vi-key bindings for mutt" — top-reacted neomutt issue!), Escape-closes-prompt vim behaviour (#1510), true colors (#85).
- Proxy support (#99), IMAP `BODYSTRUCTURE` fetching for speed (#786).
- Better attachment saving UX (#1603), mouse support (#309).
- Native OAuth2 + Office365 without building SASL plugins from source.

### Category breakdown
- **Setup/onboarding:** The worst of the bunch. SeniorMars's *single blog post* walks through neomutt + isync + msmtp + notmuch + gpg + lynx + lbdbq + urlscan + mailcap, including a from-source SASL plugin build on macOS. "It's not that hard… for less geek/expert users" — there is "NOTHING" (HN kkfx).
- **HTML rendering:** `auto_view text/html` via `lynx`/`w3m` in `mailcap`; pipe-to-browser for hard cases; no images without Kitty/Sixel hacks. "Sometimes I want images lmao!" (SeniorMars).
- **Attachments:** Saving is clunky (#1603); handled via `mailcap` per MIME type.
- **Search:** Built-in search is limited; everyone bolts on `notmuch` or `mu`. "full text search… brought me from mutt to sup to alot" (HN).
- **Multi-account:** "a pile of hacks" (nextos); folder hooks, source commands, separate smtp lines per account.
- **Threading:** A strength — "fantastic threads support" (HN). Reverse-threads + `sort_aux` is the canonical setup.
- **Notifications/new mail:** `mail_check` polling + `imap_keepalive`; no real IDLE push. Users add `goimapnotify`/`imapnotify` systemd services to trigger `mbsync`.
- **PGP/encryption:** GPGME integration is mature (`crypt_use_gpgme`, `crypt_autosign`, `crypt_replyencrypt`, opportunistic encrypt). Setup still requires `gpg --full-generate-key` and gpg-encrypted password files. `crypt_opportunistic_encrypt` can trip spam filters (SeniorMars).
- **Compose/editor:** `$EDITOR` (nvim) with `ftplugin/mail` for spell/wrap; signatures via `set signature`; format=flowed pain.
- **Performance:** Fast once cached (`header_cache`/`message_cachedir`), but opening large IMAP folders downloads all headers upfront (slow cold open). Gnus on 40k+ messages takes ">1.5 hours to open" — a cautionary tale.
- **Stability:** Rock-solid reputation ("hasn't changed a whole lot in decades… not running into a whole lot of bugs"). The fragility is in the *surrounding* sync toolchain (offlineimap IDLE "hard-to-reproduce bugs"; mbsync Gmail UID issues — "mbsync keeps messing up the UID of emails with gmail", HN msravi).

Sources: https://github.com/neomutt/neomutt/issues?q=is%3Aissue+sort%3Areactions-%2B1-desc ; https://seniormars.com/posts/neomutt/ ; https://benswift.me/blog/2025/09/12/the-great-2025-email-yak-shave-o365-mbsync-mu-neomutt-msmtp ; https://news.ycombinator.com/item?id=20090950

---

## 4. Alpine (alpineapp.email) — Pine successor

Less issue-tracker data available (hosted outside GitHub), but HN/blog sentiment is clear.

### Top complaints / pain points
1. **Infrequent releases / stale.** "Pine is not dead for me yet. I use alpine as daily driver, although the releases are not that often." (HN folmar) Perceived as maintenance-mode.
2. **HTML email weak.** "I would really love to try a terminal email client with html email parsing. I miss pine." (HN daft_pink) Alpine's HTML handling is basic.
3. **No images / modern MIME.** Same "I want images lmao" complaint applies.
4. **Configuration via old-school interactive setup** rather than a declarative file; less scriptable than mutt.

### Features people wish existed
- Modern HTML rendering with inline images.
- Better integration with notmuch-style search.
- OAuth2 / modern auth for Gmail/O365.

### Category breakdown
- **Setup/onboarding:** Easier than mutt (interactive setup, app passwords work) but no OAuth flows. Converting Thunderbird mbox→Maildir "took a couple of minutes with a script" (HN abc123abc123) — Maildir migration is a plus.
- **HTML/attachments/search/multi-account/threading/PGP/notifications:** All "adequate but dated"; people use Alpine *despite* its limits because core triage is fast ("read/file/delete scores of plaintext emails in seconds using single keystrokes", HN wgrover).
- **Performance:** Excellent — "memory and CPU footprint is too small to see."
- **Stability:** Very stable.

Sources: https://news.ycombinator.com/item?id=41321981 ; https://alpineapp.email/

---

## 5. sup (sup-heliotrope/sup) — Ruby TUI, notmuch's ancestor

Largely abandoned but still referenced as a gold standard for *interface* ("supmua… the ancestor of notmuch… interfaces all felt inferior to sup", HN). Complaints: unmaintained, Ruby/extension install pain, slow on huge mboxes, no HTML. Its spiritual successor `alot` inherits its tag-based model.

Sources: https://news.ycombinator.com/item?id=20090950 (sup mentions)

---

## 6. alot (pazz/alot) — Python TUI on notmuch

Small but issue-rich tracker; complaints cluster on crypto, theming, and missing conveniences.

### Top complaints / pain points (from top-reacted issues)
1. **Crypto gaps:** #1370 Autocrypt support (open), #1207 "Use sign and encrypt instead of nesting a signed part inside an encrypted part", #1232 "Warn if mail is not encrypted to all recipients", #1394 "Check alot for new crypto attacks", #1409 migrate to Python3 `EmailMessage` for crypto correctness.
2. **No HTML composition:** #1051 "compose html mails" (open).
3. **State/session restoration:** #1071 "save thread position and folds within and between sessions", #868 "undo (thread state changes) global command".
4. **Theming:** #1416 "make themes inherit from default".
5. **Editor integration broken:** #703 "pipe-to vim - does not work"; #1418 multi-line threadlines.
6. **Edge cases:** #1538 "Can't open e-mails without a body".

### Features people wish existed
- Autocrypt, robust PGP sign+encrypt semantics, recipient-key validation.
- HTML compose; persistent UI state; richer theming.

### Category breakdown
- **Setup/onboarding:** Depends on notmuch (you must sync+index first); Python packaging friction. "Most email tools like notmuch tend to be GPL which limits adoption IMHO." (HN snthpy) — licensing is a recurring adoption concern.
- **HTML rendering:** via notmuch/external; not first-class.
- **Search:** Excellent (notmuch).
- **Threading:** Tag/thread model is the point; fold-state not persisted (#1071).
- **PGP:** Most-active category — alot users care deeply about crypto correctness.
- **Performance:** Bounded by notmuch (good) and Python TUI (acceptable).
- **Stability:** Long-standing open issues suggest slow maintenance cadence.

Sources: https://github.com/pazz/alot/issues?q=is%3Aissue+sort%3Areactions-%2B1-desc

---

## 7. Cross-cutting sentiment (HN "Ask HN: Is the state of mail user agents that sad?")

A notable philosophical critique that clibox should heed: aerc's UX "was not to my liking, and I don't quite understand why a MUA should include a terminal emulator. Or why it should be a self-contained application altogether. This whole idea of a self-contained MUA application goes against the general UNIX-y paradigm of 'no application' and 'choose extend over embed'." (HN s5806533). I.e., a meaningful segment wants a **composable core + thin UI** (mh/mblaze/notmuch style), not a monolith. himalaya's CLI-first design wins this crowd; aerc's embedded-terminal wins the "I live in one app" crowd. clibox can serve both by exposing a stable CLI/JSON layer *and* a TUI on top.

Also: HTML-email-is-HTML-now reality check — "It's 2024 now… Email is not plain text any more. We can't pretend that it is." (HN dmd) vs the die-hard plaintext crowd. **Both** must be served: render HTML well *and* compose multipart/alternative.

---

## Top 20 most-requested features / most-felt pain points (consolidated, ranked)

| # | Pain point / wishlist | One-line description | Origin |
|---|---|---|---|
| 1 | **Great HTML email rendering in-terminal** | Render HTML to readable text *with inline images* (Sixel/iTerm/Kitty) and clickable links — the #1 reason people abandon TUI clients. | aerc, neomutt, alpine (HN 41321981, 20090950) |
| 2 | **Markdown → multipart/alternative compose** | Write Markdown, auto-generate `text/plain` + `text/html` parts so recipients on phones/desktops see proper reflow, not 80-col butchering. | neomutt #587, himalaya, aerc (HN rlue/mrzool/masukomi/gorgoiler) |
| 3 | **First-class OAuth2 (Gmail + Office365) without building SASL plugins** | Device-code flow, token refresh, keychain/pass storage — no app passwords, no from-source `cyrus-sasl-xoauth2`. | himalaya #398, neomutt/aerc (Ben Swift, SeniorMars, djha-skin) |
| 4 | **Zero-config onboarding / account wizard** | Auto-detect provider settings, guided OAuth, sane defaults — "I wish Mutt had a mail account configurator like the one in Aerc." | aerc (praised), neomutt (wished) |
| 5 | **Fast, cancellable IMAP + solid reconnect** | No disconnect-restart loops; cancel in-flight fetches; prefetch visible headers in background. | aerc (HN tjoff/opan, perf thread) |
| 6 | **IMAP IDLE push → instant new-mail** | Built-in IDLE daemon so 2FA/login emails arrive in seconds, not on a 15-min mbsync poll. | aerc/neomutt (HN rlue, goku12, Jaruzel) |
| 7 | **Local full-text search that just works** | notmuch/mu-grade search without separate install/config or the notmuch-backend bugs. | aerc #160/#341/#106, neomutt, himalaya #57 |
| 8 | **Unified inbox + easy multi-account** | All accounts in one view; per-recipient auto "From" for plus-addresses/aliases. | aerc #348, neomutt (HN nextos, xyst) |
| 9 | **Robust MIME parsing** | Handle "a zillion ways to do MIME wrong", UTF-7, malformed `From ` lines, non-ASCII without panics. | aerc #323/#340, himalaya #104 (HN tonyarkles) |
| 10 | **Vim keybindings by default + Escape-closes-prompt** | Top-reacted neomutt issue is *vi keybindings*; users "too old to learn new keybindings." | neomutt #56/#1510, aerc (HN Ringz) |
| 11 | **First-class PGP: sign+encrypt, WKD, Autocrypt, Memory Hole** | Decrypt reliably; encrypted Subject; auto key discovery; warn if not encrypted to all recipients. | aerc #306/#347/#349, alot #1370/#1207/#1232, neomutt |
| 12 | **Attachments: visible names, easy save, image preview, `[`-safe filenames** | See attachment filenames in list, one-key save-to-dir, inline image preview. | aerc #345, neomutt #1603, aerc-vim (HN) |
| 13 | **Clickable URLs / link extraction** | Render link text *and* address; `urlview`-style picker built-in. | neomutt (urlscan), aerc #326 "Broken URLs" (HN chme) |
| 14 | **Cross-platform incl. Windows** | First-class Windows (and WSL) support; mailcap that works on Windows. | aerc/neomutt (HN djha-skin) |
| 15 | **Contact autocomplete from system/vCard/CardDAV** | Native macOS Contacts / CardDAV / vdirsyncer autocomplete, not hand-rolled `lbdbq` scripts. | neomutt (SeniorMars), aerc #336 carddav-query bug, HN |
| 16 | **Secure credential storage (keychain/pass/gpg) out of the box** | No plaintext passwords; OS keychain integration; no manual gpg-encrypted password files. | aerc (HN plaintext question), neomutt, himalaya #108 |
| 17 | **Performance on large mailboxes** | Don't be Gnus (1.5hr open on 40k msg); don't prefetch all headers like mutt; cache bodies locally. | Gnus (HN sulfastor), neomutt, aerc |
| 18 | **Thread/collapse state persisted; threading respects sort** | Save folds/position across sessions; `threading-enabled=true` must not break sort. | aerc #265, alot #1071 |
| 19 | **Scriptable/JSON core + TUI on top (composability)** | A stable CLI/JSON layer for automation *and* a curses UI — serve both "extend over embed" and "one app" crowds. | himalaya (loved), aerc (critiqued), HN s5806533 |
| 20 | **Better forwarding (thread-as-attachment) + reply-to-list** | Forward a thread as `message/rfc822` attachments; proper `Reply-To-List`. | aerc #344/#276 (HN) |

Honorable mentions just outside the top 20: theming/style-sets (aerc strong; alot #1416), mouse support (neomutt #309, aerc #329), proxy support (neomutt #99), JMAP (himalaya HN), Gmail-label sync via API like `lieer`/`gmailctl` (HN), snooze/inbox-zero workflows (HN), calendar+addressbook integration parity with Outlook (HN).

---

## Things clibox should do differently to win users

These are concrete, actionable differentiation moves drawn from the gaps above.

### A. Make onboarding a 2-minute experience (attack the #3/#4 pain)
- Ship a `clibox setup` wizard: pick provider (Gmail / Google Workspace / O365 / Fastmail / ProtonBridge / IMAP-custom) → trigger OAuth2 device-code flow in the browser → store tokens in the OS keychain (macOS Keychain / libsecret / Windows Credential Manager / `pass` on Linux). **No app passwords, no from-source SASL builds, no `mutt_oauth2.py`.**
- Auto-detect IMAP/SMTP/JMAP endpoints from the email domain (autodiscover/MX-based). Map Sent/Drafts/Trash/Spam folders automatically (the "Sent vs Sent Mail vs Sent Messages" war — HN tzs).
- Ship **prebuilt static binaries for macOS/Linux/Windows** (+ `brew`, `winget`, `scoop`, `apt`/`pacman` packaging). Don't be early-aerc ("not designed for prebuilt binaries, needs `scdoc`").
- Sensible built-in defaults so the *zero-config* path works; expose a declarative config file for power users.

### B. Win HTML (attack the #1 pain — the actual reason people leave)
- Embed a real HTML→terminal renderer (not a raw `w3m` pipe): block remote images by default, **inline local/CID images via Sixel/iTerm/Kitty protocols**, preserve table/list structure, render **link text + URL** and make URLs openable with a keystroke (built-in `urlview`, no external dep).
- Provide an **"open in browser"** fallback on one key for hopeless mails, and an optional **LLM/summarize** hook (HN repeatedly suggested local LLMs to extract clean plaintext from gnarly HTML).
- For composing: **Markdown → `multipart/alternative`** built-in (write MD, auto-generate `text/plain` + `text/html`), plus optional `format=flowed` *done right*. This single feature is the most-repeated wishlist item across neomutt #587 and three HN threads.

### C. Be fast and never hang (attack #5/#9/#17)
- IMAP layer in Go: **concurrent, cancellable fetches**, prefetch the visible window of headers in the background, `BODYSTRUCTURE`-based lazy body fetch (neomutt #786). Pool multiple connections per account so one slow request never blocks the UI.
- Aggressive **local cache + optional full Maildir/JMAP sync**, with a built-in **IMAP IDLE** watcher (attack #6) so new mail is instant — no external `goimapnotify`+`mbsync`+systemd-timer stack.
- A robust MIME parser that survives "a zillion ways to do MIME wrong" (HN tonyarkles), UTF-7 (aerc #340), malformed `From ` lines (aerc #323), and never panics on non-ASCII (himalaya #104).

### D. Search without the notmuch tax (attack #7)
- Bundle a built-in full-text indexer (Go: e.g. **Bleve** — HN literally suggested this for aerc: "could be easily done with blevesearch.com"). Avoid aerc's notmuch-backend correctness bugs (#160/#341/#106) by owning the index. Support notmuch-style tag queries *and* virtual folders/saved searches.
- Optionally still read a notmuch DB for immigrants.

### E. Multi-account & identity as a first-class citizen (attack #8/#15)
- **Unified inbox** across accounts (aerc #348), with per-account color/signature/send-hooks.
- **Per-recipient auto "From"** for plus-addresses (`me+amazon@` → reply from `me+amazon@`) — the Thunderbird feature xyst asked for in two separate threads.
- Built-in **CardDAV / macOS Contacts / vCard** autocomplete (no `lbdbq`/`goobook`); cache contacts locally.

### F. Encryption done correctly (attack #11)
- Native GPGME-style PGP: reliable decrypt (aerc #306), **sign+encrypt** semantics (alot #1207), **WKD** key discovery, **Autocrypt** (alot #1370), **Memory Hole** encrypted Subject (aerc #347), and "warn if not encrypted to all recipients" (alot #1232). Opportunistic encrypt that doesn't trip spam filters (SeniorMars's `crypt_opportunistic_encrypt` caveat).

### G. Compose/editor without the rough edges (attack #10/#2)
- Default **vim keybindings**, Escape-closes-prompt (neomutt #1510), with a clean embedded-editor passthrough that *doesn't* break Emacs/micro/Ctrl+Backspace/pinentry (aerc #343/#346/#350).
- Persistent drafts, signatures, spell-check, and grammar-check hooks.

### H. Attachments & URLs (attack #12/#13)
- Show attachment filenames inline, one-key save to a configurable dir (neomutt #1603), inline image preview, and handle weird filenames (`[`, spaces, unicode — aerc #345). Built-in URL picker.

### I. Architecture: composable core + TUI (attack #19, the philosophical critique)
- Expose a **stable CLI + JSON/line-oriented output** (himalaya's beloved property) *and* a polished TUI on top, so scripters and "one-app" users are both happy. Keep a Unix-friendly, *non-GPL* license (HN snthpy: "GPL which limits adoption") — MIT/Apache-2.0 broadens adoption and corporate use.

### J. Quality-of-life table stakes
- Themable style-sets (aerc's strength), mouse support that actually works (neomutt #309, aerc #329), proxy support (neomutt #99), **JMAP** support (himalaya HN — be first among Go TUIs), Gmail-label sync via the Gmail API (lieer-style) as an optional backend, and a snooze/inbox-zero workflow that GUI users expect.

### K. Don't be the "meditation space" casualty
- Some users "don't want email in their terminal" (HN endorphine). clibox can't fully solve this, but a **non-intrusive new-mail notification** (desktop notification + status-bar unread count) and a fast open/quit reduce the feeling of email colonizing the terminal.

### TL;DR positioning
clibox's wedge against the field: **"OAuth2 + O365 in 2 minutes, HTML that actually renders, Markdown compose, instant IDLE push, built-in search, and a JSON core for scripting — single binary, cross-platform, MIT-licensed."** No existing client delivers all of these; each gap above is a documented complaint against a specific competitor.
