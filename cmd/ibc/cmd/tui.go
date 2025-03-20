package cmd

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gjermundgaraba/libibc/cmd/ibc/tui"
	"github.com/spf13/cobra"
)

func tuiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Start the TUI",
		RunE: func(cmd *cobra.Command, args []string) error {
			tui := tui.NewTui("init log", "init status")

			go func() {
				for {
					time.Sleep(10 * time.Second)
					tui.UpdateStatus(fmt.Sprintf("System status check: %s", time.Now().Format(time.RFC3339)))
				}
			}()

			go func() {
				status := 1
				for {
					time.Sleep(1 * time.Second)
					tui.UpdateProgress(status)
					status++

					if status > 100 {
						status = 0
					}
				}
			}()

			go func() {
				for {
					time.Sleep(1 * time.Second)
					tui.AddLogEntry(fmt.Sprintf("Log entry: %s", time.Now().Format(time.RFC3339)))
				}
			}()

			program := tea.NewProgram(
				tui,
				tea.WithAltScreen(),       // use the full size of the terminal in its "alternate screen buffer"
				tea.WithMouseCellMotion(), // turn on mouse support so we can track the mouse wheel
			)

			if _, err := program.Run(); err != nil {
				fmt.Println("could not run program:", err)
				os.Exit(1)
			}

			return nil
		},
	}
}
