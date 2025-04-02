package cmd

import (
	"context"
	"fmt"
	"math/big"
	"os"

	"github.com/gjermundgaraba/libibc/chains/network"
	"github.com/gjermundgaraba/libibc/cmd/ibc/loadscript"
	"github.com/gjermundgaraba/libibc/cmd/ibc/tui"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func scriptCmd() *cobra.Command {
	var (
		maxWallets          int
		numPacketsPerWallet int
		transferAmount      int

		chainAId              string
		chainAClientId        string
		chainADenom           string
		chainARelayerWalletId string

		chainBId              string
		chainBClientId        string
		chainBDenom           string
		chainBRelayerWalletId string

		selfRelay bool
	)

	cmd := &cobra.Command{
		Use:   "script",
		Short: "Run a script",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			tuiInstance := tui.NewTui(logWriter, "Starting script", "Initializing")

			network, err := cfg.ToNetwork(ctx, logger)
			if err != nil {
				return errors.Wrap(err, "failed to build network")
			}

			transferAmountBig := big.NewInt(int64(transferAmount))
			chainA, err := network.GetChain(chainAId)
			if err != nil {
				return errors.Wrapf(err, "failed to get chain %s", chainAId)
			}
			chainB, err := network.GetChain(chainBId)
			if err != nil {
				return errors.Wrapf(err, "failed to get chain %s", chainBId)
			}

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

			if len(chainBWallets) > maxWallets {
				chainBWallets = chainBWallets[:maxWallets]
			}
			if len(chainAWallets) > maxWallets {
				chainAWallets = chainAWallets[:maxWallets]
			}

			if len(chainBWallets) != len(chainAWallets) {
				return errors.Errorf("wallets length mismatch: %d != %d", len(chainBWallets), len(chainAWallets))
			}

			go func() {
				defer func() {
					if r := recover(); r != nil {
						logger.Error("Panic", zap.Any("panic", r))
						tuiInstance.UpdateMainErrorStatus(fmt.Sprintf("Panic: %v", r))
					}
				}()

				logger.Info("Starting up", zap.Int("wallet-count", len(chainBWallets)))

				var mainErrGroup errgroup.Group

				tuiInstance.UpdateMainStatus("Transferring...")

				mainErrGroup.Go(func() error {
					return run(
						ctx,
						tuiInstance,
						logger,
						network,
						chainA,
						chainAClientId,
						chainADenom,
						chainAWallets,
						chainB,
						chainBWallets,
						chainBRelayerWallet,
						transferAmountBig,
						numPacketsPerWallet,
						selfRelay,
					)
				})

				mainErrGroup.Go(func() error {
					return run(
						ctx,
						tuiInstance,
						logger,
						network,
						chainB,
						chainBClientId,
						chainBDenom,
						chainBWallets,
						chainA,
						chainAWallets,
						chainARelayerWallet,
						transferAmountBig,
						numPacketsPerWallet,
						selfRelay,
					)
				})

				if err := mainErrGroup.Wait(); err != nil {
					logger.Error("Failed to complete transfers", zap.Error(err))
					tuiInstance.UpdateMainErrorStatus(fmt.Sprintf("Failed to complete transfers: %s", err.Error()))
				}

				logger.Info("All transfers and relays completed successfully")
				tuiInstance.UpdateMainStatus("All transfers and relays completed")

			}()

			if err := tuiInstance.Run(); err != nil {
				fmt.Println("Error running TUI program:", err)
				os.Exit(1)
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&maxWallets, "max-wallets", 5, "Maximum number of wallets to use")
	cmd.Flags().IntVar(&numPacketsPerWallet, "packets-per-wallet", 5, "Number of packets to send per wallet")
	cmd.Flags().IntVar(&transferAmount, "transfer-amount", 100, "Amount to transfer")
	cmd.Flags().StringVar(&chainAId, "chain-a-id", "11155111", "Chain A ID")
	cmd.Flags().StringVar(&chainAClientId, "chain-a-client-id", "hub-testnet-1", "Chain A client ID")
	cmd.Flags().StringVar(&chainADenom, "chain-a-denom", "0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14", "Chain A denom")
	cmd.Flags().StringVar(&chainARelayerWalletId, "chain-a-relayer-wallet-id", "eth-relayer", "Chain A relayer wallet ID")
	cmd.Flags().StringVar(&chainBId, "chain-b-id", "provider", "Chain B ID")
	cmd.Flags().StringVar(&chainBClientId, "chain-b-client-id", "08-wasm-274", "Chain B client ID")
	cmd.Flags().StringVar(&chainBDenom, "chain-b-denom", "uatom", "Chain B denom")
	cmd.Flags().StringVar(&chainBRelayerWalletId, "chain-b-relayer-wallet-id", "cosmos-relayer", "Chain B relayer wallet ID")
	cmd.Flags().BoolVar(&selfRelay, "self-relay", false, "Manually relay packets")

	return cmd
}

func run(
	ctx context.Context,
	tuiInstance *tui.Tui,
	logger *zap.Logger,
	network *network.Network,
	chainA network.Chain,
	chainAClientId string,
	chainADenom string,
	chainAWallets []network.Wallet,
	chainB network.Chain,
	chainBWallets []network.Wallet,
	chainBRelayerWallet network.Wallet,
	transferAmountBig *big.Int,
	numPacketsPerWallet int,
	selfRelay bool,
) error {
	transferStatusModelAToB := tui.NewStatusModel(fmt.Sprintf("Transferring from %s to %s 0/0", chainA.GetChainID(), chainB.GetChainID()))
	tuiInstance.AddStatusModel(transferStatusModelAToB)

	relayingStatusModelAToB := tui.NewStatusModel(fmt.Sprintf("Relaying from %s to %s 0/0", chainA.GetChainID(), chainB.GetChainID()))
	tuiInstance.AddStatusModel(relayingStatusModelAToB)

	progressCh, err := loadscript.TransferAndRelayFromAToB(
		ctx,
		logger,
		network,
		chainA,
		chainAClientId,
		chainADenom,
		chainAWallets,
		chainB,
		chainBWallets,
		chainBRelayerWallet,
		transferAmountBig,
		numPacketsPerWallet,
		selfRelay,
	)
	if err != nil {
		return err
	}

	for update := range progressCh {
		switch update.UpdateType {

		case loadscript.TransferUpdate:
			transferStatusModelAToB.UpdateStatus(fmt.Sprintf("Transferring from %s to %s (%d/%d)",
				update.FromChain, update.ToChain, update.CurrentTransfers, update.TotalTransfers))
			transferStatusModelAToB.UpdateProgress(int(update.CurrentTransfers * 100 / update.TotalTransfers))

			relayingStatusModelAToB.UpdateStatus(fmt.Sprintf("Relaying from %s to %s %d/%d (waiting: %d)",
				update.FromChain, update.ToChain, update.CompletedRelaying, update.TotalTransfers, update.InQueueRelays))
			if update.TotalTransfers > 0 {
				relayingStatusModelAToB.UpdateProgress(int(update.CompletedRelaying * 100 / update.TotalTransfers))
			}
		case loadscript.RelayingUpdate:
			relayingStatusModelAToB.UpdateStatus(fmt.Sprintf("Relaying from %s to %s %d/%d (waiting: %d)",
				update.FromChain, update.ToChain, update.CompletedRelaying, update.TotalTransfers, update.InQueueRelays))
			if update.TotalTransfers > 0 {
				relayingStatusModelAToB.UpdateProgress(int(update.CompletedRelaying * 100 / update.TotalTransfers))
			}
		case loadscript.ErrorUpdate:
			transferStatusModelAToB.UpdateStatus(fmt.Sprintf("Error transferring from %s to %s: %s",
				update.FromChain, update.ToChain, update.ErrorMessage))
			transferStatusModelAToB.UpdateProgress(0)
			relayingStatusModelAToB.UpdateStatus(fmt.Sprintf("Error relaying from %s to %s: %s",
				update.FromChain, update.ToChain, update.ErrorMessage))
			relayingStatusModelAToB.UpdateProgress(0)
			return nil
		case loadscript.DoneUpdate:
			transferStatusModelAToB.UpdateStatus(fmt.Sprintf("Transfers completed from %s to %s",
				update.FromChain, update.ToChain))
			transferStatusModelAToB.UpdateProgress(100)

			relayingStatusModelAToB.UpdateStatus(fmt.Sprintf("Relay queue flushed from %s to %s %d/%d",
				update.FromChain, update.ToChain, update.TotalTransfers, update.TotalTransfers))
			relayingStatusModelAToB.UpdateProgress(100)
			return nil
		default:
			return errors.New("unexpected update type")
		}
	}

	return nil
}
