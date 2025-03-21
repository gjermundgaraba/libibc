package cmd

import (
	"fmt"
	"math/big"
	"os"

	"github.com/gjermundgaraba/libibc/cmd/ibc/loadscript"
	"github.com/gjermundgaraba/libibc/cmd/ibc/tui"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func scriptCmd() *cobra.Command {
	var (
		numPacketsPerWallet int
		transferAmount      int

		chainAId              string
		chainAClientId        string
		chainADenom           string
		chainARelayerWalletId string

		chainBId              string
		chainBClientId        string
		chainBDenom           string
		chainBRelayerWalletId string
	)

	cmd := &cobra.Command{
		Use:   "script",
		Short: "Run a script",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			tuiInstance := tui.NewTui("Starting script", "Initializing")

			network, err := cfg.ToNetwork(ctx, tuiInstance.GetLogger())
			if err != nil {
				return errors.Wrap(err, "failed to build network")
			}

			transferAmountBig := big.NewInt(int64(transferAmount))
			chainA := network.GetChain(chainAId)
			chainB := network.GetChain(chainBId)

			chainARelayerWallet, err := chainA.GetWallet(chainARelayerWalletId)
			if err != nil {
				return errors.Wrapf(err, "failed to get wallet %s", chainARelayerWalletId)
			}

			chainBRelayerWallet, err := chainB.GetWallet(chainBRelayerWalletId)
			if err != nil {
				return errors.Wrapf(err, "failed to get wallet %s", chainBRelayerWalletId)
			}

			chainBWallets := chainB.GetWallets()
			chainAWallets := chainA.GetWallets()

			// Limit wallets for testing
			if len(chainBWallets) > 10 {
				chainBWallets = chainBWallets[:10]
			}
			if len(chainAWallets) > 10 {
				chainAWallets = chainAWallets[:10]
			}

			if len(chainBWallets) != len(chainAWallets) {
				return errors.Errorf("wallets length mismatch: %d != %d", len(chainBWallets), len(chainAWallets))
			}

			go func() {
				defer func() {
					if r := recover(); r != nil {
						tuiInstance.GetLogger().Error("Panic", zap.Any("panic", r))
						tuiInstance.UpdateMainErrorStatus(fmt.Sprintf("Panic: %v", r))
					}
				}()

				if err := loadscript.RunScript(
					ctx,
					tuiInstance,
					network,
					chainA,
					chainAClientId,
					chainADenom,
					chainAWallets,
					chainARelayerWallet,
					chainB,
					chainBClientId,
					chainBDenom,
					chainBWallets,
					chainBRelayerWallet,
					transferAmountBig,
					numPacketsPerWallet,
				); err != nil {
					tuiInstance.GetLogger().Error("Script failed", zap.Error(err))
					tuiInstance.UpdateMainErrorStatus(fmt.Sprintf("Error: %s", err.Error()))
				}
			}()

			if err := tuiInstance.Run(); err != nil {
				fmt.Println("Error running TUI program:", err)
				os.Exit(1)
			}

			// Ensure resources are properly closed
			defer tuiInstance.Close()

			return nil
		},
	}

	cmd.Flags().IntVar(&numPacketsPerWallet, "packets-per-wallet", 5, "Number of packets to send per wallet")
	cmd.Flags().IntVar(&transferAmount, "transfer-amount", 100, "Amount to transfer")
	cmd.Flags().StringVar(&chainAId, "chain-a-id", "11155111", "Chain A ID")
	cmd.Flags().StringVar(&chainAClientId, "chain-a-client-id", "plz-last-hub-devnet-69", "Chain A client ID")
	cmd.Flags().StringVar(&chainADenom, "chain-a-denom", "0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14", "Chain A denom")
	cmd.Flags().StringVar(&chainARelayerWalletId, "chain-a-relayer-wallet-id", "eth-relayer", "Chain A relayer wallet ID")
	cmd.Flags().StringVar(&chainBId, "chain-b-id", "eureka-hub-dev-6", "Chain B ID")
	cmd.Flags().StringVar(&chainBClientId, "chain-b-client-id", "08-wasm-2", "Chain B client ID")
	cmd.Flags().StringVar(&chainBDenom, "chain-b-denom", "uatom", "Chain B denom")
	cmd.Flags().StringVar(&chainBRelayerWalletId, "chain-b-relayer-wallet-id", "cosmos-relayer", "Chain B relayer wallet ID")

	return cmd
}
