package cmd

import (
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/briandowns/spinner"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func scriptCmd() *cobra.Command {
	var numRepetitions int

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
			ethDenom := "0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14"
			cosmosDenom := "uatom"

			ethWallets := eth.GetWallets()
			cosmosWallets := cosmos.GetWallets()

			// TODO: REMOVE
			// only 5 wallets for each
			ethWallets = ethWallets[:5]
			cosmosWallets = cosmosWallets[:5]

			if len(ethWallets) != len(cosmosWallets) {
				return errors.Errorf("wallets length mismatch: %d != %d", len(ethWallets), len(cosmosWallets))
			}

			fmt.Printf("will use %d wallets\n", len(ethWallets))

			var wg sync.WaitGroup

			// Eth to Cosmos transfers
			for i := range len(ethWallets) {
				// TODO: REMOVE
				if i < 100000 {
					continue
				}
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					ethWallet := ethWallets[idx]
					cosmosWallet := cosmosWallets[idx]

					for j := range numRepetitions {
						logger.Info("Transferring from eth to cosmos",
							zap.String("from", ethWallet.GetAddress()),
							zap.String("from-id", ethWallet.GetID()),
							zap.String("to-id", cosmosWallet.GetID()),
							zap.String("to", cosmosWallet.GetAddress()),
							zap.String("amount", amount.String()),
						)
						err := network.TransferWithRelay(ctx, eth, cosmos, ethSideClientID, ethWallet.GetID(), ethRelayerWalletID, cosmosRelayerWalletID, amount, ethDenom, cosmosWallet.GetAddress())

						time.Sleep(5 * time.Second)
						if err != nil {
							fmt.Printf("Error transferring from eth to cosmos (wallet %d, iteration %d): %v\n", idx, j, err)
							continue // Continue with next iteration even if this one fails
						}

						fmt.Printf("Completed eth->cosmos transfer %d for wallet %d\n", j+1, idx)
					}
				}(i)
			}

			// Cosmos to Eth transfers
			for i := range len(cosmosWallets) {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					ethWallet := ethWallets[idx]
					cosmosWallet := cosmosWallets[idx]

					for j := range numRepetitions {
						logger.Info("Transferring from cosmos to eth",
							zap.String("from", cosmosWallet.GetAddress()),
							zap.String("from-id", cosmosWallet.GetID()),
							zap.String("to-id", ethWallet.GetID()),
							zap.String("to", ethWallet.GetAddress()),
							zap.String("amount", amount.String()),
						)
						err := network.TransferWithRelay(ctx, cosmos, eth, cosmosSideClientID, cosmosWallet.GetID(), cosmosRelayerWalletID, ethRelayerWalletID, amount, cosmosDenom, ethWallet.GetAddress())

						time.Sleep(10 * time.Second)
						if err != nil {
							fmt.Printf("Error transferring from cosmos to eth (wallet %d, iteration %d): %v\n", idx, j, err)
							continue // Continue with next iteration even if this one fails
						}

						fmt.Printf("Completed cosmos->eth transfer %d for wallet %d\n", j+1, idx)
					}
				}(i)
			}

			// Wait for all goroutines to complete
			wg.Wait()
			fmt.Println("All transfers completed")

			return nil
		},
	}

	cmd.Flags().IntVarP(&numRepetitions, "repetitions", "r", 5, "Number of times to repeat each transfer")
	return cmd
}
