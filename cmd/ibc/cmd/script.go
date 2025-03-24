package cmd

import (
	"fmt"
	"math/big"
	"os"

	"github.com/gjermundgaraba/libibc/cmd/ibc/loadscript"
	"github.com/gjermundgaraba/libibc/cmd/ibc/tui"
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
			tuiInstance := tui.NewTui("Starting script", "Initializing")
			tuiLogger := tuiInstance.GetLogger()

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

			if len(chainBWallets) > 5 {
				chainBWallets = chainBWallets[:5]
			}
			if len(chainAWallets) > 5 {
				chainAWallets = chainAWallets[:5]
			}

			if len(chainBWallets) != len(chainAWallets) {
				return errors.Errorf("wallets length mismatch: %d != %d", len(chainBWallets), len(chainAWallets))
			}

			go func() {
				defer func() {
					if r := recover(); r != nil {
						tuiLogger.Error("Panic", zap.Any("panic", r))
						tuiInstance.UpdateMainErrorStatus(fmt.Sprintf("Panic: %v", r))
					}
				}()

				tuiLogger.Info("Starting up", zap.Int("wallet-count", len(chainBWallets)))

				var mainErrGroup errgroup.Group

				tuiInstance.UpdateMainStatus("Transferring...")

				transferStatusModelAToB := tui.NewStatusModel(fmt.Sprintf("Transferring from %s to %s 0/0", chainA.GetChainID(), chainB.GetChainID()))
				tuiInstance.AddStatusModel(transferStatusModelAToB)

				relayingStatusModelAToB := tui.NewStatusModel(fmt.Sprintf("Relaying from %s to %s 0/0", chainA.GetChainID(), chainB.GetChainID()))
				tuiInstance.AddStatusModel(relayingStatusModelAToB)

				transferStatusModelBToA := tui.NewStatusModel(fmt.Sprintf("Transferring from %s to %s 0/0", chainB.GetChainID(), chainA.GetChainID()))
				tuiInstance.AddStatusModel(transferStatusModelBToA)

				relayingStatusModelBToA := tui.NewStatusModel(fmt.Sprintf("Relaying from %s to %s 0/0", chainB.GetChainID(), chainA.GetChainID()))
				tuiInstance.AddStatusModel(relayingStatusModelBToA)

				mainErrGroup.Go(func() error {
					progressCh, err := loadscript.TransferAndRelayFromAToB(
						ctx,
						tuiLogger,
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
						if update.IsError {
							transferStatusModelAToB.UpdateErrorStatus(fmt.Sprintf("Failed: %s", update.ErrorMessage))
							return errors.New(update.ErrorMessage)
						}

						switch update.Stage {
						case "transfer":
							transferStatusModelAToB.UpdateStatus(fmt.Sprintf("Transferring from %s to %s (%d/%d)",
								update.FromChain, update.ToChain, update.CurrentTransfers, update.TotalTransfers))
							transferStatusModelAToB.UpdateProgress(int(update.CurrentTransfers * 100 / update.TotalTransfers))

							relayingStatusModelAToB.UpdateStatus(fmt.Sprintf("Relaying from %s to %s %d/%d (waiting: %d)",
								update.FromChain, update.ToChain, update.CurrentRelays, update.TotalTransfers, update.InQueueRelays))
							if update.TotalTransfers > 0 {
								relayingStatusModelAToB.UpdateProgress(int(update.CurrentRelays * 100 / update.TotalTransfers))
							}

						case "relaying":
							relayingStatusModelAToB.UpdateStatus(fmt.Sprintf("Relaying from %s to %s %d/%d (waiting: %d)",
								update.FromChain, update.ToChain, update.CurrentRelays, update.TotalTransfers, update.InQueueRelays))
							if update.TotalTransfers > 0 {
								relayingStatusModelAToB.UpdateProgress(int(update.CurrentRelays * 100 / update.TotalTransfers))
							}

						case "completed":
							transferStatusModelAToB.UpdateStatus(fmt.Sprintf("Transfers completed from %s to %s",
								update.FromChain, update.ToChain))
							transferStatusModelAToB.UpdateProgress(100)

							relayingStatusModelAToB.UpdateStatus(fmt.Sprintf("Relay queue flushed from %s to %s %d/%d",
								update.FromChain, update.ToChain, update.TotalTransfers, update.TotalTransfers))
							relayingStatusModelAToB.UpdateProgress(100)
						}
					}

					return nil
				})

				mainErrGroup.Go(func() error {
					progressCh, err := loadscript.TransferAndRelayFromAToB(
						ctx,
						tuiLogger,
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
					if err != nil {
						return err
					}

					for update := range progressCh {
						if update.IsError {
							transferStatusModelBToA.UpdateErrorStatus(fmt.Sprintf("Failed: %s", update.ErrorMessage))
							return errors.New(update.ErrorMessage)
						}

						switch update.Stage {
						case "transfer":
							transferStatusModelBToA.UpdateStatus(fmt.Sprintf("Transferring from %s to %s (%d/%d)",
								update.FromChain, update.ToChain, update.CurrentTransfers, update.TotalTransfers))
							transferStatusModelBToA.UpdateProgress(int(update.CurrentTransfers * 100 / update.TotalTransfers))

							relayingStatusModelBToA.UpdateStatus(fmt.Sprintf("Relaying from %s to %s %d/%d (waiting: %d)",
								update.FromChain, update.ToChain, update.CurrentRelays, update.TotalTransfers, update.InQueueRelays))
							if update.TotalTransfers > 0 {
								relayingStatusModelBToA.UpdateProgress(int(update.CurrentRelays * 100 / update.TotalTransfers))
							}

						case "relaying":
							relayingStatusModelBToA.UpdateStatus(fmt.Sprintf("Relaying from %s to %s %d/%d (waiting: %d)",
								update.FromChain, update.ToChain, update.CurrentRelays, update.TotalTransfers, update.InQueueRelays))
							if update.TotalTransfers > 0 {
								relayingStatusModelBToA.UpdateProgress(int(update.CurrentRelays * 100 / update.TotalTransfers))
							}

						case "completed":
							transferStatusModelBToA.UpdateStatus(fmt.Sprintf("Transfers completed from %s to %s",
								update.FromChain, update.ToChain))
							transferStatusModelBToA.UpdateProgress(100)

							relayingStatusModelBToA.UpdateStatus(fmt.Sprintf("Relay queue flushed from %s to %s %d/%d",
								update.FromChain, update.ToChain, update.TotalTransfers, update.TotalTransfers))
							relayingStatusModelBToA.UpdateProgress(100)
						}
					}

					return nil
				})

				if err := mainErrGroup.Wait(); err != nil {
					tuiLogger.Error("Failed to complete transfers", zap.Error(err))
					tuiInstance.UpdateMainErrorStatus(fmt.Sprintf("Failed to complete transfers: %s", err.Error()))
				}

				tuiLogger.Info("All transfers and relays completed successfully")
				tuiInstance.UpdateMainStatus("All transfers and relays completed")

			}()

			if err := tuiInstance.Run(); err != nil {
				fmt.Println("Error running TUI program:", err)
				os.Exit(1)
			}

			defer tuiInstance.Close()

			return nil
		},
	}

	cmd.Flags().IntVar(&numPacketsPerWallet, "packets-per-wallet", 5, "Number of packets to send per wallet")
	cmd.Flags().IntVar(&transferAmount, "transfer-amount", 100, "Amount to transfer")
	cmd.Flags().StringVar(&chainAId, "chain-a-id", "11155111", "Chain A ID")
	cmd.Flags().StringVar(&chainAClientId, "chain-a-client-id", "hub-testnet-0", "Chain A client ID")
	cmd.Flags().StringVar(&chainADenom, "chain-a-denom", "0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14", "Chain A denom")
	cmd.Flags().StringVar(&chainARelayerWalletId, "chain-a-relayer-wallet-id", "eth-relayer", "Chain A relayer wallet ID")
	cmd.Flags().StringVar(&chainBId, "chain-b-id", "provider", "Chain B ID")
	cmd.Flags().StringVar(&chainBClientId, "chain-b-client-id", "08-wasm-262", "Chain B client ID")
	cmd.Flags().StringVar(&chainBDenom, "chain-b-denom", "uatom", "Chain B denom")
	cmd.Flags().StringVar(&chainBRelayerWalletId, "chain-b-relayer-wallet-id", "cosmos-relayer", "Chain B relayer wallet ID")
	cmd.Flags().BoolVar(&selfRelay, "self-relay", false, "Manually relay packets")

	return cmd
}

