package cmd

import (
	"fmt"
	"math/big"
	"time"

	"github.com/briandowns/spinner"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func scriptCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "script",
		Short: "Run a script",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Starting script")

			s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
			s.Start()
			defer s.Stop()

			ctx := cmd.Context()

			network, err := cfg.ToNetwork(ctx, logger)
			if err != nil {
				return errors.Wrap(err, "failed to build network")
			}

			eth := network.GetChain("11155111")
			ethSideClientID := "plz-last-hub-devnet-69"
			ethRelayerWalletID := "eth-relayer"
			cosmos := network.GetChain("eureka-hub-dev-6")
			cosmosSideClientID := "08-wasm-2"
			cosmosRelayerWalletID := "cosmos-relayer"
			amount := big.NewInt(100)
			denom := "0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14"

			ethWallets := eth.GetWallets()
			cosmosWallets := cosmos.GetWallets()
			if len(ethWallets) != len(cosmosWallets) {
				return errors.Errorf("wallets length mismatch: %d != %d", len(ethWallets), len(cosmosWallets))
			}

			for range len(ethWallets) {
				ethWallet := ethWallets[0]
				cosmosWallet := cosmosWallets[0]

				if err := network.TransferWithRelay(ctx, eth, cosmos, ethSideClientID, ethWallet.GetID(), ethRelayerWalletID, cosmosRelayerWalletID, amount, denom, cosmosWallet.GetAddress()); err != nil {
					return errors.Wrap(err, "failed to transfer with relay from eth to cosmos")
				}
			}

			for range len(ethWallets) {
				ethWallet := ethWallets[0]
				cosmosWallet := cosmosWallets[0]

				if err := network.TransferWithRelay(ctx, cosmos, eth, cosmosSideClientID, cosmosWallet.GetID(), cosmosRelayerWalletID, ethRelayerWalletID, amount, denom, ethWallet.GetAddress()); err != nil {
					return errors.Wrap(err, "failed to transfer with relay from cosmos to eth")
				}
			}

			// packet, err := eth.SendTransfer(ctx, clientID, ethWalletID, amount, denom, to)
			// if err != nil {
			// 	return errors.Wrap(err, "failed to send transfer from eth to cosmos")
			// }
			//
			// dstClient, ok := eth.GetClients()[clientID]
			// if !ok {
			// 	return errors.Errorf("client %s not found", clientID)
			// }
			// sendRelayTxHash, err := network.Relayer.Relay(ctx, eth, cosmos, dstClient.ClientID, relayerWalletID, []string{packet.TxHash})
			// if err != nil {
			// 	return errors.Wrapf(err, "failed to relay transfer tx: %s", packet.TxHash)
			// }
			//
			// logger.Info("Relay send transfer tx hash", zap.String("txHash", sendRelayTxHash))
			//
			// ackRelayTxHash, err := network.Relayer.Relay(ctx, eth, cosmos, dstClient.ClientID, relayerWalletID, []string{sendRelayTxHash})
			// if err != nil {
			// 	return errors.Wrapf(err, "failed to relay ack tx: %s", packet.TxHash)
			// }
			// fmt.Printf("Relay ack tx hash: %s\n", ackRelayTxHash)

			return nil
		},
	}

	return cmd
}

