package cmd

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func relayCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "relay [from-chain-id] [to-chain-id] [tx-hash]",
		Short: "Relay IBC packet",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			logger, err := createStandardLogger()
			if err != nil {
				return errors.Wrap(err, "failed to create logger")
			}

			network, err := cfg.ToNetwork(ctx, logger)
			if err != nil {
				return errors.Wrap(err, "failed to build network")
			}

			fromChain := network.GetChain(args[0])
			toChain := network.GetChain(args[1])
			txHash := args[2]

			relayerWalletID := "relayer"
			relayerWallet, err := fromChain.GetWallet(relayerWalletID)
			if err != nil {
				return errors.Wrapf(err, "failed to get wallet %s", relayerWalletID)
			}
			if !strings.HasPrefix(txHash, "0x") {
				relayerWalletID = "ggeth"
			}

			packets, err := fromChain.GetPackets(ctx, txHash)
			if err != nil {
				return errors.Wrap(err, "failed to get packets")
			}
			if len(packets) != 1 {
				return errors.Errorf("expected 1 packet, got %d", len(packets))
			}

			relayTxHash, err := network.Relayer.Relay(ctx, fromChain, toChain, packets[0].DestinationClient, relayerWallet, []string{txHash})
			if err != nil {
				return errors.Wrapf(err, "failed to relay transfer tx: %s", packets[0].TxHash)
			}

			logger.Info("Relay successful", zap.String("fromChain", fromChain.GetChainID()), zap.String("toChain", toChain.GetChainID()), zap.String("txHash", txHash), zap.String("relayTxHash", relayTxHash))

			return nil
		},
	}

	return cmd
}
