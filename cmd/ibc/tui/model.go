package tui

import (
	"fmt"
	"strings"

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
	spinner      spinner.Model
	mainStatus   StatusModel
	statusModels []StatusModel

	// Channels to receive updates
	logChan         chan string
	statusModelChan chan StatusModel
}

func (m Model) Init() tea.Cmd {
	var batch []tea.Cmd
	batch = append(batch, m.spinner.Tick)
	batch = append(batch, m.mainStatus.Init())
	batch = append(batch, checkForContentUpdates(m.logChan))
	batch = append(batch, checkForStatusModelUpdates(m.statusModelChan))
	for _, statusModel := range m.statusModels {
		batch = append(batch, statusModel.Init())
	}

	return tea.Batch(batch...)
}

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

	case addStatusModelMsg:
		statusModel := StatusModel(msg)
		m.statusModels = append(m.statusModels, statusModel)

		cmd = statusModel.Init()
		cmds = append(cmds, cmd)

	case spinner.TickMsg:
		// Update the spinner animation
		var spinnerCmd tea.Cmd
		m.spinner, spinnerCmd = m.spinner.Update(msg)
		cmds = append(cmds, spinnerCmd)

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

			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
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
