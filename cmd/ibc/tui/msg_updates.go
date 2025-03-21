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

// Command that polls all channel updates at once
func pollAllChannels(logCh chan string, statusCh chan string, progressCh chan int, statusModelCh chan *StatusModel) tea.Cmd {
	return func() tea.Msg {
		// Check all channels in sequence with non-blocking selects
		
		// Log channel
		select {
		case msg := <-logCh:
			return contentMsg(msg)
		default:
		}
		
		// Status channel
		select {
		case msg := <-statusCh:
			return statusMsg(msg)
		default:
		}
		
		// Progress channel
		select {
		case msg := <-progressCh:
			return progressMsg(msg)
		default:
		}
		
		// Status model channel
		select {
		case msg := <-statusModelCh:
			return addStatusModelMsg(msg)
		default:
		}
		
		// If no messages, return nil to continue
		return nil
	}
}

// Tick function to periodically trigger updates
type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(time.Millisecond * 250, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}