package cosmoschain

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"hash/fnv"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/avast/retry-go"
	tmjson "github.com/cometbft/cometbft/libs/json"
	"github.com/cometbft/cometbft/p2p"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	libclient "github.com/cometbft/cometbft/rpc/jsonrpc/client"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/gjermundgaraba/libibc/localnet/cosmoschain/dockerutils"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	// "github.com/strangelove-ventures/interchaintest/v8/blockdb"
	// "github.com/strangelove-ventures/interchaintest/v8/dockerutil"
	// "github.com/strangelove-ventures/interchaintest/v8/ibc"
	// "github.com/strangelove-ventures/interchaintest/v8/testutil"
)

// Node represents a node in the test network that is being created
type Node struct {
	VolumeName   string
	Index        int
	Chain        *CosmosChain
	Validator    bool
	NetworkID    string
	DockerClient *dockerclient.Client
	Client       rpcclient.Client
	GrpcConn     *grpc.ClientConn
	Image        dockerutils.ImageRef
	CleanupLabel string

	lock sync.Mutex
	log  *zap.Logger

	containerLifecycle *dockerutils.ContainerLifecycle

	// Ports set during StartContainer.
	hostRPCPort   string
	hostAPIPort   string
	hostGRPCPort  string
	hostP2PPort   string
	cometHostname string
}

func NewChainNode(log *zap.Logger, validator bool, chain *CosmosChain, dockerClient *dockerclient.Client, networkID string, image dockerutils.ImageRef, cleanupLabel string, index int) *Node {
	tn := &Node{
		Index:        index,
		Chain:        chain,
		Validator:    validator,
		NetworkID:    networkID,
		DockerClient: dockerClient,
		Image:        image,
		CleanupLabel: cleanupLabel,

		log: log,
	}

	tn.containerLifecycle = dockerutils.NewContainerLifecycle(log, dockerClient, tn.Name())

	return tn
}

// ChainNodes is a collection of ChainNode
type ChainNodes []*Node

const (
	valKey      = "validator"
	blockTime   = 2 // seconds
	p2pPort     = "26656/tcp"
	rpcPort     = "26657/tcp"
	grpcPort    = "9090/tcp"
	apiPort     = "1317/tcp"
	privValPort = "1234/tcp"

	cometMockRawPort = "22331"
)

var (
	sentryPorts = nat.PortMap{
		nat.Port(p2pPort):     {},
		nat.Port(rpcPort):     {},
		nat.Port(grpcPort):    {},
		nat.Port(apiPort):     {},
		nat.Port(privValPort): {},
	}
)

// NewClient creates and assigns a new Tendermint RPC client to the ChainNode
func (tn *Node) NewClient(addr string) error {
	httpClient, err := libclient.DefaultHTTPClient(addr)
	if err != nil {
		return err
	}

	httpClient.Timeout = 10 * time.Second
	rpcClient, err := rpchttp.NewWithClient(addr, "/websocket", httpClient)
	if err != nil {
		return err
	}

	tn.Client = rpcClient

	grpcConn, err := grpc.Dial(
		tn.hostGRPCPort, grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("grpc dial: %w", err)
	}
	tn.GrpcConn = grpcConn

	return nil
}

// CliContext creates a new Cosmos SDK client context
func (tn *Node) CliContext() client.Context {
	return client.Context{
		Client:            tn.Client,
		GRPCClient:        tn.GrpcConn,
		ChainID:           tn.Chain.cfg.ChainID,
		InterfaceRegistry: tn.Chain.cfg.EncodingConfig.InterfaceRegistry,
		Input:             os.Stdin,
		Output:            os.Stdout,
		OutputFormat:      "json",
		LegacyAmino:       tn.Chain.cfg.EncodingConfig.Amino,
		TxConfig:          tn.Chain.cfg.EncodingConfig.TxConfig,
	}
}

// Name of the test node container
func (tn *Node) Name() string {
	return fmt.Sprintf("%s-%s-%d", tn.Chain.cfg.ChainID, tn.NodeType(), tn.Index)
}

func (tn *Node) NodeType() string {
	nodeType := "fn"
	if tn.Validator {
		nodeType = "val"
	}
	return nodeType
}

func (tn *Node) ContainerID() string {
	return tn.containerLifecycle.ContainerID()
}

// hostname of the test node container
func (tn *Node) HostName() string {
	return dockerutils.CondenseHostName(tn.Name())
}

// hostname of the comet mock container
func (tn *Node) HostnameCometMock() string {
	return tn.cometHostname
}

func (tn *Node) GenesisFileContent(ctx context.Context) ([]byte, error) {
	gen, err := tn.ReadFile(ctx, "config/genesis.json")
	if err != nil {
		return nil, fmt.Errorf("getting genesis.json content: %w", err)
	}

	return gen, nil
}

func (tn *Node) OverwriteGenesisFile(ctx context.Context, content []byte) error {
	err := tn.WriteFile(ctx, content, "config/genesis.json")
	if err != nil {
		return fmt.Errorf("overwriting genesis.json: %w", err)
	}

	return nil
}

func (tn *Node) copyGentx(ctx context.Context, destVal *Node) error {
	nid, err := tn.NodeID(ctx)
	if err != nil {
		return fmt.Errorf("getting node ID: %w", err)
	}

	relPath := fmt.Sprintf("config/gentx/gentx-%s.json", nid)

	gentx, err := tn.ReadFile(ctx, relPath)
	if err != nil {
		return fmt.Errorf("getting gentx content: %w", err)
	}

	err = destVal.WriteFile(ctx, gentx, relPath)
	if err != nil {
		return fmt.Errorf("overwriting gentx: %w", err)
	}

	return nil
}

type PrivValidatorKey struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type PrivValidatorKeyFile struct {
	Address string           `json:"address"`
	PubKey  PrivValidatorKey `json:"pub_key"`
	PrivKey PrivValidatorKey `json:"priv_key"`
}

// Bind returns the home folder bind point for running the node
func (tn *Node) Bind() []string {
	return []string{fmt.Sprintf("%s:%s", tn.VolumeName, tn.HomeDir())}
}

func (tn *Node) HomeDir() string {
	return path.Join("/var/cosmos-chain", tn.Chain.cfg.Name)
}

// SetTestConfig modifies the config to reasonable values for use within interchaintest.
func (tn *Node) SetTestConfig(ctx context.Context) error {
	c := make(Toml)

	// Set Log Level to info
	c["log_level"] = "info"

	p2p := make(Toml)

	// Allow p2p strangeness
	p2p["allow_duplicate_ip"] = true
	p2p["addr_book_strict"] = false

	c["p2p"] = p2p

	consensus := make(Toml)

	blockT := (time.Duration(blockTime) * time.Second).String()
	consensus["timeout_commit"] = blockT
	consensus["timeout_propose"] = blockT

	c["consensus"] = consensus

	rpc := make(Toml)

	// Enable public RPC
	rpc["laddr"] = "tcp://0.0.0.0:26657"
	rpc["allowed_origins"] = []string{"*"}

	c["rpc"] = rpc

	if err := ModifyTomlConfigFile(
		ctx,
		tn.logger(),
		tn.DockerClient,
		tn.CleanupLabel,
		tn.VolumeName,
		"config/config.toml",
		c,
	); err != nil {
		return err
	}

	a := make(Toml)
	a["minimum-gas-prices"] = tn.Chain.cfg.GasPrices

	grpc := make(Toml)

	// Enable public GRPC
	grpc["address"] = "0.0.0.0:9090"

	a["grpc"] = grpc

	api := make(Toml)

	// Enable public REST API
	api["enable"] = true
	api["swagger"] = true
	api["address"] = "tcp://0.0.0.0:1317"

	a["api"] = api

	return ModifyTomlConfigFile(
		ctx,
		tn.logger(),
		tn.DockerClient,
		tn.CleanupLabel,
		tn.VolumeName,
		"config/app.toml",
		a,
	)
}

// SetPeers modifies the config persistent_peers for a node
func (tn *Node) SetPeers(ctx context.Context, peers string) error {
	c := make(Toml)
	p2p := make(Toml)

	// Set peers
	p2p["persistent_peers"] = peers
	c["p2p"] = p2p

	return ModifyTomlConfigFile(
		ctx,
		tn.logger(),
		tn.DockerClient,
		tn.CleanupLabel,
		tn.VolumeName,
		"config/config.toml",
		c,
	)
}

// NodeCommand is a helper to retrieve a full command for a chain node binary.
// when interactions with the RPC endpoint are necessary.
// For example, if chain node binary is `gaiad`, and desired command is `gaiad keys show key1`,
// pass ("keys", "show", "key1") for command to return the full command.
// Will include additional flags for node URL, home directory, and chain ID.
func (tn *Node) NodeCommand(command ...string) []string {
	command = tn.BinCommand(command...)

	endpoint := fmt.Sprintf("tcp://%s:26657", tn.HostName())

	return append(command,
		"--node", endpoint,
	)
}

// BinCommand is a helper to retrieve a full command for a chain node binary.
// For example, if chain node binary is `gaiad`, and desired command is `gaiad keys show key1`,
// pass ("keys", "show", "key1") for command to return the full command.
// Will include additional flags for home directory and chain ID.
func (tn *Node) BinCommand(command ...string) []string {
	command = append([]string{tn.Chain.cfg.Bin}, command...)
	return append(command,
		"--home", tn.HomeDir(),
	)
}

// ExecBin is a helper to execute a command for a chain node binary.
// For example, if chain node binary is `gaiad`, and desired command is `gaiad keys show key1`,
// pass ("keys", "show", "key1") for command to execute the command against the node.
// Will include additional flags for home directory and chain ID.
func (tn *Node) ExecBin(ctx context.Context, command ...string) ([]byte, []byte, error) {
	return tn.Exec(ctx, tn.BinCommand(command...), nil)
}

// CondenseMoniker fits a moniker into the cosmos character limit for monikers.
// If the moniker already fits, it is returned unmodified.
// Otherwise, the middle is truncated, and a hash is appended to the end
// in case the only unique data was in the middle.
func CondenseMoniker(m string) string {
	if len(m) <= stakingtypes.MaxMonikerLength {
		return m
	}

	// Get the hash suffix, a 32-bit uint formatted in base36.
	// fnv32 was chosen because a 32-bit number ought to be sufficient
	// as a distinguishing suffix, and it will be short enough so that
	// less of the middle will be truncated to fit in the character limit.
	// It's also non-cryptographic, not that this function will ever be a bottleneck in tests.
	h := fnv.New32()
	h.Write([]byte(m))
	suffix := "-" + strconv.FormatUint(uint64(h.Sum32()), 36)

	wantLen := stakingtypes.MaxMonikerLength - len(suffix)

	// Half of the want length, minus 2 to account for half of the ... we add in the middle.
	keepLen := (wantLen / 2) - 2

	return m[:keepLen] + "..." + m[len(m)-keepLen:] + suffix
}

// InitHomeFolder initializes a home folder for the given node
func (tn *Node) InitHomeFolder(ctx context.Context) error {
	tn.lock.Lock()
	defer tn.lock.Unlock()

	_, _, err := tn.ExecBin(ctx,
		"init", CondenseMoniker(tn.Name()),
		"--chain-id", tn.Chain.cfg.ChainID,
	)
	return err
}

// WriteFile accepts file contents in a byte slice and writes the contents to
// the docker filesystem. relPath describes the location of the file in the
// docker volume relative to the home directory
func (tn *Node) WriteFile(ctx context.Context, content []byte, relPath string) error {
	fw := dockerutils.NewFileWriter(tn.logger(), tn.DockerClient, tn.CleanupLabel)
	return fw.WriteFile(ctx, tn.VolumeName, relPath, content)
}

// CopyFile adds a file from the host filesystem to the docker filesystem
// relPath describes the location of the file in the docker volume relative to
// the home directory
func (tn *Node) CopyFile(ctx context.Context, srcPath, dstPath string) error {
	content, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	return tn.WriteFile(ctx, content, dstPath)
}

// ReadFile reads the contents of a single file at the specified path in the docker filesystem.
// relPath describes the location of the file in the docker volume relative to the home directory.
func (tn *Node) ReadFile(ctx context.Context, relPath string) ([]byte, error) {
	fr := dockerutils.NewFileRetriever(tn.logger(), tn.DockerClient, tn.CleanupLabel)
	gen, err := fr.SingleFileContent(ctx, tn.VolumeName, relPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file at %s: %w", relPath, err)
	}
	return gen, nil
}

// CreateKey creates a key in the keyring backend test for the given node
func (tn *Node) CreateKey(ctx context.Context, name string) error {
	tn.lock.Lock()
	defer tn.lock.Unlock()

	_, _, err := tn.ExecBin(ctx,
		"keys", "add", name,
		"--coin-type", tn.Chain.cfg.CoinType,
		"--keyring-backend", keyring.BackendTest,
	)
	return err
}

// RecoverKey restores a key from a given mnemonic.
func (tn *Node) RecoverKey(ctx context.Context, keyName, mnemonic string) error {
	command := []string{
		"sh",
		"-c",
		fmt.Sprintf(`echo %q | %s keys add %s --recover --keyring-backend %s --coin-type %s --home %s --output json`, mnemonic, tn.Chain.cfg.Bin, keyName, keyring.BackendTest, tn.Chain.cfg.CoinType, tn.HomeDir()),
	}

	tn.lock.Lock()
	defer tn.lock.Unlock()

	_, _, err := tn.Exec(ctx, command, nil)
	return err
}

func (tn *Node) IsAboveSDK47(ctx context.Context) bool {
	// In SDK v47, a new genesis core command was added. This spec has many state breaking features
	// so we use this to switch between new and legacy SDK logic.
	// https://github.com/cosmos/cosmos-sdk/pull/14149
	return tn.HasCommand(ctx, "genesis")
}

// AddGenesisAccount adds a genesis account for each key
func (tn *Node) AddGenesisAccount(ctx context.Context, address string, genesisAmount sdk.Coin) error {
	amount := fmt.Sprintf("%s%s", genesisAmount.Amount.String(), genesisAmount.Denom)

	tn.lock.Lock()
	defer tn.lock.Unlock()

	// Adding a genesis account should complete instantly,
	// so use a 1-minute timeout to more quickly detect if Docker has locked up.
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	var command []string
	if tn.IsAboveSDK47(ctx) {
		command = append(command, "genesis")
	}

	command = append(command, "add-genesis-account", address, amount)

	if tn.Chain.cfg.UsingChainIDFlagCLI {
		command = append(command, "--chain-id", tn.Chain.cfg.ChainID)
	}

	_, _, err := tn.ExecBin(ctx, command...)

	return err
}

// Gentx generates the gentx for a given node
func (tn *Node) Gentx(ctx context.Context, name string, genesisSelfDelegation sdk.Coin) error {
	tn.lock.Lock()
	defer tn.lock.Unlock()

	var command []string
	if tn.IsAboveSDK47(ctx) {
		command = append(command, "genesis")
	}

	command = append(command, "gentx", valKey, fmt.Sprintf("%s%s", genesisSelfDelegation.Amount.String(), genesisSelfDelegation.Denom),
		"--gas-prices", tn.Chain.cfg.GasPrices,
		"--gas-adjustment", fmt.Sprint(tn.Chain.cfg.GasAdjustment),
		"--keyring-backend", keyring.BackendTest,
		"--chain-id", tn.Chain.cfg.ChainID,
	)

	_, _, err := tn.ExecBin(ctx, command...)
	return err
}

// CollectGentxs runs collect gentxs on the node's home folders
func (tn *Node) CollectGentxs(ctx context.Context) error {
	command := []string{tn.Chain.cfg.Bin}
	if tn.IsAboveSDK47(ctx) {
		command = append(command, "genesis")
	}

	command = append(command, "collect-gentxs", "--home", tn.HomeDir())

	tn.lock.Lock()
	defer tn.lock.Unlock()

	_, _, err := tn.Exec(ctx, command, nil)
	return err
}

// HasCommand checks if a command in the chain binary is available.
func (tn *Node) HasCommand(ctx context.Context, command ...string) bool {
	_, _, err := tn.ExecBin(ctx, command...)
	if err == nil {
		return true
	}

	if strings.Contains(string(err.Error()), "Error: unknown command") {
		return false
	}

	// cmd just needed more arguments, but it is a valid command (ex: appd tx bank send)
	if strings.Contains(string(err.Error()), "Error: accepts") {
		return true
	}

	return false
}

func (tn *Node) CreateNodeContainer(ctx context.Context) error {
	chainCfg := tn.Chain.cfg

	cmd := []string{chainCfg.Bin, "start", "--home", tn.HomeDir(), "--x-crisis-skip-assert-invariants"}
	if len(chainCfg.AdditionalStartArgs) > 0 {
		cmd = append(cmd, chainCfg.AdditionalStartArgs...)
	}

	usingPorts := nat.PortMap{}
	for k, v := range sentryPorts {
		usingPorts[k] = v
	}

	if tn.Index == 0 && chainCfg.HostPortOverride != nil {
		for intP, extP := range chainCfg.HostPortOverride {
			usingPorts[nat.Port(fmt.Sprintf("%d/tcp", intP))] = []nat.PortBinding{
				{
					HostPort: fmt.Sprintf("%d", extP),
				},
			}
		}

		fmt.Printf("Port Overrides: %v. Using: %v\n", chainCfg.HostPortOverride, usingPorts)
	}

	return tn.containerLifecycle.CreateContainer(ctx, tn.CleanupLabel, tn.NetworkID, tn.Image, usingPorts, tn.Bind(), nil, tn.HostName(), cmd, chainCfg.Env)
}

func (tn *Node) StartContainer(ctx context.Context) error {
	rpcOverrideAddr := ""

	if err := tn.containerLifecycle.StartContainer(ctx); err != nil {
		return err
	}

	// Set the host ports once since they will not change after the container has started.
	hostPorts, err := tn.containerLifecycle.GetHostPorts(ctx, rpcPort, grpcPort, apiPort, p2pPort)
	if err != nil {
		return err
	}
	tn.hostRPCPort, tn.hostGRPCPort, tn.hostAPIPort, tn.hostP2PPort = hostPorts[0], hostPorts[1], hostPorts[2], hostPorts[3]

	// Override the default RPC behavior if Comet Mock is being used.
	if tn.cometHostname != "" {
		tn.hostRPCPort = rpcOverrideAddr
	}

	err = tn.NewClient("tcp://" + tn.hostRPCPort)
	if err != nil {
		return err
	}

	time.Sleep(5 * time.Second)
	return retry.Do(func() error {
		stat, err := tn.Client.Status(ctx)
		if err != nil {
			return err
		}
		// TODO: re-enable this check, having trouble with it for some reason
		if stat != nil && stat.SyncInfo.CatchingUp {
			return fmt.Errorf("still catching up: height(%d) catching-up(%t)",
				stat.SyncInfo.LatestBlockHeight, stat.SyncInfo.CatchingUp)
		}
		return nil
	}, retry.Context(ctx), retry.Attempts(40), retry.Delay(3*time.Second), retry.DelayType(retry.FixedDelay))
}

func (tn *Node) PauseContainer(ctx context.Context) error {
	return tn.containerLifecycle.PauseContainer(ctx)
}

func (tn *Node) UnpauseContainer(ctx context.Context) error {
	return tn.containerLifecycle.UnpauseContainer(ctx)
}

func (tn *Node) StopContainer(ctx context.Context) error {
	return tn.containerLifecycle.StopContainer(ctx)
}

func (tn *Node) RemoveContainer(ctx context.Context) error {
	return tn.containerLifecycle.RemoveContainer(ctx)
}

// InitValidatorFiles creates the node files and signs a genesis transaction
func (tn *Node) InitValidatorGenTx(
	ctx context.Context,
	genesisAmount sdk.Coin,
	genesisSelfDelegation sdk.Coin,
) error {
	if err := tn.CreateKey(ctx, valKey); err != nil {
		return err
	}
	bech32, err := tn.AccountKeyBech32(ctx, valKey)
	if err != nil {
		return err
	}
	if err := tn.AddGenesisAccount(ctx, bech32, genesisAmount); err != nil {
		return err
	}
	return tn.Gentx(ctx, valKey, genesisSelfDelegation)
}

func (tn *Node) InitFullNodeFiles(ctx context.Context) error {
	if err := tn.InitHomeFolder(ctx); err != nil {
		return err
	}

	return tn.SetTestConfig(ctx)
}

// NodeID returns the persistent ID of a given node.
func (tn *Node) NodeID(ctx context.Context) (string, error) {
	// This used to call p2p.LoadNodeKey against the file on the host,
	// but because we are transitioning to operating on Docker volumes,
	// we only have to tmjson.Unmarshal the raw content.
	j, err := tn.ReadFile(ctx, "config/node_key.json")
	if err != nil {
		return "", fmt.Errorf("getting node_key.json content: %w", err)
	}

	var nk p2p.NodeKey
	if err := tmjson.Unmarshal(j, &nk); err != nil {
		return "", fmt.Errorf("unmarshaling node_key.json: %w", err)
	}

	return string(nk.ID()), nil
}

// KeyBech32 retrieves the named key's address in bech32 format from the node.
// bech is the bech32 prefix (acc|val|cons). If empty, defaults to the account key (same as "acc").
func (tn *Node) KeyBech32(ctx context.Context, name string, bech string) (string, error) {
	command := []string{tn.Chain.cfg.Bin, "keys", "show", "--address", name,
		"--home", tn.HomeDir(),
		"--keyring-backend", keyring.BackendTest,
	}

	if bech != "" {
		command = append(command, "--bech", bech)
	}

	stdout, stderr, err := tn.Exec(ctx, command, nil)
	if err != nil {
		return "", fmt.Errorf("failed to show key %q (stderr=%q): %w", name, stderr, err)
	}

	return string(bytes.TrimSuffix(stdout, []byte("\n"))), nil
}

// AccountKeyBech32 retrieves the named key's address in bech32 account format.
func (tn *Node) AccountKeyBech32(ctx context.Context, name string) (string, error) {
	return tn.KeyBech32(ctx, name, "")
}

// PeerString returns the string for connecting the nodes passed in
func (nodes ChainNodes) PeerString(ctx context.Context) string {
	addrs := make([]string, len(nodes))
	for i, n := range nodes {
		id, err := n.NodeID(ctx)
		if err != nil {
			// TODO: would this be better to panic?
			// When would NodeId return an error?
			break
		}
		hostName := n.HostName()
		ps := fmt.Sprintf("%s@%s:26656", id, hostName)
		nodes.logger().Info("Peering",
			zap.String("host_name", hostName),
			zap.String("peer", ps),
			zap.String("container", n.Name()),
		)
		addrs[i] = ps
	}
	return strings.Join(addrs, ",")
}

// LogGenesisHashes logs the genesis hashes for the various nodes
func (nodes ChainNodes) LogGenesisHashes(ctx context.Context) error {
	for _, n := range nodes {
		gen, err := n.GenesisFileContent(ctx)
		if err != nil {
			return err
		}

		n.logger().Info("Genesis", zap.String("hash", fmt.Sprintf("%X", sha256.Sum256(gen))))
	}
	return nil
}

func (nodes ChainNodes) logger() *zap.Logger {
	if len(nodes) == 0 {
		return zap.NewNop()
	}
	return nodes[0].logger()
}

func (tn *Node) Exec(ctx context.Context, cmd []string, env []string) ([]byte, []byte, error) {
	job := dockerutils.NewImage(tn.logger(), tn.DockerClient, tn.NetworkID, tn.CleanupLabel, tn.Image)
	opts := dockerutils.ContainerOptions{
		Env:   env,
		Binds: tn.Bind(),
	}
	res := job.Run(ctx, cmd, opts)
	return res.Stdout, res.Stderr, res.Err
}

func (tn *Node) logger() *zap.Logger {
	return tn.log.With(
		zap.String("chain_id", tn.Chain.cfg.ChainID),
		zap.String("label", tn.CleanupLabel),
	)
}

func (tn *Node) Height(ctx context.Context) (int64, error) {
	res, err := tn.Client.Status(ctx)
	if err != nil {
		return 0, fmt.Errorf("tendermint rpc client status: %w", err)
	}
	height := res.SyncInfo.LatestBlockHeight
	return height, nil
}
