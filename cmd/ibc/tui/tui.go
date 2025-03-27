package tui

import (
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gjermundgaraba/libibc/cmd/ibc/logging"
)

var (
	titleStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Right = "├"
		return lipgloss.NewStyle().BorderStyle(b).Padding(0, 1)
	}()

	infoStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Left = "┤"
		return titleStyle.BorderStyle(b)
	}()

	statusStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFDF5")).
		Background(lipgloss.Color("#25A065")).
		Padding(0, 1)

	spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))
)

type Tui struct {
	*Model
	program *tea.Program
	mutex   sync.Mutex
}

type logUpdate struct {
	content string
}

type statusUpdate struct {
	content string
}

type errorStatusUpdate struct {
	content string
}

type progressUpdate struct {
	percent int
}

type addStatusModelUpdate struct {
	status *StatusModel
}

func NewTui(logWriter *logging.IBCLogWriter, initLog string, initStatus string) *Tui {
	mainStatus := NewStatusModel(initStatus)

	model := NewModel(initLog, mainStatus)

	tuiInstance := &Tui{
		Model: model,
		mutex: sync.Mutex{},
	}

	logWriter.AddExtraLogger(func(entry string) {
		tuiInstance.AddLogEntry(entry)
	})

	// Initialize the program here but don't run it yet
	tuiInstance.program = tea.NewProgram(
		tuiInstance.Model,
		// Don't use alt screen to allow text selection
	)

	return tuiInstance
}

// AddStatusModel adds a new status model to the UI
func (t *Tui) AddStatusModel(status *StatusModel) {
	t.program.Send(addStatusModelUpdate{status: status})
}

// UpdateMainStatus updates the status text in the TUI
func (t *Tui) UpdateMainStatus(status string) {
	t.program.Send(statusUpdate{content: status})
}

// UpdateMainErrorStatus updates the status text in the TUI and marks it as an error
func (t *Tui) UpdateMainErrorStatus(status string) {
	t.program.Send(errorStatusUpdate{content: status})
}

// UpdateProgress updates the progress percentage in the TUI
func (t *Tui) UpdateProgress(percent int) {
	t.program.Send(progressUpdate{percent: percent})
}

// AddLogEntry adds a new entry to the log area
func (t *Tui) AddLogEntry(entry string) {
	t.program.Send(logUpdate{content: entry})
}

// Run starts the TUI program and blocks until it exits
func (t *Tui) Run() error {
	_, err := t.program.Run()
	return err
}