package cmd

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type streamHandler struct {
	spinner    spinner.Model
	content    strings.Builder
	isThinking bool
	updateChan chan tea.Msg
}

func newStreamHandler(updateChan chan tea.Msg) *streamHandler {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return &streamHandler{
		spinner:    s,
		updateChan: updateChan,
	}
}

func (sh *streamHandler) processStream(content string) {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.Contains(line, "<thinking>") {
			sh.isThinking = true
			sh.updateChan <- spinner.TickMsg{}
		} else if strings.Contains(line, "</thinking>") {
			sh.isThinking = false
			sh.updateChan <- spinner.TickMsg{}
		} else {
			sh.content.WriteString(line + "\n")
			if !sh.isThinking {
				sh.updateChan <- line
			}
		}
	}
}
