package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	logs     string
	ready    bool
	viewport viewport.Model

	// Status section components
	spinner    spinner.Model
	progress   progress.Model
	statusText string
	percent    int

	// Channels to receive updates
	logChan    chan string
	statusChan chan string
	progChan   chan int
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,                      // Start the spinner animation
		checkForContentUpdates(m.logChan),   // Check for content updates
		checkForStatusUpdates(m.statusChan), // Check for status updates
		checkForProgressUpdates(m.progChan), // Check for progress updates
	)
}

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

// Message containing new content to add
type contentMsg string

// Message containing new status text
type statusMsg string

// Message containing new progress percentage (1-100)
type progressMsg int

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case contentMsg:
		// Add the new content to our existing content
		m.logs += "\n" + string(msg)
		if m.ready {
			m.viewport.SetContent(m.logs)
			// Scroll to the bottom to see new content
			m.viewport.GotoBottom()
		}

		// Check for the next content update
		cmd = checkForContentUpdates(m.logChan)
		cmds = append(cmds, cmd)

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
		cmd = checkForProgressUpdates(m.progChan)
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

	case tea.KeyMsg:
		if k := msg.String(); k == "ctrl+c" || k == "q" || k == "esc" {
			return m, tea.Quit
		}

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

			// Set progress bar width
			m.progress.Width = msg.Width - 2

			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
			m.progress.Width = msg.Width - 2
		}
	}

	// Handle keyboard and mouse events in the viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
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

func (m Model) headerView() string {
	title := titleStyle.Render("Script Runner")
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(title)))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (m Model) footerView() string {
	info := infoStyle.Render(fmt.Sprintf("Scroll %3.f%%", m.viewport.ScrollPercent()*100))
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(info)))
	return lipgloss.JoinHorizontal(lipgloss.Center, line, info)
}

func (m Model) statusView() string {
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

// func max(a, b int) int {
// 	if a > b {
// 		return a
// 	}
// 	return b
// }
