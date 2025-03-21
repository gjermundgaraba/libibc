package tui

import (
	"go.uber.org/zap/zapcore"
)

var _ zapcore.WriteSyncer = &tuiLogWriter{}

// tuiLogWriter implements zapcore.WriteSyncer interface to redirect logs to TUI
type tuiLogWriter struct {
	tui *Tui
}

// Write implements io.Writer interface
func (w *tuiLogWriter) Write(p []byte) (n int, err error) {
	w.tui.mutex.Lock()
	defer w.tui.mutex.Unlock()

	// Add log entry to the TUI
	w.tui.AddLogEntry(string(p))

	// Write to the actual log file
	_, err = w.tui.logFile.Write(p)
	if err != nil {
		return 0, err
	}

	return len(p), nil
}

// Sync implements zapcore.WriteSyncer interface
func (w *tuiLogWriter) Sync() error {
	w.tui.mutex.Lock()
	defer w.tui.mutex.Unlock()

	return w.tui.logFile.Sync()
}