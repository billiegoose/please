package main

import (
	"context"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/anthropics/anthropic-sdk-go"
)

// --- styles ---
var (
	outputStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	dividerStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	cmdStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	selectedCmdStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true)
	explanStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	promptStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
	inputStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	errorStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	cursorStyle      = lipgloss.NewStyle().Reverse(true)
)

// --- messages ---
type translationResultMsg struct {
	input       string
	suggestions []Suggestion
	err         error
}

type commandOutputMsg struct {
	cmd    string
	output string
	err    error
}

type debounceTickMsg struct {
	input string
}

// --- model ---
type model struct {
	client      *anthropic.Client
	width       int
	height      int
	input       string
	suggestions []Suggestion
	selected    int
	translating bool
	output      []string // scrollback history
	lastInput   string   // input that produced current suggestions
	lastErr     string
}

func newModel(client *anthropic.Client) model {
	return model{client: client}
}

func (m model) Init() tea.Cmd {
	return nil
}

// debounce: fire translation after 300ms of no typing
func debounceTranslate(input string) tea.Cmd {
	return tea.Tick(300*time.Millisecond, func(_ time.Time) tea.Msg {
		return debounceTickMsg{input: input}
	})
}

func doTranslate(client *anthropic.Client, input string) tea.Cmd {
	return func() tea.Msg {
		suggestions, err := translate(context.Background(), client, input)
		return translationResultMsg{input: input, suggestions: suggestions, err: err}
	}
}

func runCommand(cmdStr string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("bash", "-c", cmdStr)
		out, err := cmd.CombinedOutput()
		output := strings.TrimRight(string(out), "\n")
		if err != nil {
			return commandOutputMsg{cmd: cmdStr, output: output, err: err}
		}
		return commandOutputMsg{cmd: cmdStr, output: output}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyEnter:
			if len(m.suggestions) == 0 || m.input == "" {
				return m, nil
			}
			cmdStr := m.suggestions[m.selected].Cmd
			m.output = append(m.output, promptStyle.Render("$ ")+cmdStyle.Render(cmdStr))
			m.input = ""
			m.suggestions = nil
			m.selected = 0
			m.lastInput = ""
			m.lastErr = ""
			return m, runCommand(cmdStr)

		case tea.KeyUp:
			if m.selected > 0 {
				m.selected--
			}
			return m, nil

		case tea.KeyDown:
			if m.selected < len(m.suggestions)-1 {
				m.selected++
			}
			return m, nil

		case tea.KeyBackspace, tea.KeyDelete:
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
				m.lastErr = ""
				return m, debounceTranslate(m.input)
			}
			return m, nil

		case tea.KeySpace:
			m.input += " "
			m.lastErr = ""
			return m, debounceTranslate(m.input)

		case tea.KeyRunes:
			m.input += string(msg.Runes)
			m.lastErr = ""
			return m, debounceTranslate(m.input)
		}

	case debounceTickMsg:
		// Only fire if input hasn't changed since the tick was scheduled
		if msg.input == m.input && m.input != "" {
			m.translating = true
			return m, doTranslate(m.client, m.input)
		}
		return m, nil

	case translationResultMsg:
		m.translating = false
		if msg.input != m.input {
			// stale result, ignore
			return m, nil
		}
		if msg.err != nil {
			m.lastErr = msg.err.Error()
			m.suggestions = nil
		} else {
			m.suggestions = msg.suggestions
			m.selected = 0
			m.lastErr = ""
		}
		m.lastInput = msg.input
		return m, nil

	case commandOutputMsg:
		if msg.output != "" {
			m.output = append(m.output, outputStyle.Render(msg.output))
		}
		if msg.err != nil {
			m.output = append(m.output, errorStyle.Render("exit: "+msg.err.Error()))
		}
		return m, nil
	}

	return m, nil
}

func (m model) View() string {
	if m.width == 0 {
		return ""
	}

	var sb strings.Builder

	// --- translation / suggestion area ---
	var translationArea string
	if m.lastErr != "" {
		translationArea = errorStyle.Render("error: "+m.lastErr) + "\n"
	} else if m.translating {
		translationArea = explanStyle.Render("…") + "\n"
	} else if len(m.suggestions) == 1 {
		s := m.suggestions[0]
		translationArea = promptStyle.Render("$ ") + cmdStyle.Render(s.Cmd) + "\n"
	} else if len(m.suggestions) > 1 {
		var lines []string
		for i, s := range m.suggestions {
			prefix := "  "
			var cmdRendered string
			if i == m.selected {
				prefix = promptStyle.Render("▶ ")
				cmdRendered = selectedCmdStyle.Render("$ " + s.Cmd)
			} else {
				cmdRendered = dividerStyle.Render("$ ") + cmdStyle.Render(s.Cmd)
			}
			lines = append(lines, prefix+cmdRendered+"  "+explanStyle.Render("("+s.Explanation+")"))
		}
		translationArea = strings.Join(lines, "\n") + "\n"
	} else if m.input != "" {
		translationArea = explanStyle.Render("…") + "\n"
	}

	// --- output history ---
	// Calculate how many lines we have for output
	translationLines := strings.Count(translationArea, "\n")
	inputLine := 1
	dividerLine := 1
	reserved := translationLines + inputLine + dividerLine + 1

	outputLines := m.height - reserved
	if outputLines < 0 {
		outputLines = 0
	}

	// flatten and trim output history to fit
	var allOutputLines []string
	for _, block := range m.output {
		for _, line := range strings.Split(block, "\n") {
			allOutputLines = append(allOutputLines, line)
		}
	}
	if len(allOutputLines) > outputLines {
		allOutputLines = allOutputLines[len(allOutputLines)-outputLines:]
	}

	// pad output area to fill space
	for len(allOutputLines) < outputLines {
		allOutputLines = append([]string{""}, allOutputLines...)
	}

	for _, line := range allOutputLines {
		sb.WriteString(outputStyle.Render(line) + "\n")
	}

	// divider
	sb.WriteString(dividerStyle.Render(strings.Repeat("─", m.width)) + "\n")

	// translation
	sb.WriteString(translationArea)

	// input line with blinking cursor simulation
	sb.WriteString(promptStyle.Render("> ") + inputStyle.Render(m.input) + cursorStyle.Render("█"))

	return sb.String()
}
