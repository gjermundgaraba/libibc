package cmd

import (
	"context"
	"fmt"
	"math/big"
	"os"

	"github.com/gjermundgaraba/libibc/chains/network"
	"github.com/gjermundgaraba/libibc/cmd/ibc/tui"
	"github.com/gjermundgaraba/libibc/loadscript"
	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type LoadScriptConfig struct {
	ChainAId              string `toml:"chain-a-id"`
	ChainAClientId        string `toml:"chain-a-client-id"`
	ChainADenom           string `toml:"chain-a-denom"`
	ChainATransferAmount  int    `toml:"chain-a-transfer-amount"`
	ChainARelayerWalletId string `toml:"chain-a-relayer-wallet-id"`

	ChainBId              string `toml:"chain-b-id"`
	ChainBClientId        string `toml:"chain-b-client-id"`
	ChainBDenom           string `toml:"chain-b-denom"`
	ChainBRelayerWalletId string `toml:"chain-b-relayer-wallet-id"`
	ChainBTransferAmount  int    `toml:"chain-b-transfer-amount"`

	NumPacketsPerWallet int  `toml:"num-packets-per-wallet"`
	MaxWallets          int  `toml:"max-wallets"`
	SelfRelay           bool `toml:"self-relay"`
}

func scriptCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "load-script [load-config-file]",
		Short: "Run a load testing script using a load config file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			tuiInstance := tui.NewTui(logWriter, "Starting script", "Initializing")

			configPath := args[0]

			file, err := os.Open(configPath)
			if err != nil {
				return errors.Wrapf(err, "failed to open config file")
			}
			defer file.Close()

			var loadConfig LoadScriptConfig
			if err := toml.NewDecoder(file).Decode(&loadConfig); err != nil {
				return errors.Wrapf(err, "failed to decode config")
			}

			network, err := cfg.ToNetwork(ctx, logger, extraGwei)
			if err != nil {
				return errors.Wrap(err, "failed to build network")
			}

			chainATransferAmount := big.NewInt(int64(loadConfig.ChainATransferAmount))
			chainBTransferAmount := big.NewInt(int64(loadConfig.ChainBTransferAmount))

			chainA, err := network.GetChain(loadConfig.ChainAId)
			if err != nil {
				return errors.Wrapf(err, "failed to get chain %s", loadConfig.ChainAId)
			}
			chainB, err := network.GetChain(loadConfig.ChainBId)
			if err != nil {
				return errors.Wrapf(err, "failed to get chain %s", loadConfig.ChainBId)
			}

			chainARelayerWallet, err := chainA.GetWallet(loadConfig.ChainARelayerWalletId)
			if err != nil {
				return errors.Wrapf(err, "failed to get wallet %s", loadConfig.ChainARelayerWalletId)
			}

			chainBRelayerWallet, err := chainB.GetWallet(loadConfig.ChainBRelayerWalletId)
			if err != nil {
				return errors.Wrapf(err, "failed to get wallet %s", loadConfig.ChainBRelayerWalletId)
			}

			chainBWallets := chainB.GetWallets()
			chainAWallets := chainA.GetWallets()

			if len(chainBWallets) > loadConfig.MaxWallets {
				chainBWallets = chainBWallets[:loadConfig.MaxWallets]
			}
			if len(chainAWallets) > loadConfig.MaxWallets {
				chainAWallets = chainAWallets[:loadConfig.MaxWallets]
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
						loadConfig.ChainAClientId,
						loadConfig.ChainADenom,
						chainAWallets,
						chainB,
						chainBWallets,
						chainBRelayerWallet,
						chainATransferAmount,
						loadConfig.NumPacketsPerWallet,
						loadConfig.SelfRelay,
						cfg.EurekaAPIAddr,
						cfg.SkipAPIAddr,
					)
				})

				mainErrGroup.Go(func() error {
					return run(
						ctx,
						tuiInstance,
						logger,
						network,
						chainB,
						loadConfig.ChainBClientId,
						loadConfig.ChainBDenom,
						chainBWallets,
						chainA,
						chainAWallets,
						chainARelayerWallet,
						chainBTransferAmount,
						loadConfig.NumPacketsPerWallet,
						loadConfig.SelfRelay,
						cfg.EurekaAPIAddr,
						cfg.SkipAPIAddr,
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
	eurekaAPIAddr string,
	skipAPIAddr string,
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
		eurekaAPIAddr,
		skipAPIAddr,
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
