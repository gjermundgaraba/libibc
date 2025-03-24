// TODO: GET RID OF OPERATOR PLZ
package solidity

import (
	"os"
	"os/exec"
)

// RunGenesis is a function that runs the genesis script to generate genesis.json
func generateGenesis(proofType string, outputPath string) error {
	cmd := exec.Command("operator", "genesis", "--proof-type", proofType, "--output-path", outputPath)
	cmd.Stdout = os.Stdout
	return cmd.Run()
}
