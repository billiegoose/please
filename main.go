package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/anthropics/anthropic-sdk-go"
)

func main() {
	if len(os.Args) > 1 {
		runPlease(os.Args[1:])
		return
	}

	client := anthropic.NewClient() // reads ANTHROPIC_API_KEY from env

	p := tea.NewProgram(
		newModel(&client),
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
