package ethereumchain

func DefaultNetworkParams() NetworkParams {
	return NetworkParams{
		Participants: []Participants{
			{
				CLType:         "lodestar",
				CLImage:        "ethpandaops/lodestar:unstable",
				ELType:         "geth",
				ELImage:        "ethpandaops/geth:prague-devnet-6",
				ELExtraParams:  []string{"--gcmode=archive"},
				ELLogLevel:     "info",
				ValidatorCount: 64,
			},
		},
		NetworkParams: NetworkConfigParams{
			Preset:           "miminal",
			ElectraForkEpoch: 1,
		},
		WaitForFinalization: true,
		AdditionalServices:  []string{},
	}
}

// To see all the configuration options: github.com/ethpandaops/ethereum-package
type NetworkParams struct {
	Participants        []Participants      `json:"participants"`
	NetworkParams       NetworkConfigParams `json:"network_params"`
	WaitForFinalization bool                `json:"wait_for_finalization"`
	AdditionalServices  []string            `json:"additional_services"`
}

type Participants struct {
	CLType         string   `json:"cl_type"`
	CLImage        string   `json:"cl_image"`
	ELType         string   `json:"el_type"`
	ELImage        string   `json:"el_image"`
	ELExtraParams  []string `json:"el_extra_params"`
	ELLogLevel     string   `json:"el_log_level"`
	ValidatorCount uint64   `json:"validator_count"`
}

type NetworkConfigParams struct {
	Preset           string `json:"preset"`
	ElectraForkEpoch uint64 `json:"electra_fork_epoch"`
}
