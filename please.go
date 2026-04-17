package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type pickerModel struct {
	suggestions []Suggestion
	selected    int
	ready       bool
	input       string
	editing     bool
	ti          textinput.Model
	spinner     spinner.Model
	loading     bool
	err         string
}

func newPickerModel(input string) pickerModel {
	ti := textinput.New()
	ti.Cursor.Style = cursorStyle
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = explanStyle

	return pickerModel{input: input, ti: ti, spinner: sp, loading: true}
}

func (m pickerModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, textinput.Blink)
}

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.loading {
			if msg.Type == tea.KeyCtrlC {
				return m, tea.Quit
			}
			return m, nil
		}
		if m.editing {
			switch msg.Type {
			case tea.KeyCtrlC:
				return m, tea.Quit
			case tea.KeyEsc:
				m.editing = false
				m.ti.Blur()
			case tea.KeyEnter:
				m.ready = true
				return m, tea.Quit
			default:
				var tiCmd tea.Cmd
				m.ti, tiCmd = m.ti.Update(msg)
				return m, tiCmd
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
				if string(msg.Runes) == "e" && len(m.suggestions) > 0 {
					m.editing = true
					m.ti.SetValue(m.suggestions[m.selected].Cmd)
					m.ti.SetCursor(len(m.ti.Value()))
					m.ti.Focus()
				}
			}
		}

	case translationResultMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.suggestions = msg.suggestions
		}
		return m, nil

	case spinner.TickMsg:
		var spCmd tea.Cmd
		m.spinner, spCmd = m.spinner.Update(msg)
		return m, spCmd
	}
	return m, nil
}

func (m pickerModel) View() string {
	var sb strings.Builder
	sb.WriteString(explanStyle.Render("  "+m.input) + "\n\n")

	if m.loading {
		sb.WriteString("  " + m.spinner.View() + explanStyle.Render(" translating…") + "\n")
		return sb.String()
	}

	if m.err != "" {
		sb.WriteString(errorStyle.Render("  error: "+m.err) + "\n")
		return sb.String()
	}

	for i, s := range m.suggestions {
		if i == m.selected {
			if m.editing {
				sb.WriteString(promptStyle.Render("▶ ") + "$ " + m.ti.View() + "\n")
			} else {
				line := selectedCmdStyle.Render("$ "+s.Cmd) + "  " + explanStyle.Render("("+s.Explanation+")")
				sb.WriteString(promptStyle.Render("▶ ") + line + "\n")
			}
		} else {
			line := dividerStyle.Render("$ ") + cmdStyle.Render(s.Cmd) + "  " + explanStyle.Render("("+s.Explanation+")")
			sb.WriteString("  " + line + "\n")
		}
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

	// Start picker in loading state; kick off translation as first Cmd
	m := newPickerModel(input)

	translateCmd := func() tea.Msg {
		suggestions, err := translate(context.Background(), &client, input)
		return translationResultMsg{input: input, suggestions: suggestions, err: err}
	}

	p2 := tea.NewProgram(pickerRunner{inner: m, translateCmd: translateCmd})
	result, err := p2.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, errorStyle.Render("error: "+err.Error()))
		os.Exit(1)
	}

	pm := result.(pickerRunner).inner
	if !pm.ready {
		os.Exit(0)
	}

	var chosen Suggestion
	if pm.editing {
		chosen = Suggestion{Cmd: pm.ti.Value(), Interactive: pm.suggestions[pm.selected].Interactive}
	} else {
		chosen = pm.suggestions[pm.selected]
	}
	execCmd(chosen)
}

// pickerRunner wraps pickerModel so we can inject an initial Cmd.
type pickerRunner struct {
	inner        pickerModel
	translateCmd tea.Cmd
}

func (r pickerRunner) Init() tea.Cmd {
	return tea.Batch(r.inner.Init(), r.translateCmd)
}

func (r pickerRunner) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	inner, cmd := r.inner.Update(msg)
	r.inner = inner.(pickerModel)
	return r, cmd
}

func (r pickerRunner) View() string {
	return r.inner.View()
}

func execCmd(s Suggestion) {
	bash, err := findBash()
	if err != nil {
		fmt.Fprintln(os.Stderr, errorStyle.Render("cannot find bash: "+err.Error()))
		os.Exit(1)
	}
	if err := syscall.Exec(bash, []string{"bash", "-c", s.Cmd}, os.Environ()); err != nil {
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
