package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Freddster16/clibox/internal/app"
)

func main() {
	flags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	theme := flags.String("theme", "", "start clibox with a theme: nocturne, ember, or lagoon")
	account := flags.String("account", "", "email account name to read")
	mailbox := flags.String("mailbox", "INBOX", "mailbox/folder to read")
	backendBinary := flags.String("backend", "", "advanced: path to the email backend binary")
	himalaya := flags.String("himalaya", "", "deprecated alias for --backend")
	pageSize := flags.Int("page-size", 0, "advanced: envelopes to request per page; 0 loads all pages with backend defaults")
	showThemes := flags.Bool("themes", false, "list available themes")
	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "Usage: %s [doctor] [--theme name] [--account name] [--mailbox name] [--themes]\n\n", os.Args[0])
		fmt.Fprintln(flags.Output(), "  -account string")
		fmt.Fprintln(flags.Output(), "    \temail account name to read")
		fmt.Fprintln(flags.Output(), "  -backend string")
		fmt.Fprintln(flags.Output(), "    \tadvanced: path to the email backend binary")
		fmt.Fprintln(flags.Output(), "  -mailbox string")
		fmt.Fprintln(flags.Output(), "    \tmailbox/folder to read (default \"INBOX\")")
		fmt.Fprintln(flags.Output(), "  -page-size int")
		fmt.Fprintln(flags.Output(), "    \tadvanced: envelopes to request per page; 0 loads all pages with backend defaults")
		fmt.Fprintln(flags.Output(), "  -theme string")
		fmt.Fprintln(flags.Output(), "    \tstart clibox with a theme: nocturne, ember, or lagoon")
		fmt.Fprintln(flags.Output(), "  -themes")
		fmt.Fprintln(flags.Output(), "    \tlist available themes")
	}

	args := os.Args[1:]
	doctor := false
	if len(args) > 0 && args[0] == "doctor" {
		doctor = true
		args = args[1:]
	}

	if err := flags.Parse(args); err != nil {
		os.Exit(2)
	}
	if *showThemes {
		fmt.Fprint(os.Stdout, app.ThemeHelp())
		return
	}

	options := app.Options{
		Theme:    *theme,
		Account:  *account,
		Mailbox:  *mailbox,
		Himalaya: firstNonEmpty(*backendBinary, *himalaya),
		PageSize: *pageSize,
	}
	if doctor {
		report, err := app.Doctor(context.Background(), options)
		if err != nil {
			fmt.Fprintf(os.Stderr, "clibox doctor failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stdout, report)
		return
	}

	program := tea.NewProgram(app.NewWithOptions(options), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "clibox failed: %v\n", err)
		os.Exit(1)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
