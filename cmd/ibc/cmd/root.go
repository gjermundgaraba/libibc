package cmd

import (
	"fmt"

	"github.com/gjermundgaraba/libibc/cmd/ibc/config"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	cfg     *config.Config
)

func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "ibc",
		Short: "IBC CLI tool",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip loading config for help command
			// if cmd.Name() == "help" {
			// 	return nil
			// }

			var err error
			cfg, err = config.LoadConfig(cfgFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "config.toml", "config file path")

	rootCmd.AddCommand(
		traceCmd(),
		scriptCmd(),
	)

	return rootCmd
}

