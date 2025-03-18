package cmd

import (
	"math/big"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func distributeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "distribute [chain-id] [wallet-id] [amount]",
		Short: "Distribute tokens evenly to all other wallets on a chain",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			chainID := args[0]
			senderWalletID := args[1]

			amount, success := new(big.Int).SetString(args[2], 10)
			if !success {
				return errors.New("invalid amount, must be a valid integer")
			}

			network, err := cfg.ToNetwork(ctx, logger)
			if err != nil {
				return errors.Wrap(err, "failed to build network")
			}

			chain := network.GetChain(chainID)
			if chain == nil {
				return errors.Errorf("chain not found: %s", chainID)
			}

			wallets := chain.GetWallets()
			if len(wallets) <= 1 {
				return errors.New("no other wallets found to distribute to")
			}

			logger.Info("Distributing tokens",
				zap.String("chain", chainID),
				zap.String("sender", senderWalletID),
				zap.Int("num_recipients", len(wallets)-1),
				zap.String("amount_per_recipient", amount.String()))

			for _, wallet := range wallets {
				if wallet.GetID() == senderWalletID {
					continue
				}

				txHash, err := chain.NativeSend(ctx, senderWalletID, amount, wallet.GetAddress())
				if err != nil {
					return errors.Wrapf(err, "failed to send tokens to %s", wallet.GetID())
				}

				// Just to give it time to land
				time.Sleep(8 * time.Second)

				logger.Info("Sent tokens",
					zap.String("from", senderWalletID),
					zap.String("to", wallet.GetID()),
					zap.String("to_address", wallet.GetAddress()),
					zap.String("amount", amount.String()),
					zap.String("tx_hash", txHash))
			}

			logger.Info("Distribution completed successfully")
			return nil
		},
	}

	return cmd
}
