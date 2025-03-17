package cmd

import (
	"fmt"

	"github.com/gjermundgaraba/libibc/cmd/ibc/config"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	cfgFile string
	cfg     *config.Config
	logger  *zap.Logger
)

func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "ibc",
		Short: "IBC CLI tool",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			var err error
			logger, err = zap.NewDevelopment()
			if err != nil {
				return errors.Wrap(err, "failed to initialize logger")
			}

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
		relayCmd(),
	)

	return rootCmd
}

