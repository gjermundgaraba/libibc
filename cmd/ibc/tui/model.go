package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model is the main UI model for the TUI
type Model struct {
	logs     string
	ready    bool
	viewport viewport.Model

	// Status section components
	spinner      spinner.Model
	mainStatus   *StatusModel
	statusModels []*StatusModel
}

// NewModel creates a new model with initial state
func NewModel(initialLog string, mainStatus *StatusModel) *Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	return &Model{
		logs:         initialLog,
		ready:        false,
		spinner:      s,
		mainStatus:   mainStatus,
		statusModels: []*StatusModel{},
	}
}

// Init initializes the model and returns the first batch of commands to run
func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.spinner.Tick,
		m.mainStatus.Init(),
		tick(),
	}

	for _, statusModel := range m.statusModels {
		cmds = append(cmds, statusModel.Init())
	}

	return tea.Batch(cmds...)
}

// Update updates the model based on messages received
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if k := msg.String(); k == "ctrl+c" || k == "q" || k == "esc" {
			return m, tea.Quit
		}

	case logUpdate:
		// Add the new content to our existing content
		m.logs += "\n" + msg.content
		if m.ready {
			m.viewport.SetContent(m.logs)
			// Scroll to the bottom to see new content
			m.viewport.GotoBottom()
		}

	case statusUpdate:
		// Update main status
		m.mainStatus.UpdateStatus(msg.content)
		
	case errorStatusUpdate:
		// Update main status as error
		m.mainStatus.UpdateErrorStatus(msg.content)

	case progressUpdate:
		// Update main status progress
		m.mainStatus.UpdateProgress(msg.percent)

	case addStatusModelUpdate:
		// Add a new status model
		m.statusModels = append(m.statusModels, msg.status)

	case spinner.TickMsg:
		// Update the spinner animation
		var spinnerCmd tea.Cmd
		m.spinner, spinnerCmd = m.spinner.Update(msg)
		cmds = append(cmds, spinnerCmd)

	case tickMsg:
		// Schedule the next tick
		cmds = append(cmds, tick())

	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(m.headerView())
		footerHeight := lipgloss.Height(m.footerView())
		statusHeight := lipgloss.Height(m.statusView())
		verticalMarginHeight := headerHeight + footerHeight + statusHeight

		if !m.ready {
			// Initialize the viewport
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight
			m.viewport.SetContent(m.logs)

			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
		}
	}

	// Update the viewport
	viewportModel, cmd := m.viewport.Update(msg)
	m.viewport = viewportModel
	cmds = append(cmds, cmd)

	// Update the main status
	statusModel, cmd := m.mainStatus.Update(msg)
	m.mainStatus = statusModel.(*StatusModel)
	cmds = append(cmds, cmd)

	// Update all status models
	for i, statusModel := range m.statusModels {
		updatedModel, cmd := statusModel.Update(msg)
		m.statusModels[i] = updatedModel.(*StatusModel)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View renders the current UI state
func (m *Model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}
	return fmt.Sprintf("%s\n%s\n%s\n%s",
		m.headerView(),
		m.viewport.View(),
		m.footerView(),
		m.statusView(),
	)
}

// max returns the larger of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m *Model) headerView() string {
	title := titleStyle.Render("Script Runner")
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(title)))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (m *Model) footerView() string {
	info := infoStyle.Render(fmt.Sprintf("Scroll %3.f%%", m.viewport.ScrollPercent()*100))
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(info)))
	return lipgloss.JoinHorizontal(lipgloss.Center, line, info)
}

func (m *Model) statusView() string {
	mainStatus := m.mainStatus.View()

	horizontalSeparator := strings.Repeat("─", m.viewport.Width)

	var statusViews []string
	for _, statusModel := range m.statusModels {
		statusViews = append(statusViews, statusModel.View())
	}

	return lipgloss.JoinVertical(lipgloss.Top,
		mainStatus,
		horizontalSeparator,
		strings.Join(statusViews, "\n"),
	)
}