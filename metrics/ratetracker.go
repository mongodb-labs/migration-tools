package metrics

import (
	"cmp"
	"container/ring"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/ccoveille/go-safecast/v2"
	"github.com/mongodb-labs/migration-tools/option"
	"github.com/samber/lo"
	"golang.org/x/exp/constraints"
)

// RateTracker tracks a number of events per distinct key. This is useful,
// e.g., to compute rates. For example, to compute average events per second,
// you might use RateTracker[int64, int] with the key being the Unix timestamp
// in seconds.
//
// RateTracker is safe for use across multiple goroutines.
type RateTracker[keyT, countT constraints.Integer] struct {
	mu      sync.Mutex
	ring    *ring.Ring
	size    keyT
	filled  keyT
	lastKey keyT
	hasKey  bool
}

type bucket[keyT, countT constraints.Integer] struct {
	key   keyT
	count countT
}

// NewRateTracker creates a new RateTracker over the given duration.
// The duration must be at least 1 second.
func NewRateTracker[keyT, countT constraints.Integer](duration time.Duration) *RateTracker[keyT, countT] {
	lo.Assertf(
		duration >= time.Second,
		"duration (%s) must be at least 1 second",
		duration,
	)

	size := safecast.MustConvert[int](math.Floor(duration.Seconds()))

	r := ring.New(size)
	for range size {
		r.Value = &bucket[keyT, countT]{}
		r = r.Next()
	}
	return &RateTracker[keyT, countT]{ring: r, size: safecast.MustConvert[keyT](size)}
}

// Add adds the given number of events for keyT.
// It takes a lock, so callers should not call it too often.
//
// Returns an error if the given key precedes the most recent one.
func (c *RateTracker[keyT, countT]) Add(key keyT, count countT) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.hasKey {
		switch cmp.Compare(key, c.lastKey) {
		case 0:
			c.ring.Value.(*bucket[keyT, countT]).count += count
			return nil
		case -1:
			return fmt.Errorf(
				"got key %d which predates most recent one (%d)",
				key,
				c.lastKey,
			)
		}
	}

	c.ring = c.ring.Next()
	c.ring.Value = &bucket[keyT, countT]{key: key, count: count}
	c.lastKey = key
	c.hasKey = true
	if c.filled < c.size {
		c.filled++
	}
	return nil
}

// Average returns the average number of events per key given to Add(),
// or empty if no events have been recorded.
func (c *RateTracker[keyT, countT]) Average() option.Option[float64] {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.filled == 0 {
		return option.None[float64]()
	}

	var sum countT
	r := c.ring

	for i := keyT(0); i < c.filled; i++ {
		sum += r.Value.(*bucket[keyT, countT]).count
		r = r.Prev()
	}
	return option.Some(float64(sum) / float64(c.filled))
}
