package cmd

import (
	"fmt"

	"github.com/gjermundgaraba/libibc/cmd/ibc/config"
	"github.com/gjermundgaraba/libibc/cmd/ibc/logging"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	networkConfigPath string
	cfg               *config.Config
	logLevel          string
	extraGwei         int64

	logger    *zap.Logger
	logWriter *logging.IBCLogWriter
)

func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "ibc",
		Short: "IBC CLI tool",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			var err error
			cfg, err = config.LoadConfig(networkConfigPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			logger, logWriter = logging.NewIBCLogger(logLevel)
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(&networkConfigPath, "config", "config.toml", "config file path")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().Int64Var(&extraGwei, "extra-gwei", 0, "extra gwei to add to gas price")

	rootCmd.AddCommand(
		traceCmd(),
		scriptCmd(),
		relayCmd(),
		distributeCmd(),
		generateWalletCmd(),
		balanceCmd(),
		transferCmd(),
		apiCmd(),
		clientMigrateMsgCmd(),
		localnetCmd(),
	)

	return rootCmd
}
