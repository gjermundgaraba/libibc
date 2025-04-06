package skipapi

type RouteRequest struct {
	SourceAssetDenom     string   `json:"source_asset_denom"`
	SourceAssetChainID   string   `json:"source_asset_chain_id"`
	DestAssetDenom       string   `json:"dest_asset_denom"`
	DestAssetChainID     string   `json:"dest_asset_chain_id"`
	AllowUnsafe          bool     `json:"allow_unsafe"`
	ExperimentalFeatures []string `json:"experimental_features"`
	AllowMultiTx         bool     `json:"allow_multi_tx"`
	SmartRelay           bool     `json:"smart_relay"`
	SmartSwapOptions     struct {
		SplitRoutes bool `json:"split_routes"`
		EvmSwaps    bool `json:"evm_swaps"`
	} `json:"smart_swap_options"`
	GoFast   bool   `json:"go_fast"`
	AmountIn string `json:"amount_in"`
}
