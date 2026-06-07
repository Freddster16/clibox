package app

import (
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

type setupStep int

const (
	setupEmailStep setupStep = iota
	setupReviewStep
	setupSecretStep
	setupAccountStep
)

type providerInfo struct {
	Name          string
	Account       string
	AuthSummary   string
	Instructions  []string
	ManualWarning string
	HelpLabel     string
	HelpURL       string
	SecretLabel   string
	IMAPHost      string
	IMAPPort      int
	IMAPSecurity  string
	SMTPHost      string
	SMTPPort      int
	SMTPSecurity  string
	Folders       map[string]string
}

func detectProvider(email string) providerInfo {
	domain := emailDomain(email)
	switch domain {
	case "gmail.com", "googlemail.com":
		return providerInfo{
			Name:         "Gmail",
			Account:      "gmail",
			AuthSummary:  "Gmail is detected. clibox can fill the Gmail IMAP/SMTP settings in the background; you only need a Google app password for terminal mail access.",
			HelpLabel:    "Open Google app passwords",
			HelpURL:      "https://myaccount.google.com/apppasswords",
			SecretLabel:  "Google app password",
			IMAPHost:     "imap.gmail.com",
			IMAPPort:     993,
			IMAPSecurity: "tls",
			SMTPHost:     "smtp.gmail.com",
			SMTPPort:     587,
			SMTPSecurity: "start-tls",
			Folders: map[string]string{
				"inbox":  "INBOX",
				"sent":   "[Gmail]/Sent Mail",
				"drafts": "[Gmail]/Drafts",
				"trash":  "[Gmail]/Trash",
			},
			Instructions: []string{
				"Enable IMAP in Gmail settings if it is disabled.",
				"Use the app password from Google, not your normal Google password.",
				"clibox uses your full Gmail address as the username and writes the Gmail server settings for you.",
			},
		}
	case "icloud.com", "me.com", "mac.com":
		return providerInfo{
			Name:         "iCloud Mail",
			Account:      "icloud",
			AuthSummary:  "iCloud Mail is detected. clibox can fill the iCloud IMAP/SMTP settings in the background; you only need an Apple app-specific password.",
			HelpLabel:    "Open Apple Account",
			HelpURL:      "https://account.apple.com/account/manage",
			SecretLabel:  "Apple app-specific password",
			IMAPHost:     "imap.mail.me.com",
			IMAPPort:     993,
			IMAPSecurity: "tls",
			SMTPHost:     "smtp.mail.me.com",
			SMTPPort:     587,
			SMTPSecurity: "start-tls",
			Folders:      standardFolders(),
			Instructions: []string{
				"Generate an app-specific password for Mail in your Apple Account security settings.",
				"clibox uses your full iCloud email address as the username and writes the iCloud server settings for you.",
			},
		}
	case "outlook.com", "hotmail.com", "live.com", "msn.com":
		return providerInfo{
			Name:         "Outlook",
			Account:      "outlook",
			AuthSummary:  "Outlook is detected. clibox can fill the Outlook IMAP/SMTP hosts, but Microsoft may require Modern Auth/OAuth for some accounts.",
			HelpLabel:    "Open Microsoft security",
			HelpURL:      "https://account.microsoft.com/security",
			SecretLabel:  "Outlook password or app password",
			IMAPHost:     "outlook.office365.com",
			IMAPPort:     993,
			IMAPSecurity: "tls",
			SMTPHost:     "smtp-mail.outlook.com",
			SMTPPort:     587,
			SMTPSecurity: "start-tls",
			Folders:      standardFolders(),
			Instructions: []string{
				"Enable IMAP access in Outlook.com settings if it is disabled.",
				"If password login is rejected, this account may need OAuth support that is not in clibox yet.",
				"clibox uses your full Outlook email address as the username and writes the server settings for you.",
			},
		}
	case "yahoo.com", "ymail.com", "rocketmail.com":
		return providerInfo{
			Name:         "Yahoo Mail",
			Account:      "yahoo",
			AuthSummary:  "Yahoo Mail is detected. clibox can fill the Yahoo IMAP/SMTP settings in the background; you only need a Yahoo app password.",
			HelpLabel:    "Open Yahoo security",
			HelpURL:      "https://login.yahoo.com/account/security",
			SecretLabel:  "Yahoo app password",
			IMAPHost:     "imap.mail.yahoo.com",
			IMAPPort:     993,
			IMAPSecurity: "tls",
			SMTPHost:     "smtp.mail.yahoo.com",
			SMTPPort:     587,
			SMTPSecurity: "start-tls",
			Folders:      standardFolders(),
			Instructions: []string{
				"Create an app password in Yahoo account security settings.",
				"clibox uses your full Yahoo email address as the username and writes the Yahoo server settings for you.",
			},
		}
	case "fastmail.com":
		return providerInfo{
			Name:         "Fastmail",
			Account:      "fastmail",
			AuthSummary:  "Fastmail is detected. clibox can fill the Fastmail IMAP/SMTP settings in the background; you only need a Fastmail app password.",
			HelpLabel:    "Open Fastmail app passwords",
			HelpURL:      "https://app.fastmail.com/settings/security/integrations",
			SecretLabel:  "Fastmail app password",
			IMAPHost:     "imap.fastmail.com",
			IMAPPort:     993,
			IMAPSecurity: "tls",
			SMTPHost:     "smtp.fastmail.com",
			SMTPPort:     587,
			SMTPSecurity: "start-tls",
			Folders:      standardFolders(),
			Instructions: []string{
				"Create an app password in Fastmail settings.",
				"clibox uses your full Fastmail address as the username and writes the Fastmail server settings for you.",
			},
		}
	case "proton.me", "protonmail.com":
		return providerInfo{
			Name:          "Proton Mail",
			Account:       "proton",
			AuthSummary:   "Proton Mail usually needs Proton Mail Bridge before IMAP clients can connect.",
			ManualWarning: "Set up Proton Mail Bridge first, then use the Bridge IMAP/SMTP details in Himalaya.",
			HelpLabel:     "Open Proton Mail Bridge",
			HelpURL:       "https://proton.me/mail/bridge",
			Instructions: []string{
				"Install and sign in to Proton Mail Bridge before continuing.",
				"Use the local Bridge username and password, not your normal Proton password.",
				"Choose custom/manual settings in Himalaya if autodiscovery does not find the Bridge.",
			},
		}
	default:
		account := accountNameFromDomain(domain)
		return providerInfo{
			Name:        "Custom mail",
			Account:     account,
			AuthSummary: "clibox does not know this provider's IMAP/SMTP settings yet, so automatic background setup is limited.",
			Instructions: []string{
				"Use your full email address as the username unless your provider says otherwise.",
				"Your provider should list IMAP and SMTP settings in its help docs.",
				"You may need an app password if two-factor authentication is enabled.",
			},
		}
	}
}

func (p providerInfo) canAutoConfigure() bool {
	return strings.TrimSpace(p.IMAPHost) != "" && p.IMAPPort > 0 && strings.TrimSpace(p.SMTPHost) != "" && p.SMTPPort > 0
}

func (p providerInfo) secretLabel() string {
	return firstNonEmpty(p.SecretLabel, "Email password or app password")
}

func standardFolders() map[string]string {
	return map[string]string{
		"inbox":  "INBOX",
		"sent":   "Sent",
		"drafts": "Drafts",
		"trash":  "Trash",
	}
}

func validEmailAddress(value string) bool {
	value = strings.TrimSpace(value)
	at := strings.LastIndex(value, "@")
	if at <= 0 || at == len(value)-1 {
		return false
	}
	domain := value[at+1:]
	return strings.Contains(domain, ".") && !strings.ContainsAny(value, " \t\r\n")
}

func emailDomain(email string) string {
	email = strings.TrimSpace(strings.ToLower(email))
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return ""
	}
	return email[at+1:]
}

func accountNameFromDomain(domain string) string {
	domain = strings.TrimSpace(strings.ToLower(domain))
	if domain == "" {
		return "personal"
	}
	label := strings.Split(domain, ".")[0]
	return sanitizeAccountName(label, "personal")
}

func sanitizeAccountName(value, fallback string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	var out strings.Builder
	for _, r := range value {
		if isAccountNameRune(r) {
			out.WriteRune(r)
		}
	}
	if strings.TrimSpace(out.String()) == "" {
		return fallback
	}
	return out.String()
}

func isEmailRune(r rune) bool {
	return r > 32 && r < 127
}

func openURL(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return errors.New("missing URL")
	}

	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", rawURL).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL).Start()
	default:
		return exec.Command("xdg-open", rawURL).Start()
	}
}
