package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

const defaultDownloadsDir = "~/Downloads"

type accountSetup struct {
	Account     string
	Email       string
	DisplayName string
	Provider    providerInfo
	Secret      string
	PageSize    int
}

type credentialRef struct {
	Command string
	Raw     string
}

func (h himalayaBackend) SaveAccountSetup(setup accountSetup) error {
	setup.Account = sanitizeAccountName(setup.Account, "")
	setup.Email = strings.TrimSpace(setup.Email)
	setup.DisplayName = firstNonEmpty(setup.DisplayName, displayNameFromEmail(setup.Email))
	setup.Secret = setup.Provider.normalizeSecret(setup.Secret)
	if setup.Account == "" {
		return errors.New("missing account name")
	}
	if !validEmailAddress(setup.Email) {
		return errors.New("missing valid email address")
	}
	if !setup.Provider.canAutoConfigure() {
		return fmt.Errorf("%s needs manual IMAP/SMTP settings before clibox can configure it automatically", setup.Provider.Name)
	}
	if setup.Secret == "" {
		return fmt.Errorf("missing %s", strings.ToLower(setup.Provider.secretLabel()))
	}
	if setup.PageSize <= 0 {
		setup.PageSize = h.pageSize
	}

	credential, err := saveCredential(setup)
	if err != nil {
		return err
	}

	path, err := himalayaConfigPath()
	if err != nil {
		return err
	}
	return writeHimalayaAccountConfig(path, setup, credential)
}

func saveCredential(setup accountSetup) (credentialRef, error) {
	service := credentialServiceName(setup)
	switch runtime.GOOS {
	case "darwin":
		return saveMacOSCredential(setup, service)
	case "linux":
		if _, err := exec.LookPath("secret-tool"); err == nil {
			return saveSecretToolCredential(setup, service)
		}
	}

	if os.Getenv("CLIBOX_ALLOW_RAW_PASSWORD") == "1" {
		return credentialRef{Raw: setup.Secret}, nil
	}

	return credentialRef{}, errors.New("automatic secure password storage needs macOS Keychain or Linux secret-tool; install a supported credential store or set CLIBOX_ALLOW_RAW_PASSWORD=1 to write the password into Himalaya config")
}

func saveMacOSCredential(setup accountSetup, service string) (credentialRef, error) {
	args := macOSKeychainAddArgs(setup, service)
	if output, err := runCredentialCommand("security", args, setup.Secret+"\n"); err != nil {
		return credentialRef{}, fmt.Errorf("could not save password to macOS Keychain: %s", oneLine(firstNonEmpty(string(output), err.Error())))
	}
	return credentialRef{Command: macOSKeychainFindCommand(setup, service)}, nil
}

func macOSKeychainAddArgs(setup accountSetup, service string) []string {
	return []string{"add-generic-password", "-a", setup.Email, "-s", service, "-U", "-w"}
}

func macOSKeychainFindCommand(setup accountSetup, service string) string {
	return "security find-generic-password -a " + shellQuote(setup.Email) + " -s " + shellQuote(service) + " -w"
}

func saveSecretToolCredential(setup accountSetup, service string) (credentialRef, error) {
	args := secretToolStoreArgs(setup, service)
	if output, err := runCredentialCommand("secret-tool", args, setup.Secret+"\n"); err != nil {
		return credentialRef{}, fmt.Errorf("could not save password with secret-tool: %s", oneLine(firstNonEmpty(string(output), err.Error())))
	}
	return credentialRef{Command: secretToolLookupCommand(setup, service)}, nil
}

func secretToolStoreArgs(setup accountSetup, service string) []string {
	label := "clibox " + setup.Account + " mail password"
	return []string{"store", "--label", label, "service", service, "account", setup.Email}
}

func secretToolLookupCommand(setup accountSetup, service string) string {
	return "secret-tool lookup service " + shellQuote(service) + " account " + shellQuote(setup.Email)
}

func runCredentialCommand(name string, args []string, input string) ([]byte, error) {
	cmd := exec.Command(name, args...) // #nosec G204 -- credential-store binaries are selected by clibox; user secrets are passed via stdin, not argv.
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}
	return cmd.CombinedOutput()
}

func writeHimalayaAccountConfig(path string, setup accountSetup, credential credentialRef) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("could not create Himalaya config directory: %w", err)
	}

	var content string
	if data, err := os.ReadFile(path); err == nil { // #nosec G304 -- path is the user's selected mail config path.
		content = string(data)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("could not read Himalaya config: %w", err)
	}

	defaultAccount := defaultAccountValue(content, setup.Account)
	block := buildHimalayaAccountBlock(setup, credential, defaultAccount)
	next := upsertAccountBlock(content, setup.Account, block)

	tmp, err := os.CreateTemp(filepath.Dir(path), ".clibox-*.toml")
	if err != nil {
		return fmt.Errorf("could not create temporary Himalaya config: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("could not secure temporary Himalaya config: %w", err)
	}
	if _, err := tmp.WriteString(next); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("could not write Himalaya config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("could not close temporary Himalaya config: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("could not replace Himalaya config: %w", err)
	}
	return nil
}

func himalayaAccountHint(account string) (accountSetup, bool) {
	path, err := himalayaConfigPath()
	if err != nil {
		return accountSetup{}, false
	}
	data, err := os.ReadFile(path) // #nosec G304 -- path is the user's selected mail config path.
	if err != nil {
		return accountSetup{}, false
	}
	content := string(data)

	name := sanitizeAccountName(account, "")
	if name == "" {
		name = defaultHimalayaAccountName(content)
	}
	if name == "" {
		return accountSetup{}, false
	}

	block, ok := accountBlock(content, name)
	if !ok {
		return accountSetup{}, false
	}
	email := tomlStringField(block, "email")
	if !validEmailAddress(email) {
		return accountSetup{}, false
	}
	return accountSetup{
		Account:     name,
		Email:       email,
		DisplayName: firstNonEmpty(tomlStringField(block, "display-name"), displayNameFromEmail(email)),
		Provider:    detectProvider(email),
	}, true
}

func buildHimalayaAccountBlock(setup accountSetup, credential credentialRef, defaultAccount bool) string {
	provider := setup.Provider
	folders := mergeFolders(provider.Folders)
	var lines []string
	lines = append(lines,
		"[accounts."+setup.Account+"]",
		"default = "+strconv.FormatBool(defaultAccount),
		"email = "+tomlString(setup.Email),
		"display-name = "+tomlString(setup.DisplayName),
		"downloads-dir = "+tomlString(defaultDownloadsDir),
		"folder.aliases.inbox = "+tomlString(folders["inbox"]),
		"folder.aliases.sent = "+tomlString(folders["sent"]),
		"folder.aliases.drafts = "+tomlString(folders["drafts"]),
		"folder.aliases.trash = "+tomlString(folders["trash"]),
		"folder.aliases.archive = "+tomlString(folders["archive"]),
		"backend.type = \"imap\"",
		"backend.host = "+tomlString(provider.IMAPHost),
		"backend.port = "+strconv.Itoa(provider.IMAPPort),
		"backend.encryption.type = "+tomlString(provider.IMAPSecurity),
		"backend.login = "+tomlString(setup.Email),
		"backend.auth.type = \"password\"",
	)
	if setup.PageSize > 0 {
		lines = append(lines, "envelope.list.page-size = "+strconv.Itoa(setup.PageSize))
	}
	lines = appendCredential(lines, "backend.auth", credential)
	lines = append(lines,
		"message.send.backend.type = \"smtp\"",
		"message.send.backend.host = "+tomlString(provider.SMTPHost),
		"message.send.backend.port = "+strconv.Itoa(provider.SMTPPort),
		"message.send.backend.encryption.type = "+tomlString(provider.SMTPSecurity),
		"message.send.backend.login = "+tomlString(setup.Email),
		"message.send.backend.auth.type = \"password\"",
	)
	lines = appendCredential(lines, "message.send.backend.auth", credential)
	return strings.Join(lines, "\n") + "\n"
}

func appendCredential(lines []string, prefix string, credential credentialRef) []string {
	if credential.Command != "" {
		return append(lines, prefix+".cmd = "+tomlString(credential.Command))
	}
	return append(lines, prefix+".raw = "+tomlString(credential.Raw))
}

func upsertAccountBlock(content, account, block string) string {
	content = strings.TrimRight(content, "\n")
	if strings.TrimSpace(content) == "" {
		return "downloads-dir = " + tomlString(defaultDownloadsDir) + "\n\n" + block
	}

	re := regexp.MustCompile(`(?m)^\[accounts\.` + regexp.QuoteMeta(account) + `\]\s*$`)
	match := re.FindStringIndex(content)
	if match == nil {
		return content + "\n\n" + block
	}

	nextAccount := regexp.MustCompile(`(?m)^\[accounts\.[^\]]+\]\s*$`)
	rest := content[match[1]:]
	next := nextAccount.FindStringIndex(rest)
	end := len(content)
	if next != nil {
		end = match[1] + next[0]
	}

	before := strings.TrimRight(content[:match[0]], "\n")
	after := strings.TrimLeft(content[end:], "\n")
	parts := []string{}
	if before != "" {
		parts = append(parts, before)
	}
	parts = append(parts, strings.TrimRight(block, "\n"))
	if after != "" {
		parts = append(parts, after)
	}
	return strings.Join(parts, "\n\n") + "\n"
}

func defaultAccountValue(content, account string) bool {
	if strings.TrimSpace(content) == "" || !strings.Contains(content, "[accounts.") {
		return true
	}
	block, ok := accountBlock(content, account)
	if !ok {
		return false
	}
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "default") && strings.Contains(line, "true") {
			return true
		}
	}
	return false
}

func defaultHimalayaAccountName(content string) string {
	re := regexp.MustCompile(`(?m)^\[accounts\.([^\]]+)\]\s*$`)
	matches := re.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return ""
	}

	first := content[matches[0][2]:matches[0][3]]
	for _, match := range matches {
		name := content[match[2]:match[3]]
		block, ok := accountBlock(content, name)
		if ok && defaultAccountValue(block, name) {
			return name
		}
	}
	return first
}

func accountBlock(content, account string) (string, bool) {
	re := regexp.MustCompile(`(?m)^\[accounts\.` + regexp.QuoteMeta(account) + `\]\s*$`)
	match := re.FindStringIndex(content)
	if match == nil {
		return "", false
	}

	nextAccount := regexp.MustCompile(`(?m)^\[accounts\.[^\]]+\]\s*$`)
	rest := content[match[1]:]
	next := nextAccount.FindStringIndex(rest)
	end := len(content)
	if next != nil {
		end = match[1] + next[0]
	}
	return content[match[0]:end], true
}

func himalayaConfigPath() (string, error) {
	if raw := strings.TrimSpace(os.Getenv("HIMALAYA_CONFIG")); raw != "" {
		first := strings.Split(raw, string(os.PathListSeparator))[0]
		return expandHome(first)
	}
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, "himalaya", "config.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not find home directory: %w", err)
	}
	return filepath.Join(home, ".config", "himalaya", "config.toml"), nil
}

func expandHome(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not expand home directory: %w", err)
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

func mergeFolders(folders map[string]string) map[string]string {
	merged := standardFolders()
	for key, value := range folders {
		if strings.TrimSpace(value) != "" {
			merged[key] = value
		}
	}
	return merged
}

func displayNameFromEmail(email string) string {
	local := strings.TrimSpace(email)
	if at := strings.Index(local, "@"); at > 0 {
		local = local[:at]
	}
	return firstNonEmpty(local, "clibox user")
}

func credentialServiceName(setup accountSetup) string {
	return "clibox:" + setup.Account + ":" + strings.ToLower(setup.Email)
}

func tomlString(value string) string {
	var out strings.Builder
	out.WriteByte('"')
	for _, r := range value {
		switch r {
		case '\\':
			out.WriteString("\\\\")
		case '"':
			out.WriteString("\\\"")
		case '\n':
			out.WriteString("\\n")
		case '\r':
			out.WriteString("\\r")
		case '\t':
			out.WriteString("\\t")
		default:
			out.WriteRune(r)
		}
	}
	out.WriteByte('"')
	return out.String()
}

func tomlStringField(content, key string) string {
	re := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(key) + `\s*=\s*("(?:\\.|[^"\\])*")\s*$`)
	match := re.FindStringSubmatch(content)
	if len(match) != 2 {
		return ""
	}
	value, err := strconv.Unquote(match[1])
	if err != nil {
		return ""
	}
	return value
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
