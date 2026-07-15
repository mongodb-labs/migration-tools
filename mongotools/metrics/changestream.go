package metrics

import (
	"cmp"
	"fmt"
	"time"

	"github.com/mongodb-labs/migration-tools/metrics"
	"github.com/mongodb-labs/migration-tools/option"
	"golang.org/x/exp/constraints"
)

// ChangeStreamMetrics tracks metrics for a change stream. It is *not*
// thread-safe, so use it only from a single goroutine.
type ChangeStreamMetrics[T constraints.Integer] struct {
	lastClusterTimeT    T
	curClusterTimeCount int
	clusterWriteTracker *metrics.RateTracker[T, int]

	lastWallSecond     int64
	curWallSecondCount int
	readTracker        *metrics.RateTracker[int64, int]
}

// NewChangeStreamMetrics creates a new ChangeStreamMetrics that computes
// metrics over the given duration. The duration must be at least 1 second.
//
// The type parameter is the type of whatever you’re using to track server time.
// You should probably use the wallTime, which makes int64 a good choice.
// You could alternatively use the clusterTime.T, which would be uint32.
func NewChangeStreamMetrics[T constraints.Integer](
	duration time.Duration,
) *ChangeStreamMetrics[T] {
	return &ChangeStreamMetrics[T]{
		readTracker:         metrics.NewRateTracker[int64, int](duration),
		clusterWriteTracker: metrics.NewRateTracker[T, int](duration),
	}
}

// Add adds a new event to the metrics. Pass the event’s clusterTime.T.
func (m *ChangeStreamMetrics[T]) Add(clusterTimeT T) error {
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
		*count = 1
		return nil
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

func (m *ChangeStreamMetrics[T]) EventsReadPerSecond() option.Option[float64] {
	return m.readTracker.Average()
}

func (m *ChangeStreamMetrics[T]) ClusterEventsPerSecond() option.Option[float64] {
	return m.clusterWriteTracker.Average()
}
