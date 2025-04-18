package cosmoschain

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"os"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	dockerimage "github.com/docker/docker/api/types/image"
	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/gjermundgaraba/libibc/localnet/cosmoschain/dockerutils"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// CosmosChain is a local docker testnet for a Cosmos SDK chain.
// Implements the ibc.Chain interface.
type CosmosChain struct {
	cfg          ChainConfig
	CleanupLabel string

	numValidators int
	numFullNodes  int
	Validators    ChainNodes
	FullNodes     ChainNodes

	cdc *codec.ProtoCodec
	log *zap.Logger
}

func NewCosmosChain(log *zap.Logger, chainID string, cfg ChainConfig, cleanupLabel string, numValidators int, numFullNodes int) *CosmosChain {
	// if chainConfig.EncodingConfig == nil {
	// 	cfg := DefaultEncoding()
	// 	chainConfig.EncodingConfig = &cfg
	// }

	registry := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)

	return &CosmosChain{
		cfg:          cfg,
		CleanupLabel: cleanupLabel,

		numValidators: numValidators,
		numFullNodes:  numFullNodes,

		cdc: cdc,
		log: log,
	}
}

// GetCodec returns the codec for the chain.
func (c *CosmosChain) GetCodec() *codec.ProtoCodec {
	return c.cdc
}

// Nodes returns all nodes, including validators and fullnodes.
func (c *CosmosChain) Nodes() ChainNodes {
	return append(c.Validators, c.FullNodes...)
}

// AddFullNodes adds new fullnodes to the network, peering with the existing nodes.
func (c *CosmosChain) AddFullNodes(ctx context.Context, configFileOverrides map[string]any, inc int) error {
	// Get peer string for existing nodes
	peers := c.Nodes().PeerString(ctx)

	// Get genesis.json
	genbz, err := c.Validators[0].GenesisFileContent(ctx)
	if err != nil {
		return err
	}

	prevCount := c.numFullNodes
	c.numFullNodes += inc
	if err := c.initializeChainNodes(ctx, c.Validators[0].DockerClient, c.Validators[0].NetworkID); err != nil {
		return err
	}

	var eg errgroup.Group
	for i := prevCount; i < c.numFullNodes; i++ {
		i := i
		eg.Go(func() error {
			fullNode := c.FullNodes[i]
			if err := fullNode.InitFullNodeFiles(ctx); err != nil {
				return err
			}
			if err := fullNode.SetPeers(ctx, peers); err != nil {
				return err
			}
			if err := fullNode.OverwriteGenesisFile(ctx, genbz); err != nil {
				return err
			}
			for configFile, modifiedConfig := range configFileOverrides {
				modifiedToml, ok := modifiedConfig.(Toml)
				if !ok {
					return fmt.Errorf("Provided toml override for file %s is of type (%T). Expected (DecodedToml)", configFile, modifiedConfig)
				}
				if err := ModifyTomlConfigFile(
					ctx,
					fullNode.logger(),
					fullNode.DockerClient,
					c.cfg.ChainID,
					fullNode.VolumeName,
					configFile,
					modifiedToml,
				); err != nil {
					return err
				}
			}
			if err := fullNode.CreateNodeContainer(ctx); err != nil {
				return err
			}
			return fullNode.StartContainer(ctx)
		})
	}
	return eg.Wait()
}

// Exec implements ibc.Chain.
func (c *CosmosChain) Exec(ctx context.Context, cmd []string, env []string) (stdout, stderr []byte, err error) {
	return c.Validators[0].Exec(ctx, cmd, env)
}

// Implements Chain interface
func (c *CosmosChain) GetRPCAddress() string {
	return fmt.Sprintf("http://%s:26657", c.Validators[0].HostName())
}

// Implements Chain interface
func (c *CosmosChain) GetAPIAddress() string {
	return fmt.Sprintf("http://%s:1317", c.Validators[0].HostName())
}

// Implements Chain interface
func (c *CosmosChain) GetGRPCAddress() string {
	return fmt.Sprintf("%s:9090", c.Validators[0].HostName())
}

// GetHostRPCAddress returns the address of the RPC server accessible by the host.
// This will not return a valid address until the chain has been started.
func (c *CosmosChain) GetHostRPCAddress() string {
	return "http://" + c.Validators[0].hostRPCPort
}

// GetHostAPIAddress returns the address of the REST API server accessible by the host.
// This will not return a valid address until the chain has been started.
func (c *CosmosChain) GetHostAPIAddress() string {
	return "http://" + c.Validators[0].hostAPIPort
}

// GetHostGRPCAddress returns the address of the gRPC server accessible by the host.
// This will not return a valid address until the chain has been started.
func (c *CosmosChain) GetHostGRPCAddress() string {
	return c.Validators[0].hostGRPCPort
}

// GetHostPeerAddress returns the address of the P2P server accessible by the host.
// This will not return a valid address until the chain has been started.
func (c *CosmosChain) GetHostPeerAddress() string {
	return c.Validators[0].hostP2PPort
}

// HomeDir implements ibc.Chain.
func (c *CosmosChain) HomeDir() string {
	return c.Validators[0].HomeDir()
}

// Implements Chain interface
func (c *CosmosChain) CreateKey(ctx context.Context, keyName string) error {
	return c.Validators[0].CreateKey(ctx, keyName)
}

// Implements Chain interface
func (c *CosmosChain) RecoverKey(ctx context.Context, keyName, mnemonic string) error {
	return c.Validators[0].RecoverKey(ctx, keyName, mnemonic)
}

// Implements Chain interface
func (c *CosmosChain) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	b32Addr, err := c.Validators[0].AccountKeyBech32(ctx, keyName)
	if err != nil {
		return nil, err
	}

	return sdk.GetFromBech32(b32Addr, c.cfg.Bech32Prefix)
}

// BuildWallet will return a Cosmos wallet
// If mnemonic != "", it will restore using that mnemonic
// If mnemonic == "", it will create a new key
func (c *CosmosChain) BuildWallet(ctx context.Context, keyName string, mnemonic string) (Wallet, error) {
	if mnemonic != "" {
		if err := c.RecoverKey(ctx, keyName, mnemonic); err != nil {
			return Wallet{}, fmt.Errorf("failed to recover key with name %q on chain %s: %w", keyName, c.cfg.Name, err)
		}
	} else {
		if err := c.CreateKey(ctx, keyName); err != nil {
			return Wallet{}, fmt.Errorf("failed to create key with name %q on chain %s: %w", keyName, c.cfg.Name, err)
		}
	}

	addrBytes, err := c.GetAddress(ctx, keyName)
	if err != nil {
		return Wallet{}, fmt.Errorf("failed to get account address for key %q on chain %s: %w", keyName, c.cfg.Name, err)
	}

	return NewWallet(keyName, addrBytes, mnemonic, c.cfg), nil
}

func (c *CosmosChain) pullImages(ctx context.Context, cli *client.Client) {
	rc, err := cli.ImagePull(
		ctx,
		c.cfg.Image.Repository+":"+c.cfg.Image.Tag,
		dockerimage.PullOptions{},
	)
	if err != nil {
		c.log.Error("Failed to pull image",
			zap.Error(err),
			zap.String("repository", c.cfg.Image.Repository),
			zap.String("tag", c.cfg.Image.Tag),
		)
	} else {
		_, _ = io.Copy(io.Discard, rc)
		_ = rc.Close()
	}
}

// NewChainNode constructs a new cosmos chain node with a docker volume.
func (c *CosmosChain) NewChainNode(
	ctx context.Context,
	cli *client.Client,
	networkID string,
	image dockerutils.ImageRef,
	validator bool,
	index int,
) (*Node, error) {
	// Construct the Node first so we can access its name.
	// The Node's VolumeName cannot be set until after we create the volume.
	tn := NewChainNode(c.log, validator, c, cli, networkID, image, c.CleanupLabel, index)

	v, err := cli.VolumeCreate(ctx, volumetypes.CreateOptions{
		Labels: map[string]string{
			dockerutils.CleanupLabel: c.cfg.ChainID,

			dockerutils.NodeOwnerLabel: tn.Name(),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating volume for chain node: %w", err)
	}
	tn.VolumeName = v.Name

	if err := dockerutils.SetVolumeOwner(ctx, dockerutils.VolumeOwnerOptions{
		Log: c.log,

		Client: cli,

		VolumeName:   v.Name,
		ImageRef:     image.Ref(),
		CleanupLabel: c.cfg.ChainID,
		UidGid:       image.UidGid,
	}); err != nil {
		return nil, fmt.Errorf("set volume owner: %w", err)
	}

	return tn, nil
}

func (c *CosmosChain) initializeChainNodes(
	ctx context.Context,
	cli *client.Client,
	networkID string,
) error {
	c.pullImages(ctx, cli)

	newVals := make(ChainNodes, c.numValidators)
	copy(newVals, c.Validators)
	newFullNodes := make(ChainNodes, c.numFullNodes)
	copy(newFullNodes, c.FullNodes)

	eg, egCtx := errgroup.WithContext(ctx)
	for i := len(c.Validators); i < c.numValidators; i++ {
		i := i
		eg.Go(func() error {
			val, err := c.NewChainNode(egCtx, cli, networkID, c.cfg.Image, true, i)
			if err != nil {
				return err
			}
			newVals[i] = val
			return nil
		})
	}
	for i := len(c.FullNodes); i < c.numFullNodes; i++ {
		i := i
		eg.Go(func() error {
			fn, err := c.NewChainNode(egCtx, cli, networkID, c.cfg.Image, false, i)
			if err != nil {
				return err
			}
			newFullNodes[i] = fn
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	c.Validators = newVals
	c.FullNodes = newFullNodes
	return nil
}

func (c *CosmosChain) createFaucetWallet(ctx context.Context) (WalletAmount, error) {
	// Faucet addresses are created separately because they need to be explicitly added to the chains.
	faucetWallet, err := c.BuildWallet(ctx, "faucet", "")
	if err != nil {
		return WalletAmount{}, fmt.Errorf("failed to create faucet account: %w", err)
	}

	// Add faucet for each chain first.
	// The values are nil at this point, so it is safe to directly assign the slice.
	return WalletAmount{
		Address: faucetWallet.FormattedAddress(),
		Denom:   c.cfg.Denom,
		Amount:  sdkmath.NewInt(100_000_000_000_000), // Faucet wallet gets 100T units of denom.
	}, nil
}

type GenesisValidatorPubKey struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}
type GenesisValidators struct {
	Address string                 `json:"address"`
	Name    string                 `json:"name"`
	Power   string                 `json:"power"`
	PubKey  GenesisValidatorPubKey `json:"pub_key"`
}
type GenesisFile struct {
	Validators []GenesisValidators `json:"validators"`
}

type ValidatorWithIntPower struct {
	Address      string
	Power        int64
	PubKeyBase64 string
}

// Bootstraps the chain and starts it from genesis
func (c *CosmosChain) Start(ctx context.Context, log *zap.Logger, additionalGenesisWallets ...WalletAmount) (dockerutils.CleanupFunc, error) {
	dockerClient, networkID, cleanupFunc := dockerutils.DockerSetup(log, c.CleanupLabel)

	// Initialize the chain (pull docker images, etc.).
	if err := c.initializeChainNodes(ctx, dockerClient, networkID); err != nil {
		return nil, fmt.Errorf("failed to initialize chain nodes: %w", err)
	}

	// Set up genesis wallets
	var genesisWallets []WalletAmount
	faucetWallet, err := c.createFaucetWallet(ctx)
	if err != nil {
		return nil, err
	}
	genesisWallets = append(genesisWallets, faucetWallet)
	genesisWallets = append(genesisWallets, additionalGenesisWallets...)

	decimalPow := int64(math.Pow10(int(c.cfg.CoinDecimals)))

	genesisAmounts := make([]sdk.Coin, len(c.Validators))
	genesisSelfDelegation := make([]sdk.Coin, len(c.Validators))

	for i := range c.Validators {
		genesisAmounts[i] = sdk.Coin{Amount: sdkmath.NewInt(10_000_000).MulRaw(decimalPow), Denom: c.cfg.Denom}
		genesisSelfDelegation[i] = sdk.Coin{Amount: sdkmath.NewInt(5_000_000).MulRaw(decimalPow), Denom: c.cfg.Denom}
		if c.cfg.ModifyGenesisAmounts != nil {
			amount, selfDelegation := c.cfg.ModifyGenesisAmounts(i)
			genesisAmounts[i] = amount
			genesisSelfDelegation[i] = selfDelegation
		}
	}

	configFileOverrides := c.cfg.ConfigFileOverrides

	eg := new(errgroup.Group)
	// Initialize config for each validator node
	for i, validator := range c.Validators {
		i := i
		validator := validator
		validator.Validator = true
		eg.Go(func() error {
			if err := validator.InitFullNodeFiles(ctx); err != nil {
				return fmt.Errorf("failed to init validator files: %w", err)
			}
			for configFile, modifiedConfig := range configFileOverrides {
				modifiedToml, ok := modifiedConfig.(Toml)
				if !ok {
					return fmt.Errorf("Provided toml override for file %s is of type (%T). Expected (DecodedToml)", configFile, modifiedConfig)
				}
				if err := ModifyTomlConfigFile(
					ctx,
					validator.logger(),
					validator.DockerClient,
					c.CleanupLabel,
					validator.VolumeName,
					configFile,
					modifiedToml,
				); err != nil {
					return fmt.Errorf("failed to modify toml config file: %w", err)
				}
			}

			return validator.InitValidatorGenTx(ctx, genesisAmounts[i], genesisSelfDelegation[i])
		})
	}

	// Initialize config for each full node.
	for _, fullNode := range c.FullNodes {
		fullNode := fullNode
		fullNode.Validator = false
		eg.Go(func() error {
			if err := fullNode.InitFullNodeFiles(ctx); err != nil {
				return fmt.Errorf("failed to init full node files: %w", err)
			}
			for configFile, modifiedConfig := range configFileOverrides {
				modifiedToml, ok := modifiedConfig.(Toml)
				if !ok {
					return fmt.Errorf("Provided toml override for file %s is of type (%T). Expected (DecodedToml)", configFile, modifiedConfig)
				}
				if err := ModifyTomlConfigFile(
					ctx,
					fullNode.logger(),
					fullNode.DockerClient,
					c.CleanupLabel,
					fullNode.VolumeName,
					configFile,
					modifiedToml,
				); err != nil {
					return fmt.Errorf("failed to modify toml config file for full node: %w", err)
				}
			}
			return nil
		})
	}

	// wait for this to finish
	if err := eg.Wait(); err != nil {
		return nil, fmt.Errorf("failed to initialize chain nodes: %w", err)
	}

	// We'll use the first validator for the genesis file
	validator0 := c.Validators[0]
	validator0Address, err := validator0.AccountKeyBech32(ctx, valKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get bech32 address for validator0: %w", err)
	}

	// Add validators to genesisWallets. We do this after init (which includes InitValidatorGenTx) so that the keys exist
	for i, validator := range c.Validators {
		bech32, err := validator.AccountKeyBech32(ctx, valKey)
		if err != nil {
			return nil, fmt.Errorf("failed to get bech32 address for validator (index: %d): %w", i, err)
		}

		genesisWallets = append(genesisWallets, WalletAmount{
			Address: bech32,
			Denom:   c.cfg.Denom,
			Amount:  genesisAmounts[i].Amount,
		})

		// copy gentx from validator to the first validator
		if err := validator.copyGentx(ctx, validator0); err != nil {
			return nil, fmt.Errorf("failed to copy gentx from validator %s to validator0: %w", validator.Name(), err)
		}
	}

	for _, wallet := range genesisWallets {
		if wallet.Address == validator0Address {
			// Skip the first validator, since it added this during gentx
			continue
		}
		if err := validator0.AddGenesisAccount(ctx, wallet.Address, sdk.Coin{Denom: c.cfg.Denom, Amount: wallet.Amount}); err != nil {
			return nil, fmt.Errorf("failed to add genesis account for %s: %w", wallet.Address, err)
		}
	}

	if err := validator0.CollectGentxs(ctx); err != nil {
		return nil, fmt.Errorf("failed to collect gentxs: %w", err)
	}

	genbz, err := validator0.GenesisFileContent(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get genesis file content: %w", err)
	}

	genbz = bytes.ReplaceAll(genbz, []byte(`"stake"`), fmt.Appendf(nil, `"%s"`, c.cfg.Denom))

	if c.cfg.ModifyGenesis != nil {
		genbz, err = c.cfg.ModifyGenesis(c.cfg, genbz)
		if err != nil {
			return nil, fmt.Errorf("failed to modify genesis file: %w", err)
		}
	}

	// Provide EXPORT_GENESIS_FILE_PATH and EXPORT_GENESIS_CHAIN to help debug genesis file
	exportGenesis := os.Getenv("EXPORT_GENESIS_FILE_PATH")
	exportGenesisChain := os.Getenv("EXPORT_GENESIS_CHAIN")
	if exportGenesis != "" && exportGenesisChain == c.cfg.Name {
		c.log.Debug("Exporting genesis file",
			zap.String("chain", exportGenesisChain),
			zap.String("path", exportGenesis),
		)
		_ = os.WriteFile(exportGenesis, genbz, 0600)
	}

	chainNodes := c.Nodes()

	for _, cn := range chainNodes {
		if err := cn.OverwriteGenesisFile(ctx, genbz); err != nil {
			return nil, fmt.Errorf("failed to overwrite genesis file for %s: %w", cn.Name(), err)
		}
	}

	if err := chainNodes.LogGenesisHashes(ctx); err != nil {
		return nil, fmt.Errorf("failed to log genesis hashes: %w", err)
	}

	eg, egCtx := errgroup.WithContext(ctx)
	for _, n := range chainNodes {
		n := n
		eg.Go(func() error {
			return n.CreateNodeContainer(egCtx)
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, fmt.Errorf("failed to create node containers: %w", err)
	}

	peers := chainNodes.PeerString(ctx)

	eg, egCtx = errgroup.WithContext(ctx)
	for _, n := range chainNodes {
		n := n
		c.log.Info("Starting container", zap.String("container", n.Name()))
		eg.Go(func() error {
			if err := n.SetPeers(egCtx, peers); err != nil {
				return err
			}
			return n.StartContainer(egCtx)
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, fmt.Errorf("failed to start node containers: %w", err)
	}

	// Wait for blocks before considering the chains "started"
	return cleanupFunc, WaitForBlocks(ctx, 2, c.Validators[0])
}

// Height implements ibc.Chain
func (c *CosmosChain) Height(ctx context.Context) (int64, error) {
	return c.Validators[0].Height(ctx)
}

// StopAllNodes stops and removes all long running containers (validators and full nodes)
func (c *CosmosChain) StopAllNodes(ctx context.Context) error {
	var eg errgroup.Group
	for _, n := range c.Nodes() {
		n := n
		eg.Go(func() error {
			if err := n.StopContainer(ctx); err != nil {
				return err
			}
			return n.RemoveContainer(ctx)
		})
	}
	return eg.Wait()
}

// StartAllNodes creates and starts new containers for each node.
// Should only be used if the chain has previously been started with Start().
func (c *CosmosChain) StartAllNodes(ctx context.Context) error {
	var eg errgroup.Group
	for _, n := range c.Nodes() {
		n := n
		eg.Go(func() error {
			if err := n.CreateNodeContainer(ctx); err != nil {
				return err
			}
			return n.StartContainer(ctx)
		})
	}
	return eg.Wait()
}
