package cmd

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
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

			network, err := cfg.ToNetwork(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to build network")
			}

			eth := network.GetChain("11155111")
			clientID := "plz-last-hub-devnet-47"
			ethWalletID := "ggeth"
			cosmosWalletID := "ggcosmos"
			amount := big.NewInt(100)
			denom := "0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14"
			to := "cosmos14tvy7waxghv7sxv0h79tpv20hnt5977gtkwltu"

			packet, err := eth.SendTransfer(ctx, clientID, ethWalletID, amount, denom, to)
			if err != nil {
				return errors.Wrap(err, "failed to send transfer from eth to cosmos")
			}

			cosmos := network.GetChain("eureka-hub-dev-6")
			dstClient, ok := eth.GetClients()[clientID]
			if !ok {
				return errors.Errorf("client %s not found", clientID)
			}
			txHash := strings.TrimPrefix(packet.TxHash, "0x")
			txHashBz, err := hex.DecodeString(txHash)
			if err != nil {
				return errors.Wrapf(err, "failed to decode tx hash: %s", packet.TxHash)
			}
			relayTxHash, err := network.Relayer.Relay(ctx, eth, cosmos, dstClient.ClientID, cosmosWalletID, [][]byte{txHashBz})
			if err != nil {
				return errors.Wrapf(err, "failed to relay tx: %s", packet.TxHash)
			}

			fmt.Printf("Relay tx hash: %s\n", relayTxHash)

			return nil
		},
	}

	return cmd
}
