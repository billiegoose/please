package main

import (
	"fmt"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	tea "github.com/charmbracelet/bubbletea"
)

const version = "0.1.0"

const helpText = `please — natural language shell command translator

USAGE
  please                    open interactive TUI mode
  please <description>      translate and run a one-shot command
  please --setup            configure API key
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

CONFIGURATION
  API key is stored in ~/.config/please/config
  Set ANTHROPIC_API_KEY env var to override
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
		case "--setup":
			runSetup()
			return
		}
		runPlease(os.Args[1:])
		return
	}

	key, err := resolveAPIKey()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	client := anthropic.NewClient(option.WithAPIKey(key))

	p := tea.NewProgram(
		newModel(&client),
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
