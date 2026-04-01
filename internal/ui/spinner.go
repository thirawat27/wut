package ui

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

type spinnerDoneMsg struct {
	err error
}

type spinnerModel struct {
	spinner  spinner.Model
	text     string
	quitting bool
	done     bool
	task     func() error
	err      error
}

func (m spinnerModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			if m.task == nil {
				return spinnerDoneMsg{}
			}
			return spinnerDoneMsg{err: m.task()}
		},
	)
}

func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.quitting = true
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case spinnerDoneMsg:
		m.done = true
		m.err = msg.err
		return m, tea.Quit
	}
	return m, nil
}

func (m spinnerModel) View() string {
	if m.quitting {
		return "\n"
	}
	if m.done {
		return ""
	}
	return fmt.Sprintf("\n %s %s\n", m.spinner.View(), lipgloss.NewStyle().Foreground(lipgloss.Color("#90E0EF")).Render(m.text))
}

// RunWithSpinner runs a long-running function with a visual spinner
func RunWithSpinner(text string, f func() error) error {
	if os.Getenv("WUT_NO_SPINNER") == "true" || !term.IsTerminal(int(os.Stdout.Fd())) {
		return f()
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF70A6"))

	m := spinnerModel{
		spinner: s,
		text:    text,
		task:    f,
	}

	p := tea.NewProgram(m)
	model, err := p.Run()
	if err != nil {
		return err
	}

	finalModel, ok := model.(spinnerModel)
	if !ok {
		return nil
	}

	return finalModel.err
}
