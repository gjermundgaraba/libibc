package beaconapi

import (
	"context"
	"fmt"

	"github.com/gjermundgaraba/libibc/chainclients/utils"
	"go.uber.org/zap"
)

type Client struct {
	logger *zap.Logger
	url    string
}

func NewBeaconAPIClient(logger *zap.Logger, beaconAPIAddress string) (Client, error) {
	return Client{
		logger: logger.With(zap.String("scope", "beacon-api-client")),
		url:    beaconAPIAddress,
	}, nil
}

func (b Client) GetBeaconAPIURL() string {
	return b.url
}

// blockID: Block identifier. Can be one of: "head" (canonical head in node's view), "genesis", "finalized", <slot>, <hex encoded blockRoot with 0x prefix>.
func (b Client) GetBeaconBlockHeader(ctx context.Context, blockID string) (BeaconBlockHeaderResponse, error) {
	url := fmt.Sprintf("%s/eth/v1/beacon/headers/%s", b.url, blockID)
	return utils.HttpRequest[BeaconBlockHeaderResponse](ctx, b.logger, url, "GET", nil)
}

func (b Client) GetBootstrap(ctx context.Context, finalizedRoot string) (Bootstrap, error) {
	url := fmt.Sprintf("%s/eth/v1/beacon/light_client/bootstrap/%s", b.url, finalizedRoot)

	return utils.HttpRequest[Bootstrap](ctx, b.logger, url, "GET", nil)
}

func (b Client) GetLightClientUpdates(ctx context.Context, startPeriod uint64, count uint64) (LightClientUpdatesResponse, error) {
	url := fmt.Sprintf("%s/eth/v1/beacon/light_client/updates?start_period=%d&count=%d", b.url, startPeriod, count)
	return utils.HttpRequest[LightClientUpdatesResponse](ctx, b.logger, url, "GET", nil)
}

func (b Client) GetGenesis(ctx context.Context) (Genesis, error) {
	url := fmt.Sprintf("%s/eth/v1/beacon/genesis", b.url)
	return utils.HttpRequest[Genesis](ctx, b.logger, url, "GET", nil)
}

func (b Client) GetSpec(ctx context.Context) (Spec, error) {
	url := fmt.Sprintf("%s/eth/v1/config/spec", b.url)
	return utils.HttpRequest[Spec](ctx, b.logger, url, "GET", nil)
}

func (b Client) GetFinalityUpdate(ctx context.Context) (FinalityUpdateResponse, error) {
	url := fmt.Sprintf("%s/eth/v1/beacon/light_client/finality_update", b.url)
	return utils.HttpRequest[FinalityUpdateResponse](ctx, b.logger, url, "GET", nil)
}

func (b Client) GetBeaconBlock(ctx context.Context, blockID string) (BeaconBlocksResponse, error) {
	url := fmt.Sprintf("%s/eth/v2/beacon/blocks/%s", b.url, blockID)
	return utils.HttpRequest[BeaconBlocksResponse](ctx, b.logger, url, "GET", nil)
}

func (b Client) GetFinalizedBlocks(ctx context.Context) (BeaconBlocksResponse, error) {
	resp, err := b.GetBeaconBlock(ctx, "finalized")
	if err != nil {
		return BeaconBlocksResponse{}, err
	}

	if !resp.Finalized {
		return BeaconBlocksResponse{}, fmt.Errorf("block is not finalized")
	}

	return resp, nil
}

func (b Client) GetExecutionHeight(ctx context.Context, blockID string) (uint64, error) {
	resp, err := b.GetBeaconBlock(ctx, blockID)
	if err != nil {
		return 0, err
	}

	if blockID == "finalized" && !resp.Finalized {
		return 0, fmt.Errorf("block is not finalized")
	}

	return resp.Data.Message.Body.ExecutionPayload.BlockNumber, nil
}
