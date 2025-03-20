package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type StatusModel struct {
	ready bool

	spinner    spinner.Model
	progress   progress.Model
	statusText string
	percent    int

	// Channels to receive updates
	statusChan   chan string
	progressChan chan int
}

func NewStatusModel(initStatus string) StatusModel {
	statusChan := make(chan string)
	progressChan := make(chan int)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	p := progress.New(progress.WithScaledGradient("#FF7CCB", "#FDFF8C"))

	return StatusModel{
		spinner:    s,
		progress:   p,
		statusText: initStatus,
		percent:    0,

		statusChan:   statusChan,
		progressChan: progressChan,
	}
}

// UpdateStatus updates the status text in the TUI
func (m StatusModel) UpdateStatus(status string) {
	m.statusChan <- status
}

// UpdateProgress updates the progress bar with a value between 1-100
func (m StatusModel) UpdateProgress(percent int) {
	m.progressChan <- percent
}

func (m StatusModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,                          // Start the spinner animation
		checkForStatusUpdates(m.statusChan),     // Check for status updates
		checkForProgressUpdates(m.progressChan), // Check for progress updates
	)
}

func (m StatusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case statusMsg:
		// Update the status text
		m.statusText = string(msg)

		// Check for the next status update
		cmd = checkForStatusUpdates(m.statusChan)
		cmds = append(cmds, cmd)

	case progressMsg:
		// Update the progress percentage
		percent := int(msg)
		if percent >= 0 && percent <= 100 {
			m.percent = percent
			// Update the progress bar
			cmd = m.progress.SetPercent(float64(percent) / 100)
			cmds = append(cmds, cmd)
		}

		// Check for the next progress update
		cmd = checkForProgressUpdates(m.progressChan)
		cmds = append(cmds, cmd)

	case spinner.TickMsg:
		// Update the spinner animation
		var spinnerCmd tea.Cmd
		m.spinner, spinnerCmd = m.spinner.Update(msg)
		cmds = append(cmds, spinnerCmd)

	case progress.FrameMsg:
		// Update the progress bar animation
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		cmds = append(cmds, cmd)
	case tea.WindowSizeMsg:
		m.progress.Width = msg.Width - 2
	}

	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m StatusModel) View() string {
	// Build status bar with spinner and text
	statusBar := lipgloss.JoinHorizontal(
		lipgloss.Center,
		m.spinner.View(),
		" ",
		statusStyle.Render(m.statusText),
	)

	// Add progress bar if needed
	return fmt.Sprintf("%s\n%s",
		statusBar,
		m.progress.View(),
	)
}
