package beaconapi

import (
	"fmt"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
)

type Version [4]byte
type Root [32]byte

// String returns a string version of the structure.
func (r Root) String() string {
	return fmt.Sprintf("%#x", r)
}

// Genesis provides information about the genesis of a chain.
type Genesis struct {
	GenesisTime           time.Time
	GenesisValidatorsRoot Root
	GenesisForkVersion    Version
}

type Spec struct {
	SecondsPerSlot               time.Duration `json:"SECONDS_PER_SLOT"`
	SlotsPerEpoch                uint64        `json:"SLOTS_PER_EPOCH"`
	EpochsPerSyncCommitteePeriod uint64        `json:"EPOCHS_PER_SYNC_COMMITTEE_PERIOD"`

	// Fork Parameters
	GenesisForkVersion   Version `json:"GENESIS_FORK_VERSION"`
	GenesisSlot          uint64  `json:"GENESIS_SLOT"`
	AltairForkVersion    Version `json:"ALTAIR_FORK_VERSION"`
	AltairForkEpoch      uint64  `json:"ALTAIR_FORK_EPOCH"`
	BellatrixForkVersion Version `json:"BELLATRIX_FORK_VERSION"`
	BellatrixForkEpoch   uint64  `json:"BELLATRIX_FORK_EPOCH"`
	CapellaForkVersion   Version `json:"CAPELLA_FORK_VERSION"`
	CapellaForkEpoch     uint64  `json:"CAPELLA_FORK_EPOCH"`
	DenebForkVersion     Version `json:"DENEB_FORK_VERSION"`
	DenebForkEpoch       uint64  `json:"DENEB_FORK_EPOCH"`
	ElectraForkVersion   Version `json:"ELECTRA_FORK_VERSION"`
	ElectraForkEpoch     uint64  `json:"ELECTRA_FORK_EPOCH"`
}

func (s Spec) ToForkParameters() ForkParameters {
	return ForkParameters{
		GenesisForkVersion: ethcommon.Bytes2Hex(s.GenesisForkVersion[:]),
		GenesisSlot:        s.GenesisSlot,
		Altair: Fork{
			Version: ethcommon.Bytes2Hex(s.AltairForkVersion[:]),
			Epoch:   s.AltairForkEpoch,
		},
		Bellatrix: Fork{
			Version: ethcommon.Bytes2Hex(s.BellatrixForkVersion[:]),
			Epoch:   s.BellatrixForkEpoch,
		},
		Capella: Fork{
			Version: ethcommon.Bytes2Hex(s.CapellaForkVersion[:]),
			Epoch:   s.CapellaForkEpoch,
		},
		Deneb: Fork{
			Version: ethcommon.Bytes2Hex(s.DenebForkVersion[:]),
			Epoch:   s.DenebForkEpoch,
		},
		Electra: Fork{
			Version: ethcommon.Bytes2Hex(s.ElectraForkVersion[:]),
			Epoch:   s.ElectraForkEpoch,
		},
	}
}

func (s Spec) Period() uint64 {
	return s.EpochsPerSyncCommitteePeriod * s.SlotsPerEpoch
}

// The fork parameters
type ForkParameters struct {
	// The altair fork
	Altair Fork `json:"altair"`
	// The bellatrix fork
	Bellatrix Fork `json:"bellatrix"`
	// The capella fork
	Capella Fork `json:"capella"`
	// The deneb fork
	Deneb Fork `json:"deneb"`
	// The electra fork
	Electra Fork `json:"electra"`
	// The genesis fork version
	GenesisForkVersion string `json:"genesis_fork_version"`
	// The genesis slot
	GenesisSlot uint64 `json:"genesis_slot"`
}

type Fork struct {
	// The epoch at which this fork is activated
	Epoch uint64 `json:"epoch"`
	// The version of the fork
	Version string `json:"version"`
}

type Bootstrap struct {
	Data struct {
		Header               BootstrapHeader `json:"header"`
		CurrentSyncCommittee SyncCommittee   `json:"current_sync_committee"`
	} `json:"data"`
}

type BootstrapHeader struct {
	Beacon    Beacon    `json:"beacon"`
	Execution Execution `json:"execution"`
}

type SyncCommittee struct {
	Pubkeys         []string `json:"pubkeys"`
	AggregatePubkey string   `json:"aggregate_pubkey"`
}

type LightClientUpdatesResponse []LightClientUpdate

type Beacon struct {
	Slot          uint64 `json:"slot,string"`
	ProposerIndex uint64 `json:"proposer_index,string"`
	ParentRoot    string `json:"parent_root"`
	StateRoot     string `json:"state_root"`
	BodyRoot      string `json:"body_root"`
}

type Execution struct {
	ParentHash       string `json:"parent_hash"`
	FeeRecipient     string `json:"fee_recipient"`
	StateRoot        string `json:"state_root"`
	ReceiptsRoot     string `json:"receipts_root"`
	LogsBloom        string `json:"logs_bloom"`
	PrevRandao       string `json:"prev_randao"`
	BlockNumber      uint64 `json:"block_number,string"`
	GasLimit         uint64 `json:"gas_limit,string"`
	GasUsed          uint64 `json:"gas_used,string"`
	Timestamp        uint64 `json:"timestamp,string"`
	ExtraData        string `json:"extra_data"`
	BaseFeePerGas    uint64 `json:"base_fee_per_gas,string"`
	BlockHash        string `json:"block_hash"`
	TransactionsRoot string `json:"transactions_root"`
	WithdrawalsRoot  string `json:"withdrawals_root"`
	BlobGasUsed      uint64 `json:"blob_gas_used,string"`
	ExcessBlobGas    uint64 `json:"excess_blob_gas,string"`
}

type FinalityUpdateResponse struct {
	Version string            `json:"version"`
	Data    LightClientUpdate `json:"data"`
}

type BeaconBlocksResponse struct {
	ExecutionOptimistic bool `json:"execution_optimistic"`
	Finalized           bool `json:"finalized"`
	Data                struct {
		Message struct {
			Slot          string `json:"slot"`
			ProposerIndex string `json:"proposer_index"`
			ParentRoot    string `json:"parent_root"`
			StateRoot     string `json:"state_root"`
			Body          struct {
				RandaoReveal string `json:"randao_reveal"`
				Eth1Data     struct {
					DepositRoot  string `json:"deposit_root"`
					DepositCount string `json:"deposit_count"`
					BlockHash    string `json:"block_hash"`
				} `json:"eth1_data"`
				Graffiti          string `json:"graffiti"`
				ProposerSlashings []any  `json:"proposer_slashings"`
				AttesterSlashings []any  `json:"attester_slashings"`
				Attestations      []any  `json:"attestations"`
				Deposits          []any  `json:"deposits"`
				VoluntaryExits    []any  `json:"voluntary_exits"`
				SyncAggregate     struct {
					SyncCommitteeBits      string `json:"sync_committee_bits"`
					SyncCommitteeSignature string `json:"sync_committee_signature"`
				} `json:"sync_aggregate"`
				ExecutionPayload struct {
					ParentHash    string `json:"parent_hash"`
					FeeRecipient  string `json:"fee_recipient"`
					StateRoot     string `json:"state_root"`
					ReceiptsRoot  string `json:"receipts_root"`
					LogsBloom     string `json:"logs_bloom"`
					PrevRandao    string `json:"prev_randao"`
					BlockNumber   uint64 `json:"block_number,string"`
					GasLimit      string `json:"gas_limit"`
					GasUsed       string `json:"gas_used"`
					Timestamp     string `json:"timestamp"`
					ExtraData     string `json:"extra_data"`
					BaseFeePerGas string `json:"base_fee_per_gas"`
					BlockHash     string `json:"block_hash"`
					Transactions  []any  `json:"transactions"`
					Withdrawals   []any  `json:"withdrawals"`
					BlobGasUsed   string `json:"blob_gas_used"`
					ExcessBlobGas string `json:"excess_blob_gas"`
				} `json:"execution_payload"`
				BlsToExecutionChanges []any `json:"bls_to_execution_changes"`
				BlobKzgCommitments    []any `json:"blob_kzg_commitments"`
			} `json:"body"`
		} `json:"message"`
		Signature string `json:"signature"`
	} `json:"data"`
}

type LightClientUpdateResponse struct {
	Data LightClientUpdate `json:"data"`
}

// The consensus update
//
// A light client update
type LightClientUpdate struct {
	// Header attested to by the sync committee
	AttestedHeader LightClientHeader `json:"attested_header"`
	// Branch of the finalized header
	FinalityBranch []string `json:"finality_branch"`
	// Finalized header corresponding to `attested_header.state_root`
	FinalizedHeader LightClientHeader `json:"finalized_header"`
	// Next sync committee corresponding to `attested_header.state_root`
	NextSyncCommittee *SyncCommittee `json:"next_sync_committee"`
	// The branch of the next sync committee
	NextSyncCommitteeBranch []string `json:"next_sync_committee_branch"`
	// Slot at which the aggregate signature was created (untrusted)
	SignatureSlot string `json:"signature_slot"`
	// Sync committee aggregate signature
	SyncAggregate SyncAggregate `json:"sync_aggregate"`
}

// Sync committee aggregate signature
//
// The sync committee aggregate
type SyncAggregate struct {
	// The bits representing the sync committee's participation.
	SyncCommitteeBits string `json:"sync_committee_bits"`
	// The aggregated signature of the sync committee.
	SyncCommitteeSignature string `json:"sync_committee_signature"`
}

// Header attested to by the sync committee
//
// # The header of a light client
//
// Finalized header corresponding to `attested_header.state_root`
type LightClientHeader struct {
	// The beacon block header
	Beacon BeaconBlockHeader `json:"beacon"`
	// The execution payload header
	Execution ExecutionPayloadHeader `json:"execution"`
	// The execution branch
	ExecutionBranch []string `json:"execution_branch"`
}

// The beacon block header response object
type BeaconBlockHeaderResponse struct {
	ExecutionOptimistic bool `json:"execution_optimistic"`
	Finalized           bool `json:"finalized"`
	Data                struct {
		Root      Root              `json:"root"`
		Canonical bool              `json:"canonical"`
		Header    BeaconBlockHeader `json:"header"`
	} `json:"data"`
}

// The beacon block header
type BeaconBlockHeader struct {
	// The tree hash merkle root of the `BeaconBlockBody` for the `BeaconBlock`
	BodyRoot string `json:"body_root"`
	// The signing merkle root of the parent `BeaconBlock`
	ParentRoot string `json:"parent_root"`
	// The index of validator in validator registry
	ProposerIndex string `json:"proposer_index"`
	// The slot to which this block corresponds
	Slot string `json:"slot"`
	// The tree hash merkle root of the `BeaconState` for the `BeaconBlock`
	StateRoot string `json:"state_root"`
}

// The execution payload header
//
// Header to track the execution block
type ExecutionPayloadHeader struct {
	// Block base fee per gas
	BaseFeePerGas string `json:"base_fee_per_gas"`
	// Blob gas used (new in Deneb)
	BlobGasUsed string `json:"blob_gas_used"`
	// The block hash
	BlockHash string `json:"block_hash"`
	// The block number of the execution payload
	BlockNumber string `json:"block_number"`
	// Excess blob gas (new in Deneb)
	ExcessBlobGas string `json:"excess_blob_gas"`
	// The extra data of the execution payload
	ExtraData string `json:"extra_data"`
	// Block fee recipient
	FeeRecipient string `json:"fee_recipient"`
	// Execution block gas limit
	GasLimit string `json:"gas_limit"`
	// Execution block gas used
	GasUsed string `json:"gas_used"`
	// The logs bloom filter
	LogsBloom string `json:"logs_bloom"`
	// The parent hash of the execution payload header
	ParentHash string `json:"parent_hash"`
	// The previous Randao value, used to compute the randomness on the execution layer.
	PrevRandao string `json:"prev_randao"`
	// The root of the receipts trie
	ReceiptsRoot string `json:"receipts_root"`
	// The state root
	StateRoot string `json:"state_root"`
	// The timestamp of the execution payload
	Timestamp string `json:"timestamp"`
	// SSZ hash tree root of the transaction list
	TransactionsRoot string `json:"transactions_root"`
	// Tree root of the withdrawals list
	WithdrawalsRoot string `json:"withdrawals_root"`
}
