package main

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- styles ---
var (
	outputStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	dividerStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	cmdStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	selectedCmdStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true)
	explanStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	promptStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
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
	ti          textinput.Model
	suggestions []Suggestion
	selected    int
	translating bool
	spinner     spinner.Model
	output      []string
	lastInput   string
	lastErr     string
}

func newModel(client *anthropic.Client) model {
	ti := textinput.New()
	ti.Prompt = promptStyle.Render("> ")
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	ti.Cursor.Style = cursorStyle
	ti.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = explanStyle

	return model{client: client, ti: ti, spinner: sp}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

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
	var cmds []tea.Cmd

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
			if len(m.suggestions) == 0 || m.ti.Value() == "" {
				return m, nil
			}
			cmdStr := m.suggestions[m.selected].Cmd
			m.output = append(m.output, promptStyle.Render("$ ")+cmdStyle.Render(cmdStr))
			m.ti.SetValue("")
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
		}

		prev := m.ti.Value()
		var tiCmd tea.Cmd
		m.ti, tiCmd = m.ti.Update(msg)
		cmds = append(cmds, tiCmd)
		if m.ti.Value() != prev {
			m.lastErr = ""
			cmds = append(cmds, debounceTranslate(m.ti.Value()))
		}
		return m, tea.Batch(cmds...)

	case debounceTickMsg:
		if msg.input == m.ti.Value() && m.ti.Value() != "" {
			m.translating = true
			return m, doTranslate(m.client, m.ti.Value())
		}
		return m, nil

	case translationResultMsg:
		m.translating = false
		if msg.input != m.ti.Value() {
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

	case spinner.TickMsg:
		var spCmd tea.Cmd
		m.spinner, spCmd = m.spinner.Update(msg)
		return m, spCmd
	}

	var tiCmd tea.Cmd
	m.ti, tiCmd = m.ti.Update(msg)
	cmds = append(cmds, tiCmd)
	return m, tea.Batch(cmds...)
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
		translationArea = m.spinner.View() + explanStyle.Render(" translating…") + "\n"
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
	} else if m.ti.Value() != "" {
		translationArea = explanStyle.Render("…") + "\n"
	}

	// --- output history ---
	translationLines := strings.Count(translationArea, "\n")
	reserved := translationLines + 1 /* input */ + 1 /* divider */ + 1

	outputLines := m.height - reserved
	if outputLines < 0 {
		outputLines = 0
	}

	var allOutputLines []string
	for _, block := range m.output {
		for _, line := range strings.Split(block, "\n") {
			allOutputLines = append(allOutputLines, line)
		}
	}
	if len(allOutputLines) > outputLines {
		allOutputLines = allOutputLines[len(allOutputLines)-outputLines:]
	}
	for len(allOutputLines) < outputLines {
		allOutputLines = append([]string{""}, allOutputLines...)
	}

	for _, line := range allOutputLines {
		sb.WriteString(outputStyle.Render(line) + "\n")
	}

	sb.WriteString(dividerStyle.Render(strings.Repeat("─", m.width)) + "\n")
	sb.WriteString(translationArea)
	sb.WriteString(m.ti.View())

	return sb.String()
}
