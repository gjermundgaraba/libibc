package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// StatusModel represents a status display in the UI
type StatusModel struct {
	spinner    spinner.Model
	progress   progress.Model
	statusText string
	percent    int
	hasError   bool
}

// NewStatusModel creates a new status model with initial state
func NewStatusModel(initialStatus string) *StatusModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	p := progress.New(progress.WithScaledGradient("#FF7CCB", "#FDFF8C"))

	return &StatusModel{
		spinner:    s,
		progress:   p,
		statusText: initialStatus,
		percent:    0,
		hasError:   false,
	}
}

// UpdateStatus updates the status text
func (m *StatusModel) UpdateStatus(status string) {
	m.statusText = status
	// Reset error state when status is updated
	m.hasError = false
}

// UpdateErrorStatus updates the status text and marks it as an error
func (m *StatusModel) UpdateErrorStatus(status string) {
	m.statusText = status
	m.hasError = true
}

// UpdateProgress updates the progress percentage
func (m *StatusModel) UpdateProgress(percent int) {
	if percent >= 0 && percent <= 100 {
		m.percent = percent
	}
}

// Init initializes the model
func (m *StatusModel) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update handles messages and updates the model state
func (m *StatusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
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

	// Always update the progress bar when needed
	if m.progress.Percent() != float64(m.percent)/100 {
		cmds = append(cmds, m.progress.SetPercent(float64(m.percent)/100))
	}

	return m, tea.Batch(cmds...)
}

// View renders the status model
func (m *StatusModel) View() string {
	// Build status bar with spinner and text
	var textStyle lipgloss.Style
	if m.hasError {
		textStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#B22222")).  // Red for errors
			Padding(0, 1)
	} else {
		textStyle = statusStyle
	}

	statusBar := lipgloss.JoinHorizontal(
		lipgloss.Center,
		m.spinner.View(),
		" ",
		textStyle.Render(m.statusText),
	)

	// Add progress bar
	return fmt.Sprintf("%s\n%s",
		statusBar,
		m.progress.View(),
	)
}