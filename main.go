package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Freddster16/clibox/internal/app"
)

func main() {
	program := tea.NewProgram(app.New(), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "clibox failed: %v\n", err)
		os.Exit(1)
	}
}
