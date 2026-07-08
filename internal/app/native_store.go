package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type nativeAccount struct {
	Name          string
	Provider      string
	Email         string
	DisplayName   string
	AuthType      string
	IMAPHost      string
	IMAPPort      int
	IMAPSecurity  string
	SMTPHost      string
	SMTPPort      int
	SMTPSecurity  string
	Mailbox       string
	ArchiveFolder string
	TrashFolder   string
	SentFolder    string
	DraftsFolder  string
	SyncPolicy    string
	Editor        string
	OAuthClientID string
	OAuthScopes   []string
}

type nativeStore struct {
	path string
	db   *sql.DB
}

func openNativeStore(path string) (*nativeStore, error) {
	resolved, err := cliboxStatePath(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(resolved), 0o700); err != nil {
		return nil, fmt.Errorf("could not create clibox state directory: %w", err)
	}
	if err := secureNativeStoreFile(resolved); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", resolved)
	if err != nil {
		return nil, fmt.Errorf("could not open clibox state database: %w", err)
	}
	db.SetMaxOpenConns(1)
	store := &nativeStore{path: resolved, db: db}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func secureNativeStoreFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("could not inspect clibox state database: %w", err)
	}
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("clibox state database must not be a symlink: %s", path)
	}
	parent, err := os.Stat(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("could not inspect clibox state directory: %w", err)
	}
	if parent.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("clibox state directory must not be group- or world-writable: %s", filepath.Dir(path))
	}

	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600) // #nosec G304 -- state path is explicit user configuration after symlink and shared-directory checks.
	if err != nil {
		return fmt.Errorf("could not create clibox state database: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		closeErr := file.Close()
		return errors.Join(fmt.Errorf("could not secure clibox state database: %w", err), closeErr)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("could not close clibox state database: %w", err)
	}
	return nil
}

func (s *nativeStore) close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *nativeStore) migrate(ctx context.Context) error {
	statements := []string{
		`PRAGMA foreign_keys = ON`,
		`CREATE TABLE IF NOT EXISTS native_accounts (
			name TEXT PRIMARY KEY,
			provider TEXT NOT NULL,
			email TEXT NOT NULL,
			display_name TEXT NOT NULL DEFAULT '',
			auth_type TEXT NOT NULL,
			imap_host TEXT NOT NULL,
			imap_port INTEGER NOT NULL,
			imap_security TEXT NOT NULL,
			smtp_host TEXT NOT NULL,
			smtp_port INTEGER NOT NULL,
			smtp_security TEXT NOT NULL,
			mailbox TEXT NOT NULL,
			archive_folder TEXT NOT NULL,
			trash_folder TEXT NOT NULL,
			sent_folder TEXT NOT NULL,
			drafts_folder TEXT NOT NULL,
			sync_policy TEXT NOT NULL DEFAULT 'headers',
			editor TEXT NOT NULL DEFAULT '',
			oauth_client_id TEXT NOT NULL DEFAULT '',
			oauth_scopes TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS mailboxes (
			account TEXT NOT NULL,
			name TEXT NOT NULL,
			uid_validity INTEGER NOT NULL DEFAULT 0,
			uid_next INTEGER NOT NULL DEFAULT 0,
			last_synced_at TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (account, name),
			FOREIGN KEY (account) REFERENCES native_accounts(name) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS envelopes (
			account TEXT NOT NULL,
			mailbox TEXT NOT NULL,
			uid TEXT NOT NULL,
			message_id TEXT NOT NULL DEFAULT '',
			sender_name TEXT NOT NULL DEFAULT '',
			sender_email TEXT NOT NULL DEFAULT '',
			subject TEXT NOT NULL DEFAULT '',
			sent_at TEXT NOT NULL DEFAULT '',
			preview TEXT NOT NULL DEFAULT '',
			flags TEXT NOT NULL DEFAULT '',
			unread INTEGER NOT NULL DEFAULT 0,
			search_text TEXT NOT NULL DEFAULT '',
			body_cached INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (account, mailbox, uid),
			FOREIGN KEY (account, mailbox) REFERENCES mailboxes(account, name) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS envelopes_account_mailbox_updated ON envelopes(account, mailbox, CAST(uid AS INTEGER) DESC)`,
		`CREATE INDEX IF NOT EXISTS envelopes_search ON envelopes(account, mailbox, search_text)`,
		`CREATE TABLE IF NOT EXISTS message_bodies (
			account TEXT NOT NULL,
			mailbox TEXT NOT NULL,
			uid TEXT NOT NULL,
			body TEXT NOT NULL,
			content_type TEXT NOT NULL DEFAULT 'text/plain',
			cached_at TEXT NOT NULL,
			PRIMARY KEY (account, mailbox, uid),
			FOREIGN KEY (account, mailbox, uid) REFERENCES envelopes(account, mailbox, uid) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS sync_cursors (
			account TEXT NOT NULL,
			mailbox TEXT NOT NULL,
			highest_uid TEXT NOT NULL DEFAULT '',
			oldest_uid TEXT NOT NULL DEFAULT '',
			last_synced_at TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (account, mailbox),
			FOREIGN KEY (account, mailbox) REFERENCES mailboxes(account, name) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS app_state (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
	}
	for _, stmt := range statements {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("could not migrate native mail cache: %w", err)
		}
	}
	return nil
}

func (s *nativeStore) saveAccount(ctx context.Context, account nativeAccount) error {
	account.Name = sanitizeAccountName(account.Name, "")
	account.Email = strings.TrimSpace(strings.ToLower(account.Email))
	account.Mailbox = firstNonEmpty(account.Mailbox, "INBOX")
	account.SyncPolicy = firstNonEmpty(account.SyncPolicy, "headers")
	if account.Name == "" {
		return errors.New("missing account name")
	}
	if !validEmailAddress(account.Email) {
		return errors.New("missing valid account email")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `INSERT INTO native_accounts (
		name, provider, email, display_name, auth_type, imap_host, imap_port, imap_security,
		smtp_host, smtp_port, smtp_security, mailbox, archive_folder, trash_folder, sent_folder,
		drafts_folder, sync_policy, editor, oauth_client_id, oauth_scopes, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(name) DO UPDATE SET
		provider = excluded.provider,
		email = excluded.email,
		display_name = excluded.display_name,
		auth_type = excluded.auth_type,
		imap_host = excluded.imap_host,
		imap_port = excluded.imap_port,
		imap_security = excluded.imap_security,
		smtp_host = excluded.smtp_host,
		smtp_port = excluded.smtp_port,
		smtp_security = excluded.smtp_security,
		mailbox = excluded.mailbox,
		archive_folder = excluded.archive_folder,
		trash_folder = excluded.trash_folder,
		sent_folder = excluded.sent_folder,
		drafts_folder = excluded.drafts_folder,
		sync_policy = excluded.sync_policy,
		editor = excluded.editor,
		oauth_client_id = excluded.oauth_client_id,
		oauth_scopes = excluded.oauth_scopes,
		updated_at = excluded.updated_at`,
		account.Name, account.Provider, account.Email, account.DisplayName, account.AuthType,
		account.IMAPHost, account.IMAPPort, account.IMAPSecurity, account.SMTPHost, account.SMTPPort,
		account.SMTPSecurity, account.Mailbox, account.ArchiveFolder, account.TrashFolder,
		account.SentFolder, account.DraftsFolder, account.SyncPolicy, account.Editor,
		account.OAuthClientID, strings.Join(account.OAuthScopes, " "), now, now,
	)
	if err != nil {
		return fmt.Errorf("could not save native account metadata: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO mailboxes (account, name) VALUES (?, ?)
		ON CONFLICT(account, name) DO NOTHING`, account.Name, account.Mailbox)
	if err != nil {
		return fmt.Errorf("could not save native mailbox metadata: %w", err)
	}
	return nil
}

func (s *nativeStore) account(ctx context.Context, name string) (nativeAccount, error) {
	name = sanitizeAccountName(name, "")
	query := `SELECT name, provider, email, display_name, auth_type, imap_host, imap_port, imap_security,
		smtp_host, smtp_port, smtp_security, mailbox, archive_folder, trash_folder, sent_folder,
		drafts_folder, sync_policy, editor, oauth_client_id, oauth_scopes
		FROM native_accounts`
	var row *sql.Row
	if name != "" {
		row = s.db.QueryRowContext(ctx, query+` WHERE name = ?`, name)
	} else {
		row = s.db.QueryRowContext(ctx, query+` ORDER BY updated_at DESC, name LIMIT 1`)
	}
	account, err := scanNativeAccount(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nativeAccount{}, setupRequiredError{detail: "no native mail account configured"}
	}
	return account, err
}

func (s *nativeStore) listAccounts(ctx context.Context) ([]nativeAccount, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name, provider, email, display_name, auth_type, imap_host, imap_port, imap_security,
		smtp_host, smtp_port, smtp_security, mailbox, archive_folder, trash_folder, sent_folder,
		drafts_folder, sync_policy, editor, oauth_client_id, oauth_scopes
		FROM native_accounts ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("could not list native accounts: %w", err)
	}
	defer rows.Close()

	var accounts []nativeAccount
	for rows.Next() {
		account, err := scanNativeAccount(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

type nativeAccountScanner interface {
	Scan(dest ...any) error
}

func scanNativeAccount(scanner nativeAccountScanner) (nativeAccount, error) {
	var account nativeAccount
	var scopes string
	err := scanner.Scan(
		&account.Name, &account.Provider, &account.Email, &account.DisplayName, &account.AuthType,
		&account.IMAPHost, &account.IMAPPort, &account.IMAPSecurity, &account.SMTPHost, &account.SMTPPort,
		&account.SMTPSecurity, &account.Mailbox, &account.ArchiveFolder, &account.TrashFolder,
		&account.SentFolder, &account.DraftsFolder, &account.SyncPolicy, &account.Editor,
		&account.OAuthClientID, &scopes,
	)
	if err != nil {
		return nativeAccount{}, err
	}
	account.OAuthScopes = strings.Fields(scopes)
	return account, nil
}

func (s *nativeStore) saveMailboxSync(ctx context.Context, account, mailbox string, uidValidity, uidNext uint32) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `INSERT INTO mailboxes (account, name, uid_validity, uid_next, last_synced_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(account, name) DO UPDATE SET
			uid_validity = excluded.uid_validity,
			uid_next = excluded.uid_next,
			last_synced_at = excluded.last_synced_at`,
		account, mailbox, uidValidity, uidNext, now)
	return err
}

func (s *nativeStore) upsertEnvelopes(ctx context.Context, account, mailbox string, messages []message) error {
	if len(messages) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO envelopes (
		account, mailbox, uid, message_id, sender_name, sender_email, subject, sent_at, preview,
		flags, unread, search_text, body_cached, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(account, mailbox, uid) DO UPDATE SET
		message_id = excluded.message_id,
		sender_name = excluded.sender_name,
		sender_email = excluded.sender_email,
		subject = excluded.subject,
		sent_at = excluded.sent_at,
		preview = excluded.preview,
		flags = excluded.flags,
		unread = excluded.unread,
		search_text = excluded.search_text,
		updated_at = excluded.updated_at`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	for _, msg := range messages {
		uid := strings.TrimSpace(msg.ID)
		if uid == "" {
			continue
		}
		var flagList []string
		unread := 0
		if msg.Unread {
			unread = 1
			flagList = append(flagList, "unread")
		}
		if msg.Flagged {
			flagList = append(flagList, "flagged")
		}
		flags := strings.Join(flagList, ",")
		searchText := strings.ToLower(strings.Join([]string{msg.From, msg.Email, msg.Subject, msg.Preview, msg.Date}, " "))
		if _, err := stmt.ExecContext(ctx, account, mailbox, uid, msg.MessageID, msg.From, msg.Email, msg.Subject, msg.Date, msg.Preview, flags, unread, searchText, 0, now); err != nil {
			return fmt.Errorf("could not save native envelope %s: %w", uid, err)
		}
	}
	return tx.Commit()
}

func (s *nativeStore) cachedEnvelopePage(ctx context.Context, account, mailbox string, page, pageSize int, query string) ([]message, bool, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}

	where := []string{"account = ?", "mailbox = ?"}
	args := []any{account, mailbox}
	for _, term := range searchTerms(query) {
		where = append(where, "search_text LIKE ?")
		args = append(args, "%"+strings.ToLower(term)+"%")
	}
	args = append(args, pageSize+1, (page-1)*pageSize)

	rows, err := s.db.QueryContext(ctx, `SELECT uid, message_id, sender_name, sender_email, subject, sent_at, preview, unread, flags
		FROM envelopes WHERE `+strings.Join(where, " AND ")+`
		ORDER BY CAST(uid AS INTEGER) DESC LIMIT ? OFFSET ?`, args...) // #nosec G202 -- WHERE fragments are fixed strings; user search terms are bound parameters.
	if err != nil {
		return nil, false, fmt.Errorf("could not read cached native envelopes: %w", err)
	}
	defer rows.Close()

	var messages []message
	for rows.Next() {
		var msg message
		var unread int
		var flags string
		if err := rows.Scan(&msg.ID, &msg.MessageID, &msg.From, &msg.Email, &msg.Subject, &msg.Date, &msg.Preview, &unread, &flags); err != nil {
			return nil, false, err
		}
		msg.Unread = unread != 0
		msg.Flagged = strings.Contains(flags, "flagged")
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	done := len(messages) <= pageSize
	if len(messages) > pageSize {
		messages = messages[:pageSize]
	}
	return messages, done, nil
}

func (s *nativeStore) saveBody(ctx context.Context, account, mailbox, uid, body string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `INSERT INTO message_bodies (account, mailbox, uid, body, content_type, cached_at)
		VALUES (?, ?, ?, ?, 'text/plain', ?)
		ON CONFLICT(account, mailbox, uid) DO UPDATE SET
			body = excluded.body,
			content_type = excluded.content_type,
			cached_at = excluded.cached_at`, account, mailbox, uid, body, now)
	if err != nil {
		return fmt.Errorf("could not save native message body: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `UPDATE envelopes SET body_cached = 1 WHERE account = ? AND mailbox = ? AND uid = ?`, account, mailbox, uid)
	return err
}

func (s *nativeStore) body(ctx context.Context, account, mailbox, uid string) (string, bool, error) {
	var body string
	err := s.db.QueryRowContext(ctx, `SELECT body FROM message_bodies WHERE account = ? AND mailbox = ? AND uid = ?`, account, mailbox, uid).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("could not read cached native message body: %w", err)
	}
	return body, true, nil
}

func (s *nativeStore) saveAppState(ctx context.Context, key string, value []byte) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `INSERT INTO app_state (key, value, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			updated_at = excluded.updated_at`, key, string(value), now)
	if err != nil {
		return fmt.Errorf("could not save clibox app state: %w", err)
	}
	return nil
}

func (s *nativeStore) appState(ctx context.Context, key string) ([]byte, bool, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM app_state WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("could not read clibox app state: %w", err)
	}
	return []byte(value), true, nil
}

func (s *nativeStore) schemaHasCredentialColumns(ctx context.Context) (bool, []string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT m.name, p.name FROM sqlite_master AS m JOIN pragma_table_info(m.name) AS p
		WHERE m.type = 'table' AND m.name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return false, nil, err
	}
	defer rows.Close()
	var bad []string
	for rows.Next() {
		var tableName, columnName string
		if err := rows.Scan(&tableName, &columnName); err != nil {
			return false, nil, err
		}
		if isCredentialConfigKey(columnName) {
			bad = append(bad, tableName+"."+columnName)
		}
	}
	return len(bad) > 0, bad, rows.Err()
}

func messageIDUint(msg message) (uint32, error) {
	id := strings.TrimSpace(msg.ID)
	uid, err := strconv.ParseUint(id, 10, 32)
	if err != nil || uid == 0 {
		return 0, errors.New("email has no readable native UID")
	}
	return uint32(uid), nil
}
