package cmd

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type errMsg error

type model struct {
	spinner     spinner.Model
	title       string
	align       string
	quitting    bool
	aborted     bool
	status      int
	output      string
	showOutput  bool
	showError   bool
	timeout     time.Duration
	hasTimeout  bool
	name        string
	instruction string
	done        bool
	function    func() error
}

type functionGeneratedMsg struct{}

func functionModel(title string, function func() error) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return model{
		spinner:    s,
		title:      title,
		align:      "left",
		function:   function,
		timeout:    30 * time.Second,
		hasTimeout: true,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			err := m.function()
			if err != nil {
				return errMsg(err)
			}
			return functionGeneratedMsg{}
		},
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	case errMsg:
		m.showError = true
		m.output = msg.Error()
		return m, tea.Quit
	case functionGeneratedMsg:
		m.done = true
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.quitting {
		return "Cancelled\n"
	}
	if m.showError {
		return fmt.Sprintf("\nError: %s\n", m.output)
	}
	if m.done {
		return fmt.Sprintf("\n%s completed successfully\n", m.title)
	}
	var header string
	if m.align == "left" {
		header = m.spinner.View() + " " + m.title
	} else {
		header = m.title + " " + m.spinner.View()
	}
	return header
}
