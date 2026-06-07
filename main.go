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
	account := flags.String("account", "", "Himalaya account name to read")
	mailbox := flags.String("mailbox", "INBOX", "Himalaya mailbox/folder to read")
	himalaya := flags.String("himalaya", "", "path to the Himalaya binary")
	pageSize := flags.Int("page-size", 100, "number of envelopes to request per page while loading the mailbox")
	showThemes := flags.Bool("themes", false, "list available themes")
	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "Usage: %s [doctor] [--theme name] [--account name] [--mailbox name] [--themes]\n\n", os.Args[0])
		flags.PrintDefaults()
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
		Himalaya: *himalaya,
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
