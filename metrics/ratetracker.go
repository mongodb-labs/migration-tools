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
func NewRateTracker[keyT, countT constraints.Integer](
	duration time.Duration,
) *RateTracker[keyT, countT] {
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

// Set sets the count for key. If the key matches the most recent one, its
// count is replaced. If the key advances, the ring moves forward.
// It takes a lock, so over-frequent calls will cause contention across
// goroutines. So don’t do that.
//
// Returns an error if the given key precedes the most recent one.
func (c *RateTracker[keyT, countT]) Set(key keyT, count countT) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.hasKey {
		switch cmp.Compare(key, c.lastKey) {
		case 0:
			c.ring.Value.(*bucket[keyT, countT]).count = count
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

// AverageBefore computes the average number of events per key from the first
// key to the given exclusive limit. This lets you account for gaps in the keys.
//
// For example, if you have keys 1 and 3, and you call AverageBefore(5),
// the average will be (count1 + count3) / 4, because keys 2 and 4 are gaps.
func (c *RateTracker[keyT, countT]) AverageBefore(excLimit keyT) (option.Option[float64], error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.filled == 0 {
		return option.None[float64](), nil
	}

	newestKey := c.ring.Value.(*bucket[keyT, countT]).key
	if excLimit <= newestKey {
		return option.None[float64](), fmt.Errorf(
			"excLimit (%v) must be strictly greater than newest key (%v)",
			excLimit, newestKey,
		)
	}

	var sum countT
	var oldestKey keyT
	r := c.ring

	for i := keyT(0); i < c.filled; i++ {
		b := r.Value.(*bucket[keyT, countT])
		sum += b.count
		oldestKey = b.key
		r = r.Prev()
	}

	span := excLimit - oldestKey
	return option.Some(float64(sum) / float64(span)), nil
}
