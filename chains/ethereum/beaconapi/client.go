package beaconapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	ctx    context.Context
	cancel context.CancelFunc

	url string

	Retries   int
	RetryWait time.Duration
}

func NewBeaconAPIClient(beaconAPIAddress string) (Client, error) {
	return Client{
		url:       beaconAPIAddress,
		Retries:   60,
		RetryWait: 10 * time.Second,
	}, nil
}

func (b Client) GetBeaconAPIURL() string {
	return b.url
}

func (b Client) Close() {
	b.cancel()
}

func makeRequest[T any](url string, retries int, waitTime time.Duration) (*T, error) {
	return retry(retries, waitTime, func() (*T, error) {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("get bootstrap (%s) failed with status code: %d, body: %s", url, resp.StatusCode, body)
		}

		var data T
		if err := json.Unmarshal(body, &data); err != nil {
			return nil, err
		}

		return &data, nil
	})
}

func retry[T any](retries int, waitTime time.Duration, fn func() (T, error)) (T, error) {
	var err error
	var result T
	for range retries {
		result, err = fn()
		if err == nil {
			return result, nil
		}

		fmt.Printf("Retrying for %T: %s in %f seconds\n", result, err, waitTime.Seconds())
		time.Sleep(waitTime)
	}
	return result, err
}

// blockID: Block identifier. Can be one of: "head" (canonical head in node's view), "genesis", "finalized", <slot>, <hex encoded blockRoot with 0x prefix>.
func (b Client) GetBeaconBlockHeader(blockID string) (*BeaconBlockHeaderResponse, error) {
	url := fmt.Sprintf("%s/eth/v1/beacon/headers/%s", b.url, blockID)
	return makeRequest[BeaconBlockHeaderResponse](url, b.Retries, b.RetryWait)
}

func (b Client) GetBootstrap(finalizedRoot Root) (*Bootstrap, error) {
	finalizedRootStr := finalizedRoot.String()
	url := fmt.Sprintf("%s/eth/v1/beacon/light_client/bootstrap/%s", b.url, finalizedRootStr)

	return makeRequest[Bootstrap](url, b.Retries, b.RetryWait)
}

func (b Client) GetLightClientUpdates(startPeriod uint64, count uint64) (*LightClientUpdatesResponse, error) {
	url := fmt.Sprintf("%s/eth/v1/beacon/light_client/updates?start_period=%d&count=%d", b.url, startPeriod, count)
	return makeRequest[LightClientUpdatesResponse](url, b.Retries, b.RetryWait)
}

func (b Client) GetGenesis() (*Genesis, error) {
	url := fmt.Sprintf("%s/eth/v1/beacon/genesis", b.url)
	return makeRequest[Genesis](url, b.Retries, b.RetryWait)
}

func (b Client) GetSpec() (*Spec, error) {
	url := fmt.Sprintf("%s/eth/v1/config/spec", b.url)
	return makeRequest[Spec](url, b.Retries, b.RetryWait)
}

func (b Client) GetFinalityUpdate() (*FinalityUpdateResponse, error) {
	url := fmt.Sprintf("%s/eth/v1/beacon/light_client/finality_update", b.url)
	return makeRequest[FinalityUpdateResponse](url, b.Retries, b.RetryWait)
}

func (b Client) GetBeaconBlock(blockID string) (*BeaconBlocksResponse, error) {
	url := fmt.Sprintf("%s/eth/v2/beacon/blocks/%s", b.url, blockID)
	return makeRequest[BeaconBlocksResponse](url, b.Retries, b.RetryWait)
}

func (b Client) GetFinalizedBlocks() (*BeaconBlocksResponse, error) {
	resp, err := b.GetBeaconBlock("finalized")
	if err != nil {
		return nil, err
	}

	if !resp.Finalized {
		return nil, fmt.Errorf("block is not finalized")
	}

	return resp, nil
}

func (b Client) GetExecutionHeight(blockID string) (uint64, error) {
	resp, err := b.GetBeaconBlock(blockID)
	if err != nil {
		return 0, err
	}

	if blockID == "finalized" && !resp.Finalized {
		return 0, fmt.Errorf("block is not finalized")
	}

	return resp.Data.Message.Body.ExecutionPayload.BlockNumber, nil
}
