package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/anthropics/anthropic-sdk-go"
)

// pickerModel is a minimal TUI: show suggestions, pick one, optionally edit, exec it.
type pickerModel struct {
	suggestions []Suggestion
	selected    int
	ready       bool
	input       string

	// edit mode
	editing     bool
	editBuf     string
	editCursor  int
}

func (m pickerModel) Init() tea.Cmd {
	return nil
}

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.editing {
			switch msg.Type {
			case tea.KeyCtrlC:
				return m, tea.Quit
			case tea.KeyEsc:
				// back to picker
				m.editing = false
			case tea.KeyEnter:
				m.ready = true
				return m, tea.Quit
			case tea.KeyLeft:
				if m.editCursor > 0 {
					m.editCursor--
				}
			case tea.KeyRight:
				if m.editCursor < len(m.editBuf) {
					m.editCursor++
				}
			case tea.KeyHome, tea.KeyCtrlA:
				m.editCursor = 0
			case tea.KeyEnd, tea.KeyCtrlE:
				m.editCursor = len(m.editBuf)
			case tea.KeyBackspace:
				if m.editCursor > 0 {
					m.editBuf = m.editBuf[:m.editCursor-1] + m.editBuf[m.editCursor:]
					m.editCursor--
				}
			case tea.KeyDelete:
				if m.editCursor < len(m.editBuf) {
					m.editBuf = m.editBuf[:m.editCursor] + m.editBuf[m.editCursor+1:]
				}
			case tea.KeySpace:
				m.editBuf = m.editBuf[:m.editCursor] + " " + m.editBuf[m.editCursor:]
				m.editCursor++
			case tea.KeyRunes:
				m.editBuf = m.editBuf[:m.editCursor] + string(msg.Runes) + m.editBuf[m.editCursor:]
				m.editCursor += len(msg.Runes)
			}
		} else {
			switch msg.Type {
			case tea.KeyCtrlC, tea.KeyEsc:
				return m, tea.Quit
			case tea.KeyEnter:
				m.ready = true
				return m, tea.Quit
			case tea.KeyUp:
				if m.selected > 0 {
					m.selected--
				}
			case tea.KeyDown:
				if m.selected < len(m.suggestions)-1 {
					m.selected++
				}
			case tea.KeyRunes:
				if string(msg.Runes) == "e" {
					m.editing = true
					m.editBuf = m.suggestions[m.selected].Cmd
					m.editCursor = len(m.editBuf)
				}
			}
		}
	}
	return m, nil
}

func (m pickerModel) View() string {
	// When ready, force editing=false so we render the plain selected command without a cursor
	if m.ready {
		m.editing = false
	}
	var sb strings.Builder
	sb.WriteString(explanStyle.Render("  "+m.input) + "\n\n")

	for i, s := range m.suggestions {
		prefix := "  "
		var line string
		if i == m.selected {
			if m.editing {
				// render the editable buffer with a cursor
				before := m.editBuf[:m.editCursor]
				var cursorChar string
				if m.editCursor < len(m.editBuf) {
					cursorChar = string(m.editBuf[m.editCursor])
				} else {
					cursorChar = " "
				}
				after := ""
				if m.editCursor < len(m.editBuf) {
					after = m.editBuf[m.editCursor+1:]
				}
				editLine := selectedCmdStyle.Render("$ "+before) +
					cursorStyle.Render(cursorChar) +
					selectedCmdStyle.Render(after)
				line = editLine
			} else {
				prefix = promptStyle.Render("▶ ")
				line = selectedCmdStyle.Render("$ "+s.Cmd) + "  " + explanStyle.Render("("+s.Explanation+")")
			}
		} else {
			line = dividerStyle.Render("$ ") + cmdStyle.Render(s.Cmd) + "  " + explanStyle.Render("("+s.Explanation+")")
		}
		sb.WriteString(prefix + line + "\n")
	}

	if !m.ready {
		sb.WriteString("\n")
		if m.editing {
			sb.WriteString(explanStyle.Render("←→ move · enter run · esc back"))
		} else {
			sb.WriteString(explanStyle.Render("↑↓ select · enter run · e edit · esc cancel"))
		}
	}
	return sb.String()
}

func runPlease(args []string) {
	input := strings.Join(args, " ")

	client := anthropic.NewClient()

	fmt.Print(explanStyle.Render("translating…"))

	suggestions, err := translate(context.Background(), &client, input)

	// clear the "translating…" line
	fmt.Print("\r\033[K")

	if err != nil {
		fmt.Fprintln(os.Stderr, errorStyle.Render("error: "+err.Error()))
		os.Exit(1)
	}
	if len(suggestions) == 0 {
		fmt.Fprintln(os.Stderr, errorStyle.Render("no suggestions"))
		os.Exit(1)
	}

	p := tea.NewProgram(pickerModel{suggestions: suggestions, input: input})
	result, err := p.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, errorStyle.Render("error: "+err.Error()))
		os.Exit(1)
	}

	pm := result.(pickerModel)
	if !pm.ready {
		// user cancelled
		os.Exit(0)
	}

	var chosen Suggestion
	if pm.editing {
		chosen = Suggestion{Cmd: pm.editBuf, Interactive: pm.suggestions[pm.selected].Interactive}
	} else {
		chosen = pm.suggestions[pm.selected]
	}
	execCmd(chosen)
}

func execCmd(s Suggestion) {
	bash, err := findBash()
	if err != nil {
		fmt.Fprintln(os.Stderr, errorStyle.Render("cannot find bash: "+err.Error()))
		os.Exit(1)
	}
	args := []string{"bash", "-c", s.Cmd}
	if err := syscall.Exec(bash, args, os.Environ()); err != nil {
		fmt.Fprintln(os.Stderr, errorStyle.Render("exec failed: "+err.Error()))
		os.Exit(1)
	}
}

func findBash() (string, error) {
	for _, p := range []string{"/bin/bash", "/usr/bin/bash", "/usr/local/bin/bash"} {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("bash not found")
}
