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
	clusterWrite metricSet[T]
	read         metricSet[int64]
}

type metricSet[keyT constraints.Integer] struct {
	lastKey     keyT
	curKeyCount int
	firstDone   bool
	rateTracker *metrics.RateTracker[keyT, int]
}

func (ms *metricSet[keyT]) update(newKey keyT, label string) error {
	if newKey == 0 {
		return fmt.Errorf("zero key given for %#q, which is invalid", label)
	}

	var zero keyT

	if ms.lastKey == zero {
		ms.lastKey = newKey
		ms.curKeyCount = 1
		return nil
	}

	switch cmp.Compare(newKey, ms.lastKey) {
	case 0:
		ms.curKeyCount++
	case 1:
		if ms.firstDone {
			if err := ms.rateTracker.Set(ms.lastKey, ms.curKeyCount); err != nil {
				return err
			}
		} else {
			ms.firstDone = true
		}
		ms.lastKey = newKey
		ms.curKeyCount = 1
	default:
		return fmt.Errorf("%s (%v) precedes most recent %s (%v)", label, newKey, label, ms.lastKey)
	}

	return nil
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
		read:         metricSet[int64]{rateTracker: metrics.NewRateTracker[int64, int](duration)},
		clusterWrite: metricSet[T]{rateTracker: metrics.NewRateTracker[T, int](duration)},
	}
}

// Add adds a new event to the metrics. Pass the event’s clusterTime.T.
func (m *ChangeStreamMetrics[T]) Add(clusterTimeT T) error {
	if err := m.clusterWrite.update(clusterTimeT, "clusterTime.T"); err != nil {
		return err
	}
	return m.read.update(time.Now().Unix(), "wallSecond")
}

func (m *ChangeStreamMetrics[T]) EventsReadPerSecond() option.Option[float64] {
	return m.read.rateTracker.Average()
}

func (m *ChangeStreamMetrics[T]) ClusterEventsPerSecond() option.Option[float64] {
	return m.clusterWrite.rateTracker.Average()
}
