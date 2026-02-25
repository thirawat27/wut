package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type spinnerModel struct {
	spinner  spinner.Model
	text     string
	quitting bool
	done     bool
}

func (m spinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
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
	case string:
		if msg == "done" {
			m.done = true
			return m, tea.Quit
		}
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
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF70A6"))

	m := spinnerModel{
		spinner: s,
		text:    text,
	}

	p := tea.NewProgram(m)

	errChan := make(chan error, 1)

	go func() {
		// Run the actual function
		errChan <- f()
		// Tell bubbletea we are done
		p.Send("done")
	}()

	_, err := p.Run()
	if err != nil {
		return err
	}

	return <-errChan
}
