package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Theme         string
	Account       string
	Mailbox       string
	ArchiveFolder string
	Backend       string
	Himalaya      string
	Editor        string
	PageSize      int
	ConfirmDelete *bool
	ComposeFormat string
	Accounts      map[string]AccountConfig
}

type AccountConfig struct {
	Name          string
	Provider      string
	Email         string
	Mailbox       string
	ArchiveFolder string
	SyncPolicy    string
	Editor        string
}

func LoadConfig(path string) (Config, string, error) {
	resolved, err := cliboxConfigPath(path)
	if err != nil {
		return Config{}, "", err
	}
	data, err := os.ReadFile(resolved) // #nosec G304 -- config path is explicit user configuration, or the standard clibox config path.
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, resolved, nil
	}
	if err != nil {
		return Config{}, resolved, fmt.Errorf("could not read clibox config: %w", err)
	}
	if err := validateConfigPermissions(resolved); err != nil {
		return Config{}, resolved, err
	}
	config, err := parseConfig(string(data))
	if err != nil {
		return Config{}, resolved, fmt.Errorf("%s: %w", resolved, err)
	}
	return config, resolved, nil
}

func validateConfigPermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("could not inspect clibox config permissions: %w", err)
	}
	if info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("clibox config must not be group- or world-writable: %s", path)
	}
	return nil
}

func cliboxConfigPath(path string) (string, error) {
	if path = strings.TrimSpace(path); path != "" {
		return expandHome(path)
	}
	if env := strings.TrimSpace(os.Getenv("CLIBOX_CONFIG")); env != "" {
		return expandHome(env)
	}
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, "clibox", "config.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not find home directory: %w", err)
	}
	return filepath.Join(home, ".config", "clibox", "config.toml"), nil
}

func parseConfig(content string) (Config, error) {
	var config Config
	section := ""
	sectionName := ""
	for index, line := range strings.Split(normalizeConfigContent(content), "\n") {
		lineNo := index + 1
		line = strings.TrimSpace(stripTomlComment(line))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := strings.TrimSpace(strings.Trim(line, "[]"))
			switch {
			case strings.HasPrefix(name, "accounts."):
				account := strings.TrimSpace(strings.TrimPrefix(name, "accounts."))
				account = strings.Trim(account, `"`)
				account = sanitizeAccountName(account, "")
				if account == "" {
					return Config{}, fmt.Errorf("line %d: account section needs a name", lineNo)
				}
				if config.Accounts == nil {
					config.Accounts = map[string]AccountConfig{}
				}
				entry := config.Accounts[account]
				entry.Name = account
				config.Accounts[account] = entry
				section = "accounts"
				sectionName = account
			default:
				return Config{}, fmt.Errorf("line %d: unknown config section %q", lineNo, name)
			}
			continue
		}
		key, raw, ok := strings.Cut(line, "=")
		if !ok {
			return Config{}, fmt.Errorf("line %d: expected key = value", lineNo)
		}
		key = strings.TrimSpace(key)
		raw = strings.TrimSpace(raw)
		if isCredentialConfigKey(key) {
			return Config{}, fmt.Errorf("line %d: credential key %q is not allowed in clibox config", lineNo, key)
		}

		if section == "accounts" {
			entry := config.Accounts[sectionName]
			if err := parseAccountConfigField(&entry, key, raw, lineNo); err != nil {
				return Config{}, err
			}
			config.Accounts[sectionName] = entry
			continue
		}

		switch key {
		case "theme":
			value, err := parseConfigString(raw)
			if err != nil {
				return Config{}, fmt.Errorf("line %d: %w", lineNo, err)
			}
			config.Theme = value
		case "account":
			value, err := parseConfigString(raw)
			if err != nil {
				return Config{}, fmt.Errorf("line %d: %w", lineNo, err)
			}
			config.Account = value
		case "mailbox":
			value, err := parseConfigString(raw)
			if err != nil {
				return Config{}, fmt.Errorf("line %d: %w", lineNo, err)
			}
			config.Mailbox = value
		case "archive_folder":
			value, err := parseConfigString(raw)
			if err != nil {
				return Config{}, fmt.Errorf("line %d: %w", lineNo, err)
			}
			config.ArchiveFolder = value
		case "backend":
			value, err := parseConfigString(raw)
			if err != nil {
				return Config{}, fmt.Errorf("line %d: %w", lineNo, err)
			}
			applyBackendConfigValue(&config, value)
		case "himalaya_binary":
			value, err := parseConfigString(raw)
			if err != nil {
				return Config{}, fmt.Errorf("line %d: %w", lineNo, err)
			}
			config.Himalaya = value
		case "editor":
			value, err := parseConfigString(raw)
			if err != nil {
				return Config{}, fmt.Errorf("line %d: %w", lineNo, err)
			}
			config.Editor = value
		case "page_size":
			value, err := strconv.Atoi(raw)
			if err != nil || value < 0 {
				return Config{}, fmt.Errorf("line %d: page_size must be a non-negative integer", lineNo)
			}
			config.PageSize = value
		case "confirm_delete":
			value, err := strconv.ParseBool(raw)
			if err != nil {
				return Config{}, fmt.Errorf("line %d: confirm_delete must be true or false", lineNo)
			}
			config.ConfirmDelete = &value
		case "compose_format":
			value, err := parseConfigString(raw)
			if err != nil {
				return Config{}, fmt.Errorf("line %d: %w", lineNo, err)
			}
			value = strings.ToLower(strings.TrimSpace(value))
			if value != "" && value != "text" && value != "markdown" {
				return Config{}, fmt.Errorf("line %d: compose_format must be text or markdown", lineNo)
			}
			config.ComposeFormat = value
		default:
			return Config{}, fmt.Errorf("line %d: unknown config key %q", lineNo, key)
		}
	}
	return config, nil
}

func parseAccountConfigField(entry *AccountConfig, key, raw string, lineNo int) error {
	stringField := func() (string, error) {
		value, err := parseConfigString(raw)
		if err != nil {
			return "", fmt.Errorf("line %d: %w", lineNo, err)
		}
		return value, nil
	}

	switch key {
	case "provider":
		value, err := stringField()
		if err != nil {
			return err
		}
		entry.Provider = strings.TrimSpace(strings.ToLower(value))
	case "email":
		value, err := stringField()
		if err != nil {
			return err
		}
		entry.Email = value
	case "mailbox":
		value, err := stringField()
		if err != nil {
			return err
		}
		entry.Mailbox = value
	case "archive_folder":
		value, err := stringField()
		if err != nil {
			return err
		}
		entry.ArchiveFolder = value
	case "sync_policy":
		value, err := stringField()
		if err != nil {
			return err
		}
		entry.SyncPolicy = strings.TrimSpace(strings.ToLower(value))
	case "editor":
		value, err := stringField()
		if err != nil {
			return err
		}
		entry.Editor = value
	default:
		return fmt.Errorf("line %d: unknown account config key %q", lineNo, key)
	}
	return nil
}

func applyBackendConfigValue(config *Config, value string) {
	value = strings.TrimSpace(value)
	switch strings.ToLower(value) {
	case "", "himalaya", "native":
		config.Backend = strings.ToLower(value)
	default:
		config.Backend = "himalaya"
		config.Himalaya = value
	}
}

func isCredentialConfigKey(key string) bool {
	key = strings.TrimSpace(strings.ToLower(key))
	key = strings.TrimPrefix(key, "oauth_")
	if key == "" {
		return false
	}
	parts := strings.FieldsFunc(key, func(r rune) bool {
		return r == '.' || r == '-' || r == '_'
	})
	for _, part := range parts {
		switch part {
		case "password", "passwd", "secret":
			return true
		}
	}
	return key == "token" || strings.HasSuffix(key, "_token") || strings.HasSuffix(key, ".token") ||
		strings.Contains(key, "access_token") || strings.Contains(key, "refresh_token") || strings.Contains(key, "id_token")
}

func normalizeConfigContent(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	return strings.ReplaceAll(content, "\r", "\n")
}

func stripTomlComment(line string) string {
	inString := false
	escaped := false
	for i, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if inString && r == '\\' {
			escaped = true
			continue
		}
		if r == '"' {
			inString = !inString
			continue
		}
		if !inString && r == '#' {
			return line[:i]
		}
	}
	return line
}

func parseConfigString(raw string) (string, error) {
	value, err := strconv.Unquote(raw)
	if err != nil {
		return "", errors.New("expected a quoted string")
	}
	return strings.TrimSpace(value), nil
}

func cliboxStatePath(path string) (string, error) {
	if path = strings.TrimSpace(path); path != "" {
		return expandHome(path)
	}
	if env := strings.TrimSpace(os.Getenv("CLIBOX_STATE")); env != "" {
		return expandHome(env)
	}
	if xdg := strings.TrimSpace(os.Getenv("XDG_STATE_HOME")); xdg != "" {
		return filepath.Join(xdg, "clibox", "clibox.db"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not find home directory: %w", err)
	}
	return filepath.Join(home, ".local", "state", "clibox", "clibox.db"), nil
}
