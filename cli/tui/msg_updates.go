package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Message containing new content to add
type contentMsg string

// Message containing new status text
type statusMsg string

// Message containing new progress percentage (1-100)
type progressMsg int

type addStatusModelMsg *StatusModel

// Tick function to periodically trigger updates
type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(time.Millisecond*250, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

