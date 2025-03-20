package tui

import (
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
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

// tuiLogWriter implements zapcore.WriteSyncer interface to redirect logs to TUI
type tuiLogWriter struct {
	tui *Tui
}

// Write implements io.Writer interface
func (w *tuiLogWriter) Write(p []byte) (n int, err error) {
	w.tui.AddLogEntry(string(p))
	return len(p), nil
}

// Sync implements zapcore.WriteSyncer interface
func (w *tuiLogWriter) Sync() error {
	return nil
}

type Tui struct {
	Model

	statusChannel   chan string
	progressChannel chan int
	logChannel      chan string
	logger          *zap.Logger
}

func NewTui(initLog string, initStatus string) *Tui {
	logChannel := make(chan string)
	statusChannel := make(chan string)
	progressChannel := make(chan int)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	p := progress.New(progress.WithScaledGradient("#FF7CCB", "#FDFF8C"))

	m := Model{
		logs:       initLog,
		spinner:    s,
		progress:   p,
		statusText: initStatus,
		percent:    0,
		logChan:    logChannel,      // Use the global channel
		statusChan: statusChannel,   // Use the global channel
		progChan:   progressChannel, // Use the global channel
	}

	tuiInstance := &Tui{
		Model:           m,
		statusChannel:   statusChannel,
		progressChannel: progressChannel,
		logChannel:      logChannel,
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

	return tuiInstance
}

// UpdateStatus updates the status text in the TUI
func (t *Tui) UpdateStatus(status string) {
	t.statusChannel <- status
}

// UpdateProgress updates the progress bar with a value between 1-100
func (t *Tui) UpdateProgress(percent int) {
	t.progressChannel <- percent
}

// AddLogEntry adds a new entry to the log area
func (t *Tui) AddLogEntry(entry string) {
	t.logChannel <- entry
}

// GetLogger returns the TUI's logger for external use
func (t *Tui) GetLogger() *zap.Logger {
	return t.logger
}
