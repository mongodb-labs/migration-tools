package changestream

import (
	"context"
	"fmt"

	"github.com/mongodb-labs/migration-tools/contextplus"
)

// readEachChannelOnce reads exactly one value from each channel concurrently,
// respecting ctx. Returns the values in the same order as the input channels.
// If any read fails, the remaining reads are cancelled.
func readEachChannelOnce[T any](ctx context.Context, chans ...<-chan T) ([]T, error) {
	results := make([]T, len(chans))
	g, ctx := contextplus.ErrGroup(ctx)
	for i, ch := range chans {
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case v, ok := <-ch:
				if !ok {
					return fmt.Errorf("channel %d closed unexpectedly", i)
				}
				fmt.Printf("----- channel result: %+v\n", v)
				results[i] = v
				return nil
			}
		})
	}
	return results, g.Wait()
}
