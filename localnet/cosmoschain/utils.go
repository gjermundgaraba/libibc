package cosmoschain

import (
	"context"

	"golang.org/x/sync/errgroup"
)

type height struct {
	Chain *Node

	starting int64
	current  int64
}

// WaitForBlocks blocks until all chains reach a block height delta equal to or greater than the delta argument.
// If a ChainHeighter does not monotonically increase the height, this function may block program execution indefinitely.
func WaitForBlocks(ctx context.Context, delta int, chains ...*Node) error {
	if len(chains) == 0 {
		panic("missing chains")
	}
	eg, egCtx := errgroup.WithContext(ctx)
	for i := range chains {
		chain := chains[i]
		eg.Go(func() error {
			h := &height{Chain: chain}
			return h.WaitForDelta(egCtx, delta)
		})
	}
	return eg.Wait()
}

func (h *height) WaitForDelta(ctx context.Context, delta int) error {
	for h.delta() < delta {
		cur, err := h.Chain.Height(ctx)
		if err != nil {
			return err
		}
		// We assume the chain will eventually return a non-zero height, otherwise
		// this may block indefinitely.
		if cur == 0 {
			continue
		}
		h.update(cur)
	}
	return nil
}

func (h *height) delta() int {
	if h.starting == 0 {
		return 0
	}
	return int(h.current - h.starting)
}

func (h *height) update(height int64) {
	if h.starting == 0 {
		h.starting = height
	}
	h.current = height
}
