package cmd

import (
	"math/big"

	"github.com/gjermundgaraba/libibc/cmd/ibc/config"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func generateWalletCmd() *cobra.Command {
	var fundFromWalletId string
	var fundAmount string
	var denom string

	cmd := &cobra.Command{
		Use:   "generate-wallet [chain-id] [new-wallet-id]",
		Short: "Generate a new wallet for a chain and add it to the config",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			chainID := args[0]
			newWalletID := args[1]

			if (fundFromWalletId == "") != (fundAmount == "") {
				return errors.New("either both --fund-from-wallet and --fund-amount must be set or neither")
			}

			// Set default denom if not provided
			if denom == "" && fundAmount != "" {
				if chainID == "ethereum" {
					denom = "eth"
				} else {
					denom = "uatom" // Default for Cosmos chains
				}
			}

			network, err := cfg.ToNetwork(ctx, logger)
			if err != nil {
				return errors.Wrap(err, "failed to build network")
			}

			chain, err := network.GetChain(chainID)
			if err != nil {
				return errors.Wrapf(err, "failed to get chain %s", chainID)
			}

			if _, err := chain.GetWallet(newWalletID); err == nil {
				return errors.Errorf("wallet already exists: %s", newWalletID)
			}

			wallet, err := chain.GenerateWallet(newWalletID)
			if err != nil {
				return errors.Wrap(err, "failed to generate wallet")
			}

			logger.Info("Generated new wallet",
				zap.String("chain_id", chainID),
				zap.String("wallet_id", wallet.ID()),
				zap.String("address", wallet.Address()),
				zap.String("private_key", wallet.PrivateKeyHex()))

			for i, chainConfig := range cfg.Chains {
				if chainConfig.ChainID == chainID {
					cfg.Chains[i].WalletIDs = append(cfg.Chains[i].WalletIDs, newWalletID)
					break
				}
			}

			walletExists := false
			for i, walletConfig := range cfg.Wallets {
				if walletConfig.WalletID == newWalletID {
					// Update existing wallet
					cfg.Wallets[i].PrivateKey = wallet.PrivateKeyHex()
					walletExists = true
					break
				}
			}

			if !walletExists {
				cfg.Wallets = append(cfg.Wallets, config.WalletConfig{
					WalletID:   newWalletID,
					PrivateKey: wallet.PrivateKeyHex(),
				})
			}

			if err := cfg.SaveConfig(configPath); err != nil {
				return errors.Wrap(err, "failed to save config")
			}

			if fundFromWalletId != "" && fundAmount != "" {
				amount, success := new(big.Int).SetString(fundAmount, 10)
				if !success {
					return errors.New("invalid fund amount, must be a valid integer")
				}

				fundFromWallet, err := chain.GetWallet(fundFromWalletId)
				if err != nil {
					return errors.Wrapf(err, "failed to get wallet %s", fundFromWalletId)
				}

				txHash, err := chain.Send(ctx, fundFromWallet, amount, denom, wallet.Address())
				if err != nil {
					return errors.Wrap(err, "failed to fund new wallet")
				}

				logger.Info("Funded new wallet",
					zap.String("from_wallet", fundFromWalletId),
					zap.String("to_wallet", newWalletID),
					zap.String("amount", amount.String()),
					zap.String("denom", denom),
					zap.String("tx_hash", txHash))
			}

			logger.Info("Wallet generation completed successfully",
				zap.String("wallet_id", newWalletID),
				zap.String("chain_id", chainID),
				zap.String("config_file", configPath))

			return nil
		},
	}

	cmd.Flags().StringVar(&fundFromWalletId, "fund-from-wallet", "", "Optional wallet ID to fund the new wallet from")
	cmd.Flags().StringVar(&fundAmount, "fund-amount", "", "Optional amount to fund the new wallet with")
	cmd.Flags().StringVar(&denom, "denom", "", "Token denomination for funding (e.g., 'uatom' for Cosmos, 'eth' for Ethereum, or ERC20 contract address)")

	return cmd
}
