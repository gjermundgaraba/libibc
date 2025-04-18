package dockerutils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"go.uber.org/zap"
)

// TODO: Clean up all the comments and rename stuff that has interchaintest or IBCTEST or whatever

const CleanupLabel = "localnet.cleanup"

// CleanupLabel is the "old" format.
// Note that any new labels should follow the reverse DNS format suggested at
// https://docs.docker.com/config/labels-custom-metadata/#key-format-recommendations.

const (
	LabelPrefix = "tech.gjermundgaraba.localnet."

	// NodeOwnerLabel indicates the logical node owning a particular object (probably a volume).
	NodeOwnerLabel = LabelPrefix + "node-owner"
)

// KeepVolumesOnFailure determines whether volumes associated with a test
// using DockerSetup are retained or deleted following a test failure.
//
// The value is false by default, but can be initialized to true by setting the
// environment variable IBCTEST_SKIP_FAILURE_CLEANUP to a non-empty value.
// Alternatively, importers of the dockerutil package may set the variable to true.
// Because dockerutil is an internal package, the public API for setting this value
// is interchaintest.KeepDockerVolumesOnFailure(bool).
var KeepVolumesOnFailure = os.Getenv("IBCTEST_SKIP_FAILURE_CLEANUP") != ""

type CleanupFunc func()

func DockerSetup(logger *zap.Logger, cleanupLabel string) (*client.Client, string, CleanupFunc) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(fmt.Errorf("failed to create docker client: %v", err))
	}

	// Clean up docker resources at end of test.
	cleanupFunc := DockerCleanup(logger, cli, cleanupLabel)

	// Also eagerly clean up any leftover resources from a previous test run,
	// e.g. if the test was interrupted.
	cleanupFunc()

	name := fmt.Sprintf("localnet-%s", RandLowerCaseLetterString(8))
	network, err := cli.NetworkCreate(context.TODO(), name, network.CreateOptions{
		Labels: map[string]string{CleanupLabel: cleanupLabel},
	})
	if err != nil {
		panic(fmt.Errorf("failed to create docker network: %v", err))
	}

	return cli, network.ID, cleanupFunc
}

// DockerCleanup will clean up Docker containers, networks, and the other various config files generated in testing
func DockerCleanup(logger *zap.Logger, cli *client.Client, cleanupLabel string) func() {
	return func() {
		showContainerLogs := os.Getenv("SHOW_CONTAINER_LOGS")
		containerLogTail := os.Getenv("CONTAINER_LOG_TAIL")
		keepContainers := os.Getenv("KEEP_CONTAINERS") != ""

		ctx := context.TODO()
		cli.NegotiateAPIVersion(ctx)
		cs, err := cli.ContainerList(ctx, container.ListOptions{
			All: true,
			Filters: filters.NewArgs(
				filters.Arg("label", CleanupLabel+"="+cleanupLabel),
			),
		})
		if err != nil {
			logger.Error("Failed to list containers during docker cleanup", zap.Error(err))
			return
		}

		for _, c := range cs {
			if showContainerLogs == "always" {
				logTail := "50"
				if containerLogTail != "" {
					logTail = containerLogTail
				}
				rc, err := cli.ContainerLogs(ctx, c.ID, container.LogsOptions{
					ShowStdout: true,
					ShowStderr: true,
					Tail:       logTail,
				})
				if err == nil {
					b := new(bytes.Buffer)
					_, err := b.ReadFrom(rc)
					if err == nil {
						logger.Info("Container logs", zap.String("container", c.ID), zap.String("logs", b.String()))
					}
				}
			}
			if !keepContainers {
				var stopTimeout container.StopOptions
				timeout := 10
				timeoutDur := time.Duration(timeout * int(time.Second))
				deadline := time.Now().Add(timeoutDur)
				stopTimeout.Timeout = &timeout
				if err := cli.ContainerStop(ctx, c.ID, stopTimeout); IsLoggableStopError(err) {
					logger.Error("Failed to stop container during docker cleanup", zap.String("container", c.ID), zap.Error(err))
				}

				waitCtx, cancel := context.WithDeadline(ctx, deadline.Add(500*time.Millisecond))
				waitCh, errCh := cli.ContainerWait(waitCtx, c.ID, container.WaitConditionNotRunning)
				select {
				case <-waitCtx.Done():
					logger.Error("Timed out waiting for container during docker cleanup", zap.String("container", c.ID))
				case err := <-errCh:
					logger.Error("Error while waiting for container during docker cleanup", zap.String("container", c.ID), zap.Error(err))
				case res := <-waitCh:
					if res.Error != nil {
						logger.Error("Error while waiting for container during docker cleanup", zap.String("container", c.ID), zap.String("error", res.Error.Message))
					}
					// Ignoring statuscode for now.
				}
				cancel()

				if err := cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{
					// Not removing volumes with the container, because we separately handle them conditionally.
					Force: true,
				}); err != nil {
					logger.Error("Failed to remove container during docker cleanup", zap.String("container", c.ID), zap.Error(err))
				}
			}
		}

		if !keepContainers {
			PruneVolumesWithRetry(ctx, logger, cli, cleanupLabel)
			PruneNetworksWithRetry(ctx, logger, cli, cleanupLabel)
		} else {
			logger.Info("Keeping containers", zap.String("message", "Docker cleanup skipped"))
		}
	}
}

func PruneVolumesWithRetry(ctx context.Context, logger *zap.Logger, cli *client.Client, cleanupLabel string) {
	if KeepVolumesOnFailure {
		return
	}

	var msg string
	err := retry.Do(
		func() error {
			res, err := cli.VolumesPrune(ctx, filters.NewArgs(filters.Arg("label", CleanupLabel+"="+cleanupLabel)))
			if err != nil {
				if errdefs.IsConflict(err) {
					// Prune is already in progress; try again.
					return err
				}

				// Give up on any other error.
				return retry.Unrecoverable(err)
			}

			if len(res.VolumesDeleted) > 0 {
				msg = fmt.Sprintf("Pruned %d volumes, reclaiming approximately %.1f MB", len(res.VolumesDeleted), float64(res.SpaceReclaimed)/(1024*1024))
			}

			return nil
		},
		retry.Context(ctx),
		retry.DelayType(retry.FixedDelay),
	)

	if err != nil {
		logger.Error("Failed to prune volumes during docker cleanup", zap.Error(err))
		return
	}

	if msg != "" {
		// Odd to Logf %s, but this is a defensive way to keep the DockerSetupTestingT interface
		// with only Logf and not need to add Log.
		logger.Info("Pruned volumes", zap.String("message", msg))
	}
}

func PruneNetworksWithRetry(ctx context.Context, logger *zap.Logger, cli *client.Client, cleanupLabel string) {
	var deleted []string
	err := retry.Do(
		func() error {
			res, err := cli.NetworksPrune(ctx, filters.NewArgs(filters.Arg("label", CleanupLabel+"="+cleanupLabel)))
			if err != nil {
				if errdefs.IsConflict(err) {
					// Prune is already in progress; try again.
					return err
				}

				// Give up on any other error.
				return retry.Unrecoverable(err)
			}

			deleted = res.NetworksDeleted
			return nil
		},
		retry.Context(ctx),
		retry.DelayType(retry.FixedDelay),
	)

	if err != nil {
		logger.Error("Failed to prune networks during docker cleanup", zap.Error(err))
		return
	}

	if len(deleted) > 0 {
		logger.Info("Pruned networks", zap.Strings("deleted", deleted))
	}
}

func IsLoggableStopError(err error) bool {
	if err == nil {
		return false
	}
	return !(errdefs.IsNotModified(err) || errdefs.IsNotFound(err))
}
