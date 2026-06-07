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
}

func detectProvider(email string) providerInfo {
	domain := emailDomain(email)
	switch domain {
	case "gmail.com", "googlemail.com":
		return providerInfo{
			Name:        "Gmail",
			Account:     "gmail",
			AuthSummary: "Gmail setup opens in your browser so you can create the app password Google requires for terminal mail clients.",
			HelpLabel:   "Open Google app passwords",
			HelpURL:     "https://myaccount.google.com/apppasswords",
			Instructions: []string{
				"Enable IMAP in Gmail settings if it is disabled.",
				"Use your full Gmail address as the username.",
				"Use a Google app password when Himalaya asks for a password.",
				"Accept the discovered Gmail server settings when Himalaya suggests them.",
			},
		}
	case "icloud.com", "me.com", "mac.com":
		return providerInfo{
			Name:        "iCloud Mail",
			Account:     "icloud",
			AuthSummary: "iCloud requires an app-specific password from your Apple Account.",
			HelpLabel:   "Open Apple Account",
			HelpURL:     "https://account.apple.com/account/manage",
			Instructions: []string{
				"Use your full iCloud email address as the username.",
				"Generate an app-specific password for Mail in your Apple Account security settings.",
				"Paste that app-specific password when Himalaya asks for a password.",
				"Accept the discovered iCloud server settings when Himalaya suggests them.",
			},
		}
	case "outlook.com", "hotmail.com", "live.com", "msn.com":
		return providerInfo{
			Name:        "Outlook",
			Account:     "outlook",
			AuthSummary: "Outlook may require an app password or IMAP access depending on your account security.",
			HelpLabel:   "Open Microsoft security",
			HelpURL:     "https://account.microsoft.com/security",
			Instructions: []string{
				"Use your full Outlook email address as the username.",
				"If your normal password is rejected, create an app password or enable IMAP access in Microsoft account settings.",
				"Accept the discovered Outlook server settings when Himalaya suggests them.",
			},
		}
	case "yahoo.com", "ymail.com", "rocketmail.com":
		return providerInfo{
			Name:        "Yahoo Mail",
			Account:     "yahoo",
			AuthSummary: "Yahoo usually requires an app password for third-party mail clients.",
			HelpLabel:   "Open Yahoo security",
			HelpURL:     "https://login.yahoo.com/account/security",
			Instructions: []string{
				"Use your full Yahoo email address as the username.",
				"Create an app password in Yahoo account security settings.",
				"Paste the app password when Himalaya asks for a password.",
				"Accept the discovered Yahoo server settings when Himalaya suggests them.",
			},
		}
	case "fastmail.com":
		return providerInfo{
			Name:        "Fastmail",
			Account:     "fastmail",
			AuthSummary: "Fastmail works best with an app password for mail clients.",
			HelpLabel:   "Open Fastmail app passwords",
			HelpURL:     "https://app.fastmail.com/settings/security/integrations",
			Instructions: []string{
				"Use your full Fastmail address as the username.",
				"Create an app password in Fastmail settings.",
				"Paste the app password when Himalaya asks for a password.",
				"Accept the discovered Fastmail server settings when Himalaya suggests them.",
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
			AuthSummary: "clibox will let Himalaya try automatic provider discovery.",
			Instructions: []string{
				"Use your full email address as the username unless your provider says otherwise.",
				"If discovery fails, your provider may list IMAP and SMTP settings in its help docs.",
				"You may need an app password if two-factor authentication is enabled.",
			},
		}
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
