package solidity

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"

	ibctm "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"
	"github.com/ethereum/go-ethereum/crypto"
)

var DefaultTrustLevel = ibctm.Fraction{Numerator: 2, Denominator: 3}.ToTendermint()

const DefaultTrustPeriod = 1209669

type ForgeScriptReturnValues struct {
	InternalType string `json:"internal_type"`
	Value        string `json:"value"`
}

type ForgeDeployOutput struct {
	Returns map[string]ForgeScriptReturnValues `json:"returns"`
}

type DeployedContracts struct {
	Ics07Tendermint string `json:"ics07Tendermint"`
	Ics26Router     string `json:"ics26Router"`
	RelayerHelper   string `json:"relayerHelper"`
	Ics20Transfer   string `json:"ics20Transfer"`
	Erc20           string `json:"erc20"`
}

func DeployIBC(
	ethRPC string,
	deployerPrivKey *ecdsa.PrivateKey,
	faucetPubKey ecdsa.PublicKey,
) (DeployedContracts, error) {
	genesisFilePath := "solidity-ibc-eureka/scripts/genesis.json"
	if err := generateGenesis("groth16", genesisFilePath); err != nil {
		return DeployedContracts{}, err
	}

	stdout, err := runDeployScript(ethRPC, deployerPrivKey, faucetPubKey)
	if err != nil {
		return DeployedContracts{}, err
	}

	return getEthContractsFromDeployOutput(string(stdout))
}

func runDeployScript(
	ethRPC string,
	deployerPrivKey *ecdsa.PrivateKey,
	faucetPubKey ecdsa.PublicKey,
) ([]byte, error) {
	cmd := exec.Command(
		"forge",
		"script",
		"solidity-ibc-eureka/scripts/E2ETestDeploy.s.sol:E2ETestDeploy",
		"--rpc-url", ethRPC,
		"--private-key", hex.EncodeToString(crypto.FromECDSA(deployerPrivKey)),
		"--broadcaoast",
		"--non-interactive",
		"-vvvv",
	)

	faucetAddress := crypto.PubkeyToAddress(faucetPubKey)
	extraEnv := []string{
		fmt.Sprintf("E2E_FAUCET_ADDRESS=%s", faucetAddress.Hex()),
	}

	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, extraEnv...)

	var stdoutBuf bytes.Buffer

	// Create a MultiWriter to write to both os.Stdout and the buffer
	multiWriter := io.MultiWriter(os.Stdout, &stdoutBuf)

	// Set the command's stdout to the MultiWriter
	cmd.Stdout = multiWriter
	cmd.Stderr = os.Stderr

	// Run the command
	if err := cmd.Run(); err != nil {
		fmt.Println("Error start command", cmd.Args, err)
		return nil, err
	}

	// Get the output as byte slices
	stdoutBytes := stdoutBuf.Bytes()

	return stdoutBytes, nil

}

func getEthContractsFromDeployOutput(stdout string) (DeployedContracts, error) {
	// Remove everything above the JSON part
	cutOff := "== Return =="
	cutoffIndex := strings.Index(stdout, cutOff)
	stdout = stdout[cutoffIndex+len(cutOff):]

	// Extract the JSON part using regex
	re := regexp.MustCompile(`\{.*\}`)
	jsonPart := re.FindString(stdout)

	jsonPart = strings.ReplaceAll(jsonPart, `\"`, `"`)
	jsonPart = strings.Trim(jsonPart, `"`)

	var embeddedContracts DeployedContracts
	err := json.Unmarshal([]byte(jsonPart), &embeddedContracts)
	if err != nil {
		return DeployedContracts{}, err
	}

	if embeddedContracts.Erc20 == "" ||
		embeddedContracts.Ics07Tendermint == "" ||
		embeddedContracts.Ics20Transfer == "" ||
		embeddedContracts.Ics26Router == "" {

		return DeployedContracts{}, fmt.Errorf("one or more contracts missing: %+v", embeddedContracts)
	}

	return embeddedContracts, nil
}
