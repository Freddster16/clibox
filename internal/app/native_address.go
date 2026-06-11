package app

import (
	"net/mail"
	"strings"
)

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
