package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/anthropics/anthropic-sdk-go"
)

const version = "0.1.0"

const helpText = `please — natural language shell command translator

USAGE
  please                    open interactive TUI mode
  please <description>      translate and run a one-shot command
  please --help             show this help
  please --version          show version

EXAMPLES
  please list all docker containers including stopped ones
  please undo my last git commit but keep the changes
  please find files modified in the last hour

PICKER KEYS
  ↑↓      select between options
  enter   run selected command
  e       edit command before running
  esc     cancel

ENVIRONMENT
  ANTHROPIC_API_KEY   required
`

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help", "-h":
			fmt.Print(helpText)
			return
		case "--version", "-v":
			fmt.Println("please " + version)
			return
		}
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
