package skipapi

import "time"

type RouteRequest struct {
	SourceAssetDenom     string           `json:"source_asset_denom"`
	SourceAssetChainID   string           `json:"source_asset_chain_id"`
	DestAssetDenom       string           `json:"dest_asset_denom"`
	DestAssetChainID     string           `json:"dest_asset_chain_id"`
	AllowUnsafe          bool             `json:"allow_unsafe"`
	ExperimentalFeatures []string         `json:"experimental_features"`
	AllowMultiTx         bool             `json:"allow_multi_tx"`
	SmartRelay           bool             `json:"smart_relay"`
	SmartSwapOptions     SmartSwapOptions `json:"smart_swap_options"`
	GoFast               bool             `json:"go_fast"`
	AmountIn             string           `json:"amount_in"`
}

type SmartSwapOptions struct {
	SplitRoutes bool `json:"split_routes"`
	EvmSwaps    bool `json:"evm_swaps"`
}

type RouteResponse struct {
	AmountIn                      string      `json:"amount_in"`
	AmountOut                     string      `json:"amount_out"`
	ChainIds                      []string    `json:"chain_ids"`
	DestAssetChainID              string      `json:"dest_asset_chain_id"`
	DestAssetDenom                string      `json:"dest_asset_denom"`
	DoesSwap                      bool        `json:"does_swap"`
	EstimatedAmountOut            string      `json:"estimated_amount_out"`
	EstimatedFees                 []any       `json:"estimated_fees"`
	EstimatedRouteDurationSeconds int         `json:"estimated_route_duration_seconds"`
	Operations                    []Operation `json:"operations"`
	RequiredChainAddresses        []string    `json:"required_chain_addresses"`
	SourceAssetChainID            string      `json:"source_asset_chain_id"`
	SourceAssetDenom              string      `json:"source_asset_denom"`
	SwapVenues                    []any       `json:"swap_venues"`
	TxsRequired                   int         `json:"txs_required"`
}

type MsgsRequest struct {
	SourceAssetDenom         string               `json:"source_asset_denom"`
	SourceAssetChainID       string               `json:"source_asset_chain_id"`
	DestAssetDenom           string               `json:"dest_asset_denom"`
	DestAssetChainID         string               `json:"dest_asset_chain_id"`
	AmountIn                 string               `json:"amount_in"`
	AmountOut                string               `json:"amount_out"`
	AddressList              []string             `json:"address_list"`
	Operations               []Operation          `json:"operations"`
	EstimatedAmountOut       string               `json:"estimated_amount_out"`
	SlippageTolerancePercent string               `json:"slippage_tolerance_percent"`
	ChainIdsToAffiliates     ChainIdsToAffiliates `json:"chain_ids_to_affiliates"`
}

type Operation struct {
	TxIndex        int            `json:"tx_index"`
	AmountIn       string         `json:"amount_in"`
	AmountOut      string         `json:"amount_out"`
	EurekaTransfer EurekaTransfer `json:"eureka_transfer"`
}

type EurekaTransfer struct {
	DestinationPort                string             `json:"destination_port"`
	SourceClient                   string             `json:"source_client"`
	FromChainID                    string             `json:"from_chain_id"`
	ToChainID                      string             `json:"to_chain_id"`
	PfmEnabled                     bool               `json:"pfm_enabled"`
	SupportsMemo                   bool               `json:"supports_memo"`
	EntryContractAddress           string             `json:"entry_contract_address"`
	CallbackAdapterContractAddress string             `json:"callback_adapter_contract_address"`
	DenomIn                        string             `json:"denom_in"`
	DenomOut                       string             `json:"denom_out"`
	BridgeID                       string             `json:"bridge_id"`
	SmartRelay                     bool               `json:"smart_relay"`
	SmartRelayFeeQuote             SmartRelayFeeQuote `json:"smart_relay_fee_quote"`
}

type SmartRelayFeeQuote struct {
	FeeAmount         string    `json:"fee_amount"`
	RelayerAddress    string    `json:"relayer_address"`
	Expiration        time.Time `json:"expiration"`
	FeePaymentAddress string    `json:"fee_payment_address"`
	FeeDenom          string    `json:"fee_denom"`
}

type ChainIdsToAffiliates struct{}

type MsgsResponse struct {
	EstimatedFees []any `json:"estimated_fees"`
	Msgs          []struct {
		MultiChainMsg *MultiChainMsg `json:"multi_chain_msg,omitempty"`
		EvmTx         *EvmTx         `json:"evm_tx,omitempty"`
	} `json:"msgs"`
	Txs []struct {
		CosmosTx          *CosmosTx `json:"cosmos_tx,omitempty"`
		EvmTx             *EvmTx    `json:"evm_tx,omitempty"`
		OperationsIndices []int     `json:"operations_indices"`
	} `json:"txs"`
}

type CosmosTx struct {
	ChainID       string        `json:"chain_id"`
	Msgs          []CosmosTxMsg `json:"msgs"`
	Path          []string      `json:"path"`
	SignerAddress string        `json:"signer_address"`
}

type EvmTx struct {
	ChainID                string                  `json:"chain_id"`
	To                     string                  `json:"to"`
	Value                  string                  `json:"value"`
	Data                   string                  `json:"data"`
	RequiredErc20Approvals []RequiredErc20Approval `json:"required_erc20_approvals"`
	SignerAddress          string                  `json:"signer_address"`
}

type MultiChainMsg struct {
	ChainID    string   `json:"chain_id"`
	Msg        string   `json:"msg"`
	MsgTypeURL string   `json:"msg_type_url"`
	Path       []string `json:"path"`
}

type RequiredErc20Approval struct {
	TokenContract string `json:"token_contract"`
	Spender       string `json:"spender"`
	Amount        string `json:"amount"`
}

type CosmosTxMsg struct {
	Msg        string `json:"msg"`
	MsgTypeURL string `json:"msg_type_url"`
}
