package tui

import (
	"os"
	"sync"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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
	logFile *os.File
	mutex   sync.Mutex
	logger  *zap.Logger
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

func NewTui(initLog string, initStatus string) *Tui {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	mainStatus := NewStatusModel(initStatus)

	model := NewModel(initLog, mainStatus)

	// Create an actual file for logging
	file, err := os.OpenFile("ibc.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}

	tuiInstance := &Tui{
		Model:   model,
		logFile: file,
		mutex:   sync.Mutex{},
	}

	// Create a custom zap logger that redirects to the TUI
	encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoderConfig.TimeKey = ""
	encoderConfig.LevelKey = ""
	encoderConfig.NameKey = ""
	encoderConfig.CallerKey = ""

	tuiCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.AddSync(&tuiLogWriter{tui: tuiInstance}),
		zap.InfoLevel,
	)
	tuiInstance.logger = zap.New(tuiCore)

	// Initialize the program here but don't run it yet
	tuiInstance.program = tea.NewProgram(
		tuiInstance.Model,
		tea.WithAltScreen(),       // use the full size of the terminal in its "alternate screen buffer"
		tea.WithMouseCellMotion(), // turn on mouse support so we can track the mouse wheel
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

// GetLogger returns the TUI's logger for external use
func (t *Tui) GetLogger() *zap.Logger {
	return t.logger
}

// Run starts the TUI program and blocks until it exits
func (t *Tui) Run() error {
	_, err := t.program.Run()
	return err
}

// Close closes any resources used by the TUI
func (t *Tui) Close() {
	if t.logFile != nil {
		t.logFile.Close()
	}
}