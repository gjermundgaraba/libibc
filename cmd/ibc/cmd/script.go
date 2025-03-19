package cmd

import (
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/briandowns/spinner"
	"github.com/gjermundgaraba/libibc/ibc"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func scriptCmd() *cobra.Command {
	var numPacketsPerWallet int

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
			ethWallets = ethWallets[:3]
			cosmosWallets = cosmosWallets[:3]

			if len(ethWallets) != len(cosmosWallets) {
				return errors.Errorf("wallets length mismatch: %d != %d", len(ethWallets), len(cosmosWallets))
			}

			fmt.Printf("will use %d wallets\n", len(ethWallets))

			var transferEg errgroup.Group
			var relayEg errgroup.Group

			var transfersCompleted atomic.Uint64
			totalTransfers := uint64(len(ethWallets) * numPacketsPerWallet * 2)

			ethToCosmosMutex := &sync.Mutex{}
			// Eth to Cosmos transfers
			for i := range len(ethWallets) {
				idx := i
				transferEg.Go(func() error {
					ethWallet := ethWallets[idx]
					cosmosWallet := cosmosWallets[idx]

					var relayQueue []ibc.Packet
					for j := range numPacketsPerWallet {
						currentTransfer := transfersCompleted.Add(1)
						logger.Info("Transferring from eth to cosmos",
							zap.Uint64("currentTransfer", currentTransfer),
							zap.Uint64("totalTransfers", totalTransfers),
							zap.String("from", ethWallet.GetAddress()),
							zap.String("from-id", ethWallet.GetID()),
							zap.String("to-id", cosmosWallet.GetID()),
							zap.String("to", cosmosWallet.GetAddress()),
							zap.String("amount", amount.String()),
						)
						var packet ibc.Packet
						err = withRetry(func() error {
							packet, err = eth.SendTransfer(ctx, ethSideClientID, ethWallet.GetID(), amount, ethDenom, cosmosWallet.GetAddress())
							return err
						})
						if err != nil {
							return errors.Wrapf(err, "failed to create transfer from eth to cosmos")
						}

						relayQueue = append(relayQueue, packet)

						if len(relayQueue) >= 10 || j == numPacketsPerWallet-1 {
							// copy the queue to new variable so we can clear the original queue
							packetsToRelay := make([]ibc.Packet, len(relayQueue))
							copy(packetsToRelay, relayQueue)
							relayQueue = []ibc.Packet{}
							relayEg.Go(func() error {
								ethToCosmosMutex.Lock()
								defer ethToCosmosMutex.Unlock()

								var txIDs []string
								for _, packet := range relayQueue {
									txIDs = append(txIDs, packet.TxHash)
								}

								logger.Info("Relaying eth->cosmos transfer",
									zap.Uint64("currentTransfer", currentTransfer),
									zap.Uint64("totalTransfers", totalTransfers),
									zap.Int("packet-count", len(txIDs)),
									zap.String("from", ethWallet.GetAddress()),
									zap.String("to", cosmosWallet.GetAddress()),
								)
								if _, err := network.Relayer.Relay(ctx, eth, cosmos, packet.DestinationClient, cosmosRelayerWalletID, txIDs); err != nil {
									return errors.Wrapf(err, "failed to relay eth->cosmos transfer with txIDs: %v", txIDs)
								}

								return nil
							})
						}
					}

					return nil
				})
			}

			cosmosToEthMutex := &sync.Mutex{}
			// Cosmos to Eth transfers
			for i := range len(cosmosWallets) {
				idx := i
				transferEg.Go(func() error {
					ethWallet := ethWallets[idx]
					cosmosWallet := cosmosWallets[idx]

					var relayQueue []ibc.Packet
					for j := range numPacketsPerWallet {
						currentTransfer := transfersCompleted.Add(1)
						logger.Info("Transferring from cosmos to eth",
							zap.Uint64("currentTransfer", currentTransfer),
							zap.Uint64("totalTransfers", totalTransfers),
							zap.String("from", cosmosWallet.GetAddress()),
							zap.String("from-id", cosmosWallet.GetID()),
							zap.String("to-id", ethWallet.GetID()),
							zap.String("to", ethWallet.GetAddress()),
							zap.String("amount", amount.String()),
						)
						var packet ibc.Packet
						err = withRetry(func() error {
							packet, err = cosmos.SendTransfer(ctx, cosmosSideClientID, cosmosWallet.GetID(), amount, cosmosDenom, ethWallet.GetAddress())
							return err
						})
						if err != nil {
							return errors.Wrapf(err, "failed to send transfer from cosmos to eth")
						}

						relayQueue = append(relayQueue, packet)

						if len(relayQueue) >= 10 || j == numPacketsPerWallet-1 {
							// copy the queue to new variable so we can clear the original queue
							packetsToRelay := make([]ibc.Packet, len(relayQueue))
							copy(packetsToRelay, relayQueue)
							relayQueue = []ibc.Packet{}
							relayEg.Go(func() error {
								cosmosToEthMutex.Lock()
								defer cosmosToEthMutex.Unlock()

								var txIDs []string
								for _, packet := range relayQueue {
									txIDs = append(txIDs, packet.TxHash)
								}

								logger.Info("Relaying cosmos->eth transfer",
									zap.Uint64("currentTransfer", currentTransfer),
									zap.Uint64("totalTransfers", totalTransfers),
									zap.Int("packet-count", len(txIDs)),
									zap.String("from", cosmosWallet.GetAddress()),
									zap.String("to", ethWallet.GetAddress()),
								)
								if _, err := network.Relayer.Relay(ctx, cosmos, eth, packet.DestinationClient, ethRelayerWalletID, txIDs); err != nil {
									return errors.Wrapf(err, "failed to relay cosmos->eth transfer with txIDs: %v", txIDs)
								}

								return nil
							})
						}
					}

					return nil
				})
			}

			defer func() {
				transfersCompletedUint := transfersCompleted.Load()
				logger.Info("Script done (error or not)",
					zap.Uint64("transfers-completed", transfersCompletedUint),
					zap.Uint64("total-transfers", totalTransfers),
				)
			}()

			// Wait for all goroutines to complete
			if err := transferEg.Wait(); err != nil {
				return errors.Wrap(err, "failed to complete transfers")
			}
			if err := relayEg.Wait(); err != nil {
				return errors.Wrap(err, "failed to complete relays")
			}
			fmt.Println("All transfers completed")

			return nil
		},
	}

	cmd.Flags().IntVarP(&numPacketsPerWallet, "packet-per-wallet", "r", 5, "Number of packets to send per wallet")
	return cmd
}

func withRetry(f func() error) error {
	const maxRetries = 3
	var err error
	for range maxRetries {
		err = f()
		if err == nil {
			return nil
		}
	}

	return err
}
