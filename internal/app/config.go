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
	Editor        string
	PageSize      int
	ConfirmDelete *bool
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
	for index, line := range strings.Split(normalizeConfigContent(content), "\n") {
		lineNo := index + 1
		line = strings.TrimSpace(stripTomlComment(line))
		if line == "" {
			continue
		}
		key, raw, ok := strings.Cut(line, "=")
		if !ok {
			return Config{}, fmt.Errorf("line %d: expected key = value", lineNo)
		}
		key = strings.TrimSpace(key)
		raw = strings.TrimSpace(raw)
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
			config.Backend = value
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
		default:
			return Config{}, fmt.Errorf("line %d: unknown config key %q", lineNo, key)
		}
	}
	return config, nil
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
