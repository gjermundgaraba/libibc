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
		Use:   "distribute [chain-id] [wallet-id] [denom] [minimum-amount]",
		Short: "Distribute tokens to ensure all wallets have at least the minimum amount",
		Long: `Distribute tokens from a sender wallet to all other wallets on a chain to ensure they have at least the minimum amount.
- Each recipient's balance is checked and tokens are only sent if their balance is below the minimum amount.
- If a recipient's balance is already equal to or higher than the minimum amount, no tokens are sent.
- The denom argument specifies which token to distribute (e.g., 'uatom' for Cosmos, 'eth' for Ethereum, or ERC20 contract address).`,
		Args: cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			chainID := args[0]
			senderWalletID := args[1]
			denom := args[2]

			logWriter.AddExtraLogger(func(entry string) {
				cmd.Println(entry)
			})

			minimumAmount, success := new(big.Int).SetString(args[3], 10)
			if !success {
				return errors.New("invalid minimum amount, must be a valid integer")
			}

			network, err := cfg.ToNetwork(ctx, logger, extraGwei)
			if err != nil {
				return errors.Wrap(err, "failed to build network")
			}

			chain, err := network.GetChain(chainID)
			if err != nil {
				return errors.Wrapf(err, "failed to get chain %s", chainID)
			}

			senderWallet, err := chain.GetWallet(senderWalletID)
			if err != nil {
				return errors.Wrapf(err, "failed to get sender wallet %s", senderWalletID)
			}

			wallets := chain.GetWallets()
			if len(wallets) <= 1 {
				return errors.New("no other wallets found to distribute to")
			}

			logger.Info("Distributing tokens",
				zap.String("chain", chainID),
				zap.String("sender", senderWalletID),
				zap.Int("num_recipients", len(wallets)-1),
				zap.String("minimum_amount", minimumAmount.String()),
				zap.String("denom", denom))

			for _, wallet := range wallets {
				if wallet.ID() == senderWalletID {
					continue
				}

				// Check current balance
				currentBalance, err := chain.GetBalance(ctx, wallet.Address(), denom)
				if err != nil {
					return errors.Wrapf(err, "failed to query balance for %s", wallet.ID())
				}

				logger.Info("Checking current balance",
					zap.String("wallet_id", wallet.ID()),
					zap.String("address", wallet.Address()),
					zap.String("current_balance", currentBalance.String()),
					zap.String("minimum_amount", minimumAmount.String()))

				// If current balance is already >= minimumAmount, skip this wallet
				if currentBalance.Cmp(minimumAmount) >= 0 {
					logger.Info("Skipping wallet as balance already meets or exceeds minimum amount",
						zap.String("wallet_id", wallet.ID()),
						zap.String("address", wallet.Address()),
						zap.String("current_balance", currentBalance.String()),
						zap.String("minimum_amount", minimumAmount.String()))
					continue
				}

				// Calculate amount needed to reach minimum
				amountToSend := new(big.Int).Sub(minimumAmount, currentBalance)

				logger.Info("Sending additional tokens to reach minimum amount",
					zap.String("wallet_id", wallet.ID()),
					zap.String("current_balance", currentBalance.String()),
					zap.String("amount_to_send", amountToSend.String()),
					zap.String("target_minimum", minimumAmount.String()))

				// Send the calculated amount
				txHash, err := chain.Send(ctx, senderWallet, amountToSend, denom, wallet.Address())
				if err != nil {
					return errors.Wrapf(err, "failed to send tokens to %s", wallet.ID())
				}

				// Just to give it time to land
				time.Sleep(12 * time.Second)

				logger.Info("Sent tokens",
					zap.String("from", senderWalletID),
					zap.String("to", wallet.ID()),
					zap.String("to_address", wallet.Address()),
					zap.String("amount", amountToSend.String()),
					zap.String("denom", denom),
					zap.String("tx_hash", txHash))
			}

			logger.Info("Distribution completed successfully")
			return nil
		},
	}

	return cmd
}
