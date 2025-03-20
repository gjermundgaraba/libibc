package cmd

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gjermundgaraba/libibc/chains/network"
	"github.com/gjermundgaraba/libibc/cmd/ibc/tui"
	"github.com/gjermundgaraba/libibc/ibc"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func scriptCmd() *cobra.Command {
	var (
		numPacketsPerWallet int
		transferAmount      int

		chainAId              string
		chainAClientId        string
		chainADenom           string
		chainARelayerWalletId string

		chainBId              string
		chainBSideClientId    string
		chainBDenom           string
		chainBRelayerWalletId string
	)

	cmd := &cobra.Command{
		Use:   "script",
		Short: "Run a script",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			tuiInstance := tui.NewTui("Starting script", "Initializing")

			network, err := cfg.ToNetwork(ctx, tuiInstance.GetLogger())
			if err != nil {
				return errors.Wrap(err, "failed to build network")
			}

			transferAmountBig := big.NewInt(int64(transferAmount))
			chainA := network.GetChain(chainAId)
			chainB := network.GetChain(chainBId)

			chainARelayerWallet, err := chainA.GetWallet(chainARelayerWalletId)
			if err != nil {
				return errors.Wrapf(err, "failed to get wallet %s", chainARelayerWalletId)
			}

			chainBRelayerWallet, err := chainB.GetWallet(chainBRelayerWalletId)
			if err != nil {
				return errors.Wrapf(err, "failed to get wallet %s", chainBRelayerWalletId)
			}

			chainBWallets := chainB.GetWallets()
			chainAWallets := chainA.GetWallets()

			// TODO: REMOVE
			chainBWallets = chainBWallets[:3]
			chainAWallets = chainAWallets[:3]

			if len(chainBWallets) != len(chainAWallets) {
				return errors.Errorf("wallets length mismatch: %d != %d", len(chainBWallets), len(chainAWallets))
			}

			go func() {
				defer func() {
					if r := recover(); r != nil {
						tuiInstance.UpdateStatus(fmt.Sprintf("Panic: %s", r))
					}
				}()

				if err := runScript(
					ctx,
					tuiInstance,
					network,
					chainA,
					chainAClientId,
					chainADenom,
					chainAWallets,
					chainARelayerWallet,
					chainB,
					chainBSideClientId,
					chainBDenom,
					chainBWallets,
					chainBRelayerWallet,
					transferAmountBig,
					numPacketsPerWallet,
				); err != nil {
					tuiInstance.UpdateStatus(fmt.Sprintf("Error: %s", err))
				}
			}()

			program := tea.NewProgram(
				tuiInstance,
				tea.WithAltScreen(),       // use the full size of the terminal in its "alternate screen buffer"
				tea.WithMouseCellMotion(), // turn on mouse support so we can track the mouse wheel
			)

			if _, err := program.Run(); err != nil {
				fmt.Println("Error running TUI program:", err)
				os.Exit(1)
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&numPacketsPerWallet, "packets-per-wallet", 5, "Number of packets to send per wallet")
	cmd.Flags().IntVar(&transferAmount, "transfer-amount", 100, "Amount to transfer")
	cmd.Flags().StringVar(&chainAId, "chain-a-id", "eureka-hub-dev-6", "Chain A ID")
	cmd.Flags().StringVar(&chainAClientId, "chain-a-client-id", "08-wasm-2", "Chain A client ID")
	cmd.Flags().StringVar(&chainADenom, "chain-a-denom", "uatom", "Chain A denom")
	cmd.Flags().StringVar(&chainARelayerWalletId, "chain-a-relayer-wallet-id", "cosmos-relayer", "Chain A relayer wallet ID")
	cmd.Flags().StringVar(&chainBId, "chain-b-id", "11155111", "Chain B ID")
	cmd.Flags().StringVar(&chainBSideClientId, "chain-b-client-id", "plz-last-hub-devnet-69", "Chain B client ID")
	cmd.Flags().StringVar(&chainBDenom, "chain-b-denom", "0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14", "Chain B denom")
	cmd.Flags().StringVar(&chainBRelayerWalletId, "chain-b-relayer-wallet-id", "eth-relayer", "Chain B relayer wallet ID")

	return cmd
}

func runScript(
	ctx context.Context,
	tuiInstance *tui.Tui,
	network *network.Network,
	chainA network.Chain,
	chainASideClientId string,
	chainADenom string,
	chainAWallets []network.Wallet,
	chainARelayerWallet network.Wallet,
	chainB network.Chain,
	chainBSideClientId string,
	chainBDenom string,
	chainBWallets []network.Wallet,
	chainBRelayerWallet network.Wallet,
	transferAmountBig *big.Int,
	numPacketsPerWallet int,
) error {
	// Get the logger from the TUI
	tuiLogger := tuiInstance.GetLogger()

	aToBRelayerQueue := network.NewRelayerQueue(tuiLogger, chainA, chainB, chainARelayerWallet, 10)
	bToARelayerQueue := network.NewRelayerQueue(tuiLogger, chainB, chainA, chainBRelayerWallet, 10)

	tuiLogger.Info("Starting up", zap.Int("wallet-count", len(chainBWallets)))

	updateMutex := sync.Mutex{}
	var mainErrGroup errgroup.Group

	var transfersCompleted uint64
	totalTransfers := uint64(len(chainBWallets) * numPacketsPerWallet * 2)

	// Eth to Cosmos transfers
	for i := range len(chainBWallets) {
		idx := i
		mainErrGroup.Go(func() error {
			ethWallet := chainBWallets[idx]
			cosmosWallet := chainAWallets[idx]

			for range numPacketsPerWallet {
				updateMutex.Lock()
				transfersCompleted++
				currentTransfer := transfersCompleted
				tuiInstance.UpdateProgress(int(currentTransfer * 100 / totalTransfers))
				tuiInstance.UpdateStatus(fmt.Sprintf("Transferring (%d/%d)", currentTransfer, totalTransfers))
				updateMutex.Unlock()

				tuiLogger.Info("Transferring from eth to cosmos",
					zap.Uint64("currentTransfer", currentTransfer),
					zap.Uint64("totalTransfers", totalTransfers),
					zap.String("from", ethWallet.Address()),
					zap.String("from-id", ethWallet.ID()),
					zap.String("to-id", cosmosWallet.ID()),
					zap.String("to", cosmosWallet.Address()),
					zap.String("amount", transferAmountBig.String()),
				)
				var packet ibc.Packet
				if err := withRetry(func() error {
					var err error
					packet, err = chainB.SendTransfer(ctx, chainBSideClientId, ethWallet, transferAmountBig, chainBDenom, cosmosWallet.Address())
					return err
				}); err != nil {
					return errors.Wrapf(err, "failed to create transfer from eth to cosmos")
				}

				aToBRelayerQueue.Add(packet)
			}

			return nil
		})
	}

	// Cosmos to Eth transfers
	for i := range len(chainAWallets) {
		idx := i
		mainErrGroup.Go(func() error {
			ethWallet := chainBWallets[idx]
			cosmosWallet := chainAWallets[idx]

			for range numPacketsPerWallet {
				updateMutex.Lock()
				transfersCompleted++
				currentTransfer := transfersCompleted
				tuiInstance.UpdateProgress(int(currentTransfer * 100 / totalTransfers))
				tuiInstance.UpdateStatus(fmt.Sprintf("Transferring (%d/%d)", currentTransfer, totalTransfers))
				updateMutex.Unlock()

				tuiLogger.Info("Transferring from cosmos to eth",
					zap.Uint64("currentTransfer", currentTransfer),
					zap.Uint64("totalTransfers", totalTransfers),
					zap.String("from", cosmosWallet.Address()),
					zap.String("from-id", cosmosWallet.ID()),
					zap.String("to-id", ethWallet.ID()),
					zap.String("to", ethWallet.Address()),
					zap.String("amount", transferAmountBig.String()),
				)
				var packet ibc.Packet
				if err := withRetry(func() error {
					var err error
					packet, err = chainA.SendTransfer(ctx, chainASideClientId, cosmosWallet, transferAmountBig, chainADenom, ethWallet.Address())
					return err
				}); err != nil {
					return errors.Wrapf(err, "failed to send transfer from cosmos to eth")
				}

				bToARelayerQueue.Add(packet)
			}

			return nil
		})
	}

	defer func() {
		tuiLogger.Info("Script done (error or not)",
			zap.Uint64("transfers-completed", transfersCompleted),
			zap.Uint64("total-transfers", totalTransfers),
		)
	}()

	// Wait for everything to complete
	if err := mainErrGroup.Wait(); err != nil {
		return errors.Wrap(err, "failed to complete transfers")
	}
	// Flush relayer queues
	mainErrGroup.Go(func() error {
		return aToBRelayerQueue.Flush()
	})
	mainErrGroup.Go(func() error {
		return bToARelayerQueue.Flush()
	})
	if err := mainErrGroup.Wait(); err != nil {
		return errors.Wrap(err, "failed to flush queues")
	}

	tuiInstance.UpdateStatus("All transfers completed")

	return nil
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
