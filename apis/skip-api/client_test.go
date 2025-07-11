package skipapi

import (
	"testing"

	"github.com/gjermundgaraba/libibc/chains/cosmos"
	"github.com/stretchr/testify/require"
)

func TestCosmosMsgToAny(t *testing.T) {
	cdc := cosmos.SetupCodec()
	cosmosMsg := CosmosTxMsg{
		Msg:        "{\"sender\":\"cosmos14tvy7waxghv7sxv0h79tpv20hnt5977gtkwltu\",\"contract\":\"cosmos1uq4ztnt3lrtwx0ryjtvy66ncxd2q92fdg78mgxcr76mm2582xkwsqwrjr4\",\"msg\":{\"action\":{\"timeout_timestamp\":1744038009,\"action\":{\"ibc_transfer\":{\"ibc_info\":{\"source_channel\":\"08-wasm-262\",\"receiver\":\"0xAe3E5CCaF3216de61090E68Cf5a191f3b75CaAd3\",\"memo\":\"\",\"recover_address\":\"cosmos14tvy7waxghv7sxv0h79tpv20hnt5977gtkwltu\",\"encoding\":\"application/x-solidity-abi\"}}},\"exact_out\":false}},\"funds\":[{\"denom\":\"uatom\",\"amount\":\"100000\"}]}",
		MsgTypeURL: "/cosmwasm.wasm.v1.MsgExecuteContract",
	}

	any, err := cosmosMsgToAny(cdc, cosmosMsg)
	require.NoError(t, err)
	require.NotNil(t, any)
}

func TestCosmosTxToNewTx(t *testing.T) {
	cdc := cosmos.SetupCodec()
	cosmosTx := CosmosTx{
		ChainID: "doesntmatter",
		Msgs: []CosmosTxMsg{
			{
				Msg:        "{\"sender\":\"cosmos14tvy7waxghv7sxv0h79tpv20hnt5977gtkwltu\",\"contract\":\"cosmos1uq4ztnt3lrtwx0ryjtvy66ncxd2q92fdg78mgxcr76mm2582xkwsqwrjr4\",\"msg\":{\"action\":{\"timeout_timestamp\":1744038009,\"action\":{\"ibc_transfer\":{\"ibc_info\":{\"source_channel\":\"08-wasm-262\",\"receiver\":\"0xAe3E5CCaF3216de61090E68Cf5a191f3b75CaAd3\",\"memo\":\"\",\"recover_address\":\"cosmos14tvy7waxghv7sxv0h79tpv20hnt5977gtkwltu\",\"encoding\":\"application/x-solidity-abi\"}}},\"exact_out\":false}},\"funds\":[{\"denom\":\"uatom\",\"amount\":\"100000\"}]}",
				MsgTypeURL: "/cosmwasm.wasm.v1.MsgExecuteContract",
			}},
		Path:          []string{},
		SignerAddress: "",
	}

	newTx, err := cosmosTxToNewTx(cdc, cosmosTx)
	require.NoError(t, err)
	require.NotNil(t, newTx)
	require.NotNil(t, newTx.GetTxBytes())
}
