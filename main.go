package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Freddster16/clibox/internal/app"
)

func main() {
	flags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	theme := flags.String("theme", "", "start clibox with a theme: nocturne, ember, or lagoon")
	showThemes := flags.Bool("themes", false, "list available themes")
	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "Usage: %s [--theme name] [--themes]\n\n", os.Args[0])
		flags.PrintDefaults()
	}

	if err := flags.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}
	if *showThemes {
		fmt.Fprint(os.Stdout, app.ThemeHelp())
		return
	}

	program := tea.NewProgram(app.NewWithTheme(*theme), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "clibox failed: %v\n", err)
		os.Exit(1)
	}
}
