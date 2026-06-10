package app

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/mail"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	imapclient "github.com/emersion/go-imap/client"
	gomessage "github.com/emersion/go-message"
	mailmessage "github.com/emersion/go-message/mail"
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

const (
	maxMessageImages    = 1
	maxInlineImageBytes = 2 << 20
)

type nativeBackend struct {
	account        string
	mailbox        string
	archiveFolder  string
	pageSize       int
	statePath      string
	editor         string
	configAccounts map[string]AccountConfig
}

func newNativeBackend(options Options) nativeBackend {
	return nativeBackend{
		account:        strings.TrimSpace(options.Account),
		mailbox:        firstNonEmpty(options.Mailbox, "INBOX"),
		archiveFolder:  strings.TrimSpace(options.ArchiveFolder),
		pageSize:       options.PageSize,
		statePath:      options.StatePath,
		editor:         strings.TrimSpace(options.Editor),
		configAccounts: options.Accounts,
	}
}

func (n nativeBackend) Label() string {
	return strings.Join(nonEmpty(n.account, n.mailbox, "native"), " ")
}

func (n nativeBackend) WithAccount(account string) inboxBackend {
	n.account = sanitizeAccountName(account, "")
	return n
}

func (n nativeBackend) WithMailbox(mailbox string) inboxBackend {
	n.mailbox = firstNonEmpty(mailbox, "INBOX")
	return n
}

func (n nativeBackend) accountHint() (accountSetup, bool) {
	store, err := openNativeStore(n.statePath)
	if err != nil {
		return accountSetup{}, false
	}
	defer store.close()
	if err := n.seedConfiguredAccounts(context.Background(), store); err != nil {
		return accountSetup{}, false
	}
	account, err := store.account(context.Background(), n.account)
	if err != nil {
		return accountSetup{}, false
	}
	return accountSetup{
		Account:     account.Name,
		Email:       account.Email,
		DisplayName: firstNonEmpty(account.DisplayName, displayNameFromEmail(account.Email)),
		Provider:    detectProvider(account.Email),
	}, true
}

func (n nativeBackend) SaveAccountSetup(setup accountSetup) error {
	provider := setup.Provider
	if provider.Name == "" {
		provider = detectProvider(setup.Email)
	}
	if providerNeedsOAuth(provider) {
		return fmt.Errorf("%s native setup uses browser OAuth. Run clibox auth login --account %s after setting CLIBOX_%s_CLIENT_ID", provider.Name, sanitizeAccountName(setup.Account, provider.Account), strings.ToUpper(provider.Account))
	}
	if !provider.canAutoConfigure() {
		return fmt.Errorf("%s needs manual IMAP/SMTP settings before native setup can run", provider.Name)
	}
	setup.Account = sanitizeAccountName(setup.Account, provider.Account)
	setup.Email = strings.TrimSpace(strings.ToLower(setup.Email))
	setup.Secret = provider.normalizeSecret(setup.Secret)
	if setup.Account == "" {
		return errors.New("missing account name")
	}
	if !validEmailAddress(setup.Email) {
		return errors.New("missing valid email address")
	}
	if setup.Secret == "" {
		return fmt.Errorf("missing %s", strings.ToLower(provider.secretLabel()))
	}

	store, err := openNativeStore(n.statePath)
	if err != nil {
		return err
	}
	defer store.close()

	account := nativeAccountFromProvider(setup.Account, setup.Email, setup.DisplayName, provider, "password")
	account.Mailbox = firstNonEmpty(n.mailbox, account.Mailbox)
	account.ArchiveFolder = firstNonEmpty(n.archiveFolder, account.ArchiveFolder)
	account.Editor = n.editor
	if err := store.saveAccount(context.Background(), account); err != nil {
		return err
	}
	if err := saveNativeSecret(account.Name, account.Email, storedSecretPassword, setup.Secret); err != nil {
		return err
	}
	return nil
}

func (n nativeBackend) SaveOAuthAccountSetup(ctx context.Context, setup accountSetup) error {
	provider := setup.Provider
	if provider.Name == "" {
		provider = detectProvider(setup.Email)
	}
	oauthProvider, ok := oauthProviderForEmail(setup.Email)
	if !ok || !providerNeedsOAuth(provider) {
		return fmt.Errorf("%s does not support native browser OAuth yet", firstNonEmpty(provider.Name, setup.Email))
	}
	accountName := sanitizeAccountName(setup.Account, oauthProvider.Key)
	options := Options{
		Account:       accountName,
		Mailbox:       firstNonEmpty(n.mailbox, "INBOX"),
		ArchiveFolder: n.archiveFolder,
		Editor:        n.editor,
		StatePath:     n.statePath,
	}
	if _, err := NativeAuthAdd(ctx, options, setup.Email); err != nil {
		return err
	}
	if _, err := NativeAuthLogin(ctx, options); err != nil {
		return err
	}
	return nil
}

func (n nativeBackend) Diagnose(ctx context.Context, verbose bool) (string, error) {
	store, err := openNativeStore(n.statePath)
	if err != nil {
		return "", err
	}
	defer store.close()
	if err := n.seedConfiguredAccounts(ctx, store); err != nil {
		return "", err
	}

	lines := []string{
		"Native mail backend",
		"State: " + store.path,
	}
	if bad, columns, err := store.schemaHasCredentialColumns(ctx); err != nil {
		lines = append(lines, "Credential schema check: could not inspect cache schema: "+oneLine(err.Error()))
	} else if bad {
		lines = append(lines, "Credential schema check: failed; credential-like columns found: "+strings.Join(columns, ", "))
	} else {
		lines = append(lines, "Credential schema check: OK; cache has no password/token columns")
	}

	accounts, err := store.listAccounts(ctx)
	if err != nil {
		return strings.Join(lines, "\n"), err
	}
	if len(accounts) == 0 {
		lines = append(lines, "Accounts: none")
		lines = append(lines, "Next step: run clibox auth login --account gmail, or open clibox and add a non-OAuth provider with an app password.")
		return strings.Join(lines, "\n"), nil
	}
	lines = append(lines, fmt.Sprintf("Accounts: %d", len(accounts)))
	if verbose {
		for _, account := range accounts {
			credential := "password"
			if strings.EqualFold(account.AuthType, "oauth2") {
				// #nosec G101 -- this is a human-readable credential type label, not a token value.
				credential = "OAuth refresh token"
			}
			lines = append(lines, fmt.Sprintf("- %s <%s> provider=%s auth=%s credential=%s", account.Name, account.Email, account.Provider, account.AuthType, credential))
		}
	}

	account, err := store.account(ctx, n.account)
	if err != nil {
		return strings.Join(lines, "\n"), err
	}
	lines = append(lines, "Selected account: "+account.Name)
	lines = append(lines, "Mailbox: "+firstNonEmpty(n.mailbox, account.Mailbox, "INBOX"))
	messages, done, err := n.ListEnvelopePage(ctx, 1)
	if err != nil {
		lines = append(lines, "Connection: failed - "+oneLine(err.Error()))
		return strings.Join(lines, "\n"), nil
	}
	older := ""
	if !done {
		older = " (older mail available)"
	}
	lines = append(lines, fmt.Sprintf("Connection: OK; first page has %d emails%s", len(messages), older))
	return strings.Join(lines, "\n"), nil
}

func nativeAccountFromProvider(name, email, displayName string, provider providerInfo, authType string) nativeAccount {
	folders := mergeFolders(provider.Folders)
	return nativeAccount{
		Name:          sanitizeAccountName(name, provider.Account),
		Provider:      strings.ToLower(firstNonEmpty(provider.Account, accountNameFromDomain(emailDomain(email)))),
		Email:         strings.TrimSpace(strings.ToLower(email)),
		DisplayName:   firstNonEmpty(displayName, displayNameFromEmail(email)),
		AuthType:      firstNonEmpty(authType, "password"),
		IMAPHost:      provider.IMAPHost,
		IMAPPort:      provider.IMAPPort,
		IMAPSecurity:  provider.IMAPSecurity,
		SMTPHost:      provider.SMTPHost,
		SMTPPort:      provider.SMTPPort,
		SMTPSecurity:  provider.SMTPSecurity,
		Mailbox:       firstNonEmpty(folders["inbox"], "INBOX"),
		ArchiveFolder: firstNonEmpty(folders["archive"], "Archive"),
		TrashFolder:   firstNonEmpty(folders["trash"], "Trash"),
		SentFolder:    firstNonEmpty(folders["sent"], "Sent"),
		DraftsFolder:  firstNonEmpty(folders["drafts"], "Drafts"),
		SyncPolicy:    "headers",
	}
}

func (n nativeBackend) ListEnvelopes(ctx context.Context) ([]message, error) {
	var out []message
	for page := 1; ; page++ {
		messages, done, err := n.ListEnvelopePage(ctx, page)
		if err != nil {
			return nil, err
		}
		out = append(out, messages...)
		if done {
			return out, nil
		}
	}
}

func (n nativeBackend) ListEnvelopePage(ctx context.Context, page int) ([]message, bool, error) {
	return n.searchOrListEnvelopePage(ctx, page, "")
}

func (n nativeBackend) SearchEnvelopePage(ctx context.Context, page int, query string) ([]message, bool, error) {
	return n.searchOrListEnvelopePage(ctx, page, query)
}

func (n nativeBackend) searchOrListEnvelopePage(ctx context.Context, page int, query string) ([]message, bool, error) {
	store, account, mailbox, err := n.openStoreAndAccount(ctx)
	if err != nil {
		return nil, false, err
	}
	defer store.close()

	remoteDone, err := n.syncEnvelopePage(ctx, store, account, mailbox, page, query)
	if err != nil {
		cached, done, cacheErr := store.cachedEnvelopePage(ctx, account.Name, mailbox, page, n.effectivePageSize(), query)
		if cacheErr == nil && len(cached) > 0 {
			return cached, done, nil
		}
		return nil, false, err
	}
	messages, _, err := store.cachedEnvelopePage(ctx, account.Name, mailbox, page, n.effectivePageSize(), query)
	return messages, remoteDone, err
}

func (n nativeBackend) ReadMessage(ctx context.Context, msg message) (string, error) {
	content, err := n.ReadMessageContent(ctx, msg)
	if err != nil {
		return "", err
	}
	return content.Body, nil
}

func (n nativeBackend) ReadMessageContent(ctx context.Context, msg message) (messageContent, error) {
	store, account, mailbox, err := n.openStoreAndAccount(ctx)
	if err != nil {
		return messageContent{}, err
	}
	defer store.close()

	uid, err := messageIDUint(msg)
	if err != nil {
		return messageContent{}, err
	}

	client, err := n.connectIMAP(ctx, account)
	if err != nil {
		if body, ok, cacheErr := store.body(ctx, account.Name, mailbox, msg.ID); cacheErr == nil && ok {
			return messageContent{Body: body}, nil
		}
		return messageContent{}, err
	}
	defer client.Logout()
	if _, err := client.Select(mailbox, true); err != nil {
		return messageContent{}, fmt.Errorf("could not open %s: %w", mailbox, err)
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(uid)
	section, _ := imap.ParseBodySectionName("BODY.PEEK[]")
	messages := make(chan *imap.Message, 1)
	done := make(chan error, 1)
	go func() {
		done <- client.UidFetch(seqset, []imap.FetchItem{section.FetchItem()}, messages)
	}()
	var raw []byte
	for fetched := range messages {
		literal := fetched.GetBody(section)
		if literal == nil {
			continue
		}
		raw, err = io.ReadAll(io.LimitReader(literal, 20<<20))
		if err != nil {
			return messageContent{}, err
		}
	}
	if err := <-done; err != nil {
		return messageContent{}, fmt.Errorf("could not read email body: %w", err)
	}
	content := extractReadableMessageContent(raw)
	if strings.TrimSpace(content.Body) == "" {
		content.Body = "(empty message)"
	}
	_ = store.saveBody(ctx, account.Name, mailbox, msg.ID, content.Body)
	return content, nil
}

func (n nativeBackend) ArchiveMessage(ctx context.Context, msg message) error {
	store, account, mailbox, err := n.openStoreAndAccount(ctx)
	if err != nil {
		return err
	}
	defer store.close()
	return n.moveMessage(ctx, account, mailbox, msg, firstNonEmpty(n.archiveFolder, account.ArchiveFolder, "Archive"))
}

func (n nativeBackend) DeleteMessage(ctx context.Context, msg message) error {
	store, account, mailbox, err := n.openStoreAndAccount(ctx)
	if err != nil {
		return err
	}
	defer store.close()
	return n.moveMessage(ctx, account, mailbox, msg, firstNonEmpty(account.TrashFolder, "Trash"))
}

func (n nativeBackend) PrepareDraft(_ context.Context, req draftRequest) (string, error) {
	store, account, _, err := n.openStoreAndAccount(context.Background())
	if err != nil {
		return "", err
	}
	defer store.close()
	from := account.Email
	if strings.TrimSpace(account.DisplayName) != "" {
		from = fmt.Sprintf("%s <%s>", account.DisplayName, account.Email)
	}
	if req.Kind == replyDraft {
		return localReplyTemplate(req.Message, from), nil
	}
	return localComposeTemplate(from), nil
}

func (n nativeBackend) SendDraft(ctx context.Context, content string) error {
	store, account, _, err := n.openStoreAndAccount(ctx)
	if err != nil {
		return err
	}
	defer store.close()
	content = normalizeDraftContent(content)
	summary := parseDraftSummary(content)
	if err := validateDraftForSend(summary); err != nil {
		return err
	}
	from := firstAddress(summary.From)
	if from == "" {
		from = account.Email
	}
	recipients := draftRecipients(summary)
	if len(recipients) == 0 {
		return errors.New("add at least one recipient before sending")
	}
	auth, err := n.smtpAuth(ctx, account)
	if err != nil {
		return err
	}
	addr := net.JoinHostPort(account.SMTPHost, fmt.Sprint(account.SMTPPort))
	reader := strings.NewReader(smtpDraftContent(content))
	if strings.EqualFold(account.SMTPSecurity, "tls") || account.SMTPPort == 465 {
		return smtp.SendMailTLS(addr, auth, from, recipients, reader)
	}
	return smtp.SendMail(addr, auth, from, recipients, reader)
}

func (n nativeBackend) moveMessage(ctx context.Context, account nativeAccount, mailbox string, msg message, dest string) error {
	uid, err := messageIDUint(msg)
	if err != nil {
		return err
	}
	client, err := n.connectIMAP(ctx, account)
	if err != nil {
		return err
	}
	defer client.Logout()
	if _, err := client.Select(mailbox, false); err != nil {
		return fmt.Errorf("could not open %s: %w", mailbox, err)
	}
	seqset := new(imap.SeqSet)
	seqset.AddNum(uid)
	if strings.TrimSpace(dest) != "" {
		if err := client.UidMove(seqset, dest); err == nil {
			return nil
		}
		if err := client.UidCopy(seqset, dest); err != nil {
			return fmt.Errorf("could not move email to %s: %w", dest, err)
		}
	}
	item := imap.FormatFlagsOp(imap.AddFlags, true)
	if err := client.UidStore(seqset, item, []interface{}{"\\Deleted"}, nil); err != nil {
		return fmt.Errorf("could not mark email deleted: %w", err)
	}
	return client.Expunge(nil)
}

func (n nativeBackend) syncEnvelopePage(ctx context.Context, store *nativeStore, account nativeAccount, mailbox string, page int, query string) (bool, error) {
	client, err := n.connectIMAP(ctx, account)
	if err != nil {
		return false, err
	}
	defer client.Logout()

	status, err := client.Select(mailbox, true)
	if err != nil {
		return false, fmt.Errorf("could not open %s: %w", mailbox, err)
	}
	_ = store.saveMailboxSync(ctx, account.Name, mailbox, status.UidValidity, status.UidNext)
	if status.Messages == 0 {
		return true, nil
	}

	seqset := new(imap.SeqSet)
	byUID := false
	remoteDone := false
	pageSize := uint32(n.effectivePageSize()) // #nosec G115 -- effectivePageSize is bounded to a small positive request size.
	if strings.TrimSpace(query) != "" {
		criteria := imap.NewSearchCriteria()
		criteria.Text = searchTerms(query)
		uids, err := client.UidSearch(criteria)
		if err != nil {
			return false, fmt.Errorf("could not search mailbox: %w", err)
		}
		sort.Slice(uids, func(i, j int) bool { return uids[i] > uids[j] })
		start, end, done := nativePageWindow(len(uids), page, int(pageSize))
		if start == end {
			return true, nil
		}
		remoteDone = done
		seqset.AddNum(uids[start:end]...)
		byUID = true
	} else {
		from, to, done, ok := nativeEnvelopeSeqRange(status.Messages, page, pageSize)
		if !ok {
			return true, nil
		}
		remoteDone = done
		seqset.AddRange(from, to)
	}

	messages, err := fetchNativeEnvelopes(client, seqset, byUID)
	if err != nil {
		return false, err
	}
	return remoteDone, store.upsertEnvelopes(ctx, account.Name, mailbox, messages)
}

func nativePageWindow(total, page, pageSize int) (int, int, bool) {
	if pageSize <= 0 {
		pageSize = 50
	}
	if total <= 0 {
		return 0, 0, true
	}
	start := (max(1, page) - 1) * pageSize
	if start >= total {
		return 0, 0, true
	}
	end := min(total, start+pageSize)
	return start, end, end >= total
}

func nativeEnvelopeSeqRange(total uint32, page int, pageSize uint32) (uint32, uint32, bool, bool) {
	if pageSize == 0 {
		pageSize = 50
	}
	if total == 0 {
		return 0, 0, true, false
	}
	pageIndex := max(0, page-1)
	offset := uint64(pageIndex) * uint64(pageSize) // #nosec G115 -- pageIndex is clamped non-negative and compared against the uint32 message count below.
	if offset >= uint64(total) {
		return 0, 0, true, false
	}
	to := total - uint32(offset) // #nosec G115 -- offset is checked to be less than the uint32 message count.
	from := uint32(1)
	if to > pageSize {
		from = to - pageSize + 1
	}
	done := offset+uint64(pageSize) >= uint64(total)
	return from, to, done, true
}

func fetchNativeEnvelopes(client *imapclient.Client, seqset *imap.SeqSet, byUID bool) ([]message, error) {
	items := []imap.FetchItem{imap.FetchEnvelope, imap.FetchFlags, imap.FetchUid}
	ch := make(chan *imap.Message, 50)
	done := make(chan error, 1)
	go func() {
		if byUID {
			done <- client.UidFetch(seqset, items, ch)
			return
		}
		done <- client.Fetch(seqset, items, ch)
	}()

	var messages []message
	for fetched := range ch {
		messages = append(messages, nativeEnvelopeMessage(fetched))
	}
	if err := <-done; err != nil {
		return nil, fmt.Errorf("could not fetch mailbox envelopes: %w", err)
	}
	sort.Slice(messages, func(i, j int) bool {
		left, _ := messageIDUint(messages[i])
		right, _ := messageIDUint(messages[j])
		return left > right
	})
	return messages, nil
}

func nativeEnvelopeMessage(fetched *imap.Message) message {
	msg := message{ID: fmt.Sprint(fetched.Uid), Subject: "(no subject)"}
	if fetched.Envelope != nil {
		msg.Subject = firstNonEmpty(fetched.Envelope.Subject, "(no subject)")
		msg.Date = formatNativeDate(fetched.Envelope.Date)
		msg.From, msg.Email = nativeAddress(fetched.Envelope.From)
		if msg.Email == "" {
			msg.From, msg.Email = nativeAddress(fetched.Envelope.Sender)
		}
	}
	msg.From = firstNonEmpty(msg.From, msg.Email, "Unknown")
	msg.Preview = "Flags: " + strings.Join(fetched.Flags, ", ")
	msg.Unread = true
	for _, flag := range fetched.Flags {
		if strings.EqualFold(strings.TrimLeft(flag, "\\"), "Seen") {
			msg.Unread = false
			break
		}
	}
	return msg
}

func nativeAddress(addresses []*imap.Address) (string, string) {
	for _, addr := range addresses {
		if addr == nil {
			continue
		}
		email := strings.TrimSpace(addr.MailboxName + "@" + addr.HostName)
		if validEmailAddress(email) {
			return firstNonEmpty(addr.PersonalName, email), email
		}
	}
	return "", ""
}

func formatNativeDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return t.Format("3:04 PM")
	}
	if t.Year() == now.Year() {
		return t.Format("Jan 2")
	}
	return t.Format("Jan 2 2006")
}

func (n nativeBackend) openStoreAndAccount(ctx context.Context) (*nativeStore, nativeAccount, string, error) {
	store, err := openNativeStore(n.statePath)
	if err != nil {
		return nil, nativeAccount{}, "", err
	}
	if err := n.seedConfiguredAccounts(ctx, store); err != nil {
		_ = store.close()
		return nil, nativeAccount{}, "", err
	}
	account, err := store.account(ctx, n.account)
	if err != nil {
		_ = store.close()
		return nil, nativeAccount{}, "", err
	}
	mailbox := firstNonEmpty(n.mailbox, account.Mailbox, "INBOX")
	return store, account, mailbox, nil
}

func (n nativeBackend) seedConfiguredAccounts(ctx context.Context, store *nativeStore) error {
	for name, entry := range n.configAccounts {
		email := strings.TrimSpace(strings.ToLower(entry.Email))
		if !validEmailAddress(email) {
			continue
		}
		provider := detectProvider(email)
		if !provider.canAutoConfigure() {
			continue
		}
		authType := "password"
		var scopes []string
		var clientID string
		if oauthProvider, ok := oauthProviderForEmail(email); ok {
			authType = "oauth2"
			scopes = oauthProvider.Scopes
			clientID = oauthClientID(oauthProvider)
		}
		account := nativeAccountFromProvider(firstNonEmpty(entry.Name, name), email, displayNameFromEmail(email), provider, authType)
		if clientID == "" {
			if existing, err := store.account(ctx, account.Name); err == nil && strings.TrimSpace(existing.OAuthClientID) != "" {
				clientID = existing.OAuthClientID
			}
		}
		account.Provider = firstNonEmpty(entry.Provider, account.Provider)
		account.Mailbox = firstNonEmpty(entry.Mailbox, account.Mailbox)
		account.ArchiveFolder = firstNonEmpty(entry.ArchiveFolder, account.ArchiveFolder)
		account.SyncPolicy = firstNonEmpty(entry.SyncPolicy, account.SyncPolicy)
		account.Editor = firstNonEmpty(entry.Editor, n.editor)
		account.OAuthScopes = scopes
		account.OAuthClientID = clientID
		if err := store.saveAccount(ctx, account); err != nil {
			return err
		}
	}
	return nil
}

func (n nativeBackend) connectIMAP(ctx context.Context, account nativeAccount) (*imapclient.Client, error) {
	addr := net.JoinHostPort(account.IMAPHost, fmt.Sprint(account.IMAPPort))
	dialer := &net.Dialer{Timeout: 20 * time.Second}
	tlsConfig := &tls.Config{ServerName: account.IMAPHost, MinVersion: tls.VersionTLS12}
	var client *imapclient.Client
	var err error
	if strings.EqualFold(account.IMAPSecurity, "tls") || account.IMAPPort == 993 {
		client, err = imapclient.DialWithDialerTLS(dialer, addr, tlsConfig)
	} else {
		client, err = imapclient.DialWithDialer(dialer, addr)
		if err == nil && strings.EqualFold(account.IMAPSecurity, "start-tls") {
			err = client.StartTLS(tlsConfig)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("could not connect to IMAP server: %w", err)
	}
	if err := n.authenticateIMAP(ctx, client, account); err != nil {
		_ = client.Logout()
		return nil, err
	}
	return client, nil
}

func (n nativeBackend) authenticateIMAP(ctx context.Context, client *imapclient.Client, account nativeAccount) error {
	switch strings.ToLower(account.AuthType) {
	case "oauth2":
		auth, err := n.oauthSASL(ctx, account)
		if err != nil {
			return err
		}
		if err := client.Authenticate(auth); err != nil {
			return fmt.Errorf("OAuth IMAP authentication failed: %w", err)
		}
	default:
		password, err := loadNativeSecret(account.Name, account.Email, storedSecretPassword)
		if err != nil {
			return err
		}
		if err := client.Login(account.Email, password); err != nil {
			return fmt.Errorf("IMAP authentication failed: %w", err)
		}
	}
	return nil
}

func (n nativeBackend) smtpAuth(ctx context.Context, account nativeAccount) (sasl.Client, error) {
	switch strings.ToLower(account.AuthType) {
	case "oauth2":
		return n.oauthSASL(ctx, account)
	default:
		password, err := loadNativeSecret(account.Name, account.Email, storedSecretPassword)
		if err != nil {
			return nil, err
		}
		return sasl.NewPlainClient("", account.Email, password), nil
	}
}

func (n nativeBackend) oauthSASL(ctx context.Context, account nativeAccount) (sasl.Client, error) {
	refreshToken, err := loadNativeSecret(account.Name, account.Email, storedSecretRefreshToken)
	if err != nil {
		return nil, err
	}
	provider, ok := oauthProviderForEmail(account.Email)
	if !ok {
		return nil, fmt.Errorf("%s does not have a native OAuth provider yet", account.Email)
	}
	token, err := refreshOAuthAccessToken(ctx, provider, account.OAuthClientID, refreshToken)
	if err != nil {
		return nil, err
	}
	return xoauth2Client{username: account.Email, accessToken: token.AccessToken}, nil
}

func (n nativeBackend) effectivePageSize() int {
	switch {
	case n.pageSize <= 0:
		return 50
	case n.pageSize > 1000:
		return 1000
	default:
		return n.pageSize
	}
}

type xoauth2Client struct {
	username    string
	accessToken string
}

func (c xoauth2Client) Start() (string, []byte, error) {
	if strings.TrimSpace(c.username) == "" || strings.TrimSpace(c.accessToken) == "" {
		return "", nil, errors.New("XOAUTH2 requires username and access token")
	}
	resp := "user=" + c.username + "\x01auth=Bearer " + c.accessToken + "\x01\x01"
	return "XOAUTH2", []byte(resp), nil
}

func (c xoauth2Client) Next(_ []byte) ([]byte, error) {
	return nil, nil
}

func extractReadableMessageBody(raw []byte) string {
	return extractReadableMessageContent(raw).Body
}

func extractReadableMessageContent(raw []byte) messageContent {
	if len(bytes.TrimSpace(raw)) == 0 {
		return messageContent{}
	}
	reader, err := mailmessage.CreateReader(bytes.NewReader(raw))
	if err != nil {
		return messageContent{Body: normalizeMessageBody(raw)}
	}
	var plainParts []string
	var htmlParts []string
	var images []messageImage
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil && !gomessage.IsUnknownCharset(err) {
			break
		}
		if part == nil || part.Body == nil {
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(part.Body, 4<<20))
		if readErr != nil {
			continue
		}
		mediaType, name := partContentInfo(part.Header)
		switch strings.ToLower(mediaType) {
		case "text/plain", "":
			plainParts = append(plainParts, normalizeMessageBody(body))
		case "text/html":
			htmlBody := string(body)
			htmlParts = append(htmlParts, htmlToText(htmlBody))
		default:
			if strings.HasPrefix(strings.ToLower(mediaType), "image/") {
				images = appendMessageImage(images, messageImage{
					Name:        firstNonEmpty(name, "inline image"),
					ContentType: mediaType,
					Data:        body,
				})
			}
		}
	}
	if text := strings.TrimSpace(strings.Join(nonEmpty(plainParts...), "\n\n")); text != "" {
		return messageContent{Body: text, Images: images}
	}
	if text := strings.TrimSpace(strings.Join(nonEmpty(htmlParts...), "\n\n")); text != "" {
		return messageContent{Body: text, Images: images}
	}
	return messageContent{Body: normalizeMessageBody(raw), Images: images}
}

func partContentInfo(header any) (string, string) {
	switch header := header.(type) {
	case *mailmessage.InlineHeader:
		mediaType, params, _ := header.ContentType()
		_, dispositionParams, _ := header.ContentDisposition()
		return strings.ToLower(mediaType), firstNonEmpty(dispositionParams["filename"], params["name"], headerText(header.Header, "Content-Description"))
	case *mailmessage.AttachmentHeader:
		mediaType, params, _ := header.ContentType()
		filename, _ := header.Filename()
		return strings.ToLower(mediaType), firstNonEmpty(filename, params["name"], headerText(header.Header, "Content-Description"))
	}
	return "", ""
}

func headerText(header gomessage.Header, key string) string {
	value, err := header.Text(key)
	if err != nil {
		return ""
	}
	return value
}

func appendMessageImage(images []messageImage, image messageImage) []messageImage {
	if len(images) >= maxMessageImages || len(image.Data) == 0 || len(image.Data) > maxInlineImageBytes {
		return images
	}
	image.Name = terminalSafeLine(firstNonEmpty(image.Name, "image"))
	image.ContentType = strings.ToLower(firstNonEmpty(image.ContentType, "image"))
	return append(images, image)
}

func htmlToText(input string) string {
	input = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`).ReplaceAllString(input, "")
	input = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`).ReplaceAllString(input, "")
	input = regexp.MustCompile(`(?i)<br\s*/?>`).ReplaceAllString(input, "\n")
	input = regexp.MustCompile(`(?i)</p\s*>`).ReplaceAllString(input, "\n\n")
	input = regexp.MustCompile(`(?s)<[^>]+>`).ReplaceAllString(input, " ")
	input = html.UnescapeString(input)
	return strings.Join(strings.Fields(input), " ")
}

func firstAddress(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed, err := mail.ParseAddress(value)
	if err == nil {
		return parsed.Address
	}
	_, email := parseAddressString(value)
	return email
}

func draftRecipients(summary draftSummary) []string {
	var out []string
	for _, raw := range []string{summary.To, summary.Cc, summary.Bcc} {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		addresses, err := mail.ParseAddressList(raw)
		if err != nil {
			_, email := parseAddressString(raw)
			if email != "" {
				out = append(out, email)
			}
			continue
		}
		for _, addr := range addresses {
			if addr != nil && validEmailAddress(addr.Address) {
				out = append(out, addr.Address)
			}
		}
	}
	return out
}
