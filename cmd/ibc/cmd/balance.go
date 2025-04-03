package cmd

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func balanceCmd() *cobra.Command {
	var walletID string

	cmd := &cobra.Command{
		Use:   "balance [chain-id] [denom] [address]",
		Short: "Query balance for an address on a specific chain",
		Long: `Query the balance for an address on a specific chain and denomination.
If address is not provided, it will use the address from the specified wallet.
For Ethereum chain, use "eth" as the denom for native ETH balance, 
or ERC20 contract address for token balances.`,
		Args: cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			chainID := args[0]
			denom := args[1]

			logWriter.AddExtraLogger(func(entry string) {
				fmt.Println(entry)
			})

			var address string
			if len(args) == 3 {
				address = args[2]
			} else if walletID != "" {
				// Use wallet address if no address provided
			} else {
				return errors.New("either wallet-id flag or address argument must be provided")
			}

			network, err := cfg.ToNetwork(ctx, logger, extraGwei)
			if err != nil {
				return errors.Wrap(err, "failed to build network")
			}

			chain, err := network.GetChain(chainID)
			if err != nil {
				return errors.Wrapf(err, "failed to get chain %s", chainID)
			}

			// If using wallet, get the address
			if address == "" {
				wallet, err := chain.GetWallet(walletID)
				if err != nil {
					return errors.Wrapf(err, "failed to get wallet %s", walletID)
				}
				address = wallet.Address()
			}

			balance, err := chain.GetBalance(ctx, address, denom)
			if err != nil {
				return errors.Wrapf(err, "failed to get balance for address %s with denom %s", address, denom)
			}

			logger.Info("Balance retrieved",
				zap.String("chain_id", chainID),
				zap.String("address", address),
				zap.String("denom", denom),
				zap.String("balance", balance.String()))

			// Print balance to stdout for easy consumption by scripts
			fmt.Println(balance.String())

			return nil
		},
	}

	cmd.Flags().StringVar(&walletID, "wallet-id", "", "Optional wallet ID to query balance for")

	return cmd
}
