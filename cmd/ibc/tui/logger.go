package tui

import (
	"os"

	"go.uber.org/zap/zapcore"
)

var _ zapcore.WriteSyncer = &tuiLogWriter{}

// tuiLogWriter implements zapcore.WriteSyncer interface to redirect logs to TUI
type tuiLogWriter struct {
	tui *Tui
}

// Write implements io.Writer interface
func (w *tuiLogWriter) Write(p []byte) (n int, err error) {
	w.tui.AddLogEntry(string(p))

	// also append to file
	file := "ibc.log"
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	if _, err := f.Write(p); err != nil {
		return 0, err
	}

	return len(p), nil
}

// Sync implements zapcore.WriteSyncer interface
func (w *tuiLogWriter) Sync() error {
	return nil
}
