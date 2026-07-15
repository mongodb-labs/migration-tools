package metrics

import (
	"cmp"
	"fmt"
	"time"

	"github.com/mongodb-labs/migration-tools/metrics"
	"github.com/mongodb-labs/migration-tools/option"
)

// ChangeStreamMetrics tracks metrics for a change stream. It is *not*
// thread-safe, so use it only from a single goroutine.
type ChangeStreamMetrics struct {
	lastClusterTimeT    uint32
	curClusterTimeCount int
	lastWallSecond      int64
	curWallSecondCount  int

	readTracker         *metrics.RateTracker[int64, int]
	clusterWriteTracker *metrics.RateTracker[uint32, int]
}

// NewChangeStreamMetrics creates a new ChangeStreamMetrics that computes
// metrics over the given duration. The duration must be at least 1 second.
func NewChangeStreamMetrics(
	duration time.Duration,
) *ChangeStreamMetrics {
	return &ChangeStreamMetrics{
		readTracker:         metrics.NewRateTracker[int64, int](duration),
		clusterWriteTracker: metrics.NewRateTracker[uint32, int](duration),
	}
}

func (m *ChangeStreamMetrics) Add(clusterTimeT uint32) error {
	if err := trackBucket(clusterTimeT, &m.lastClusterTimeT, &m.curClusterTimeCount, m.clusterWriteTracker.Add, "clusterTime.T"); err != nil {
		return err
	}
	return trackBucket(time.Now().Unix(), &m.lastWallSecond, &m.curWallSecondCount, m.readTracker.Add, "wallSecond")
}

func trackBucket[T cmp.Ordered](
	current T,
	last *T,
	count *int,
	add func(T, int) error,
	label string,
) error {
	var zero T

	if *last == zero {
		*last = current
	}

	switch cmp.Compare(current, *last) {
	case 0:
		*count++
	case 1:
		if err := add(*last, *count); err != nil {
			return err
		}
		*last = current
		*count = 1
	default:
		return fmt.Errorf("%s (%v) precedes most recent %s (%v)", label, current, label, *last)
	}

	return nil
}

func (m *ChangeStreamMetrics) EventsReadPerSecond() option.Option[float64] {
	return m.readTracker.Average()
}

func (m *ChangeStreamMetrics) ClusterEventsPerSecond() option.Option[float64] {
	return m.clusterWriteTracker.Average()
}
