package cmd

import (
	"strings"

	"github.com/gjermundgaraba/libibc/relayer"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func relayCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "relay [from-chain-id] [to-chain-id] [tx-hash] [relayer-wallet-id]",
		Short: "Relay IBC packet",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			logWriter.AddExtraLogger(func(entry string) {
				cmd.Println(entry)
			})

			network, err := cfg.ToNetwork(ctx, logger, extraGwei)
			if err != nil {
				return errors.Wrap(err, "failed to build network")
			}

			fromChain, err := network.GetChain(args[0])
			if err != nil {
				return errors.Wrapf(err, "failed to get chain %s", args[0])
			}
			toChain, err := network.GetChain(args[1])
			if err != nil {
				return errors.Wrapf(err, "failed to get chain %s", args[1])
			}
			txHash := args[2]

			relayerWalletID := args[3]
			relayerWallet, err := toChain.GetWallet(relayerWalletID)
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

			relayer := relayer.NewRelayerQueue(logger, fromChain, toChain, relayerWallet, 1, true, "")

			relayer.Add(packets[0])

			if err := relayer.Flush(); err != nil {
				return errors.Wrapf(err, "failed to relay transfer tx: %s", packets[0].TxHash)
			}

			logger.Info("Relay successful", zap.String("fromChain", fromChain.GetChainID()), zap.String("toChain", toChain.GetChainID()), zap.String("txHash", txHash))

			return nil
		},
	}

	return cmd
}
