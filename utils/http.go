package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func HttpRequest[T any](ctx context.Context, logger *zap.Logger, url string, method string, req any) (T, error) {
	httpClient := &http.Client{}
	logger.Debug("sending request to skip api", zap.String("url", url), zap.Any("req", req))

	var resp T
	requestBody, err := json.Marshal(req)
	if err != nil {
		return resp, errors.Wrapf(err, "failed to marshal request body")
	}

	httpRequest, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(requestBody))
	if err != nil {
		return resp, errors.Wrapf(err, "failed to create request to %s", url)
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	response, err := httpClient.Do(httpRequest)
	if err != nil {
		return resp, errors.Wrapf(err, "failed to send request to %s with req=%+v", url, req)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return resp, errors.Errorf("unexpected status code: %d for %s with req=%+v", response.StatusCode, url, req)
	}

	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		return resp, errors.Wrapf(err, "failed to decode response body")
	}

	logger.Debug("received response from skip api", zap.String("url", url), zap.Any("resp", resp))

	return resp, nil
}
