package tui

import tea "github.com/charmbracelet/bubbletea"

// Message containing new content to add
type contentMsg string

// Message containing new status text
type statusMsg string

// Message containing new progress percentage (1-100)
type progressMsg int

type addStatusModelMsg StatusModel

// Command that checks for content updates from the goroutine
func checkForContentUpdates(ch chan string) tea.Cmd {
	return func() tea.Msg {
		return contentMsg(<-ch)
	}
}

// Command that checks for status updates from the goroutine
func checkForStatusUpdates(ch chan string) tea.Cmd {
	return func() tea.Msg {
		return statusMsg(<-ch)
	}
}

// Command that checks for progress updates from the goroutine
func checkForProgressUpdates(ch chan int) tea.Cmd {
	return func() tea.Msg {
		return progressMsg(<-ch)
	}
}

// Command that checks for status model updates from the goroutine
func checkForStatusModelUpdates(ch chan StatusModel) tea.Cmd {
	return func() tea.Msg {
		return addStatusModelMsg(<-ch)
	}
}
