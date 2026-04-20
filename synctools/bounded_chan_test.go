package synctools

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type boundedChanTestSuite struct {
	suite.Suite
}

func TestBoundedChanTestSuite(t *testing.T) {
	suite.Run(t, &boundedChanTestSuite{})
}

func (s *boundedChanTestSuite) TestBasicSendReceive() {
	out, in, _ := NewBoundedChan(10, 1000, func(int) int64 { return 1 })

	// Send a few items
	for i := range 5 {
		in <- i
	}

	// Receive them back
	for i := range 5 {
		val := <-out
		s.Assert().Equal(i, val)
	}
}

func (s *boundedChanTestSuite) TestCountLimitEnforced() {
	maxCount := 3
	out, in, _ := NewBoundedChan(maxCount, 10000, func(int) int64 { return 1 })

	// Send maxCount + 1 items in a goroutine to avoid deadlock
	go func() {
		for i := range maxCount + 1 {
			in <- i
		}
		close(in)
	}()

	// When buffer hits limit and we send another, the first should be drained
	// Collect all outputs to verify they come in order
	items := []int{}
	for item := range out {
		items = append(items, item)
	}

	s.Assert().Len(items, maxCount+1)
	// Verify items came in order
	for i := range items {
		s.Assert().Equal(i, items[i])
	}
}

func (s *boundedChanTestSuite) TestMemoryLimitEnforced() {
	// maxCount is high (won't trigger); only memory limit should bound the buffer.
	// Each item costs 20 bytes, maxMem=60. Three items reach exactly 60 bytes,
	// triggering forced drain (>=). A fourth item cannot be accepted until one
	// is drained, so peak BufferedBytes should never exceed maxMem.
	//
	// A concurrent observer tracks peak BufferedBytes.
	// With enforcement (>=): peak ≤ 60.
	// Without enforcement: peak could reach 100 × 20 = 2000.
	const maxMem = 60
	const itemSize = 20
	out, in, stats := NewBoundedChan(100, maxMem, func(int) int64 { return itemSize })

	// Observer: track peak BufferedBytes.
	var peakBytes atomic.Int64
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				cur := stats().BufferedBytes
				for old := peakBytes.Load(); cur > old; old = peakBytes.Load() {
					peakBytes.CompareAndSwap(old, cur)
				}
				runtime.Gosched()
			}
		}
	}()

	// Producer: send 100 items.
	go func() {
		for range 100 {
			in <- 0
		}
		close(in)
	}()

	received := consumeItems(out)
	close(done)

	s.Assert().Equal(100, received)
	s.Assert().LessOrEqual(peakBytes.Load(), int64(maxMem),
		"peak buffered bytes should not exceed maxMem")
}

func (s *boundedChanTestSuite) TestInputChannelClosed() {
	out, in, _ := NewBoundedChan(10, 1000, func(int) int64 { return 1 })

	// Send some items and close
	in <- 1
	in <- 2
	in <- 3
	close(in)

	// All items should be received
	items := []int{}
	for item := range out {
		items = append(items, item)
	}

	s.Assert().Equal([]int{1, 2, 3}, items)
}

func (s *boundedChanTestSuite) TestOversizedItem() {
	// A single item larger than maxMem should still pass through.
	// The memory limit is temporarily exceeded, then restored after drain.
	out, in, stats := NewBoundedChan(10, 50, func(i int) int64 { return int64(i) })

	go func() {
		in <- 200
		in <- 200
		in <- 200
		close(in)
	}()

	items := []int{}
	for item := range out {
		items = append(items, item)
	}

	s.Assert().Equal([]int{200, 200, 200}, items)

	snap := stats()
	s.Assert().Zero(snap.BufferedItems)
	s.Assert().Equal(int64(0), snap.BufferedBytes)
}

func (s *boundedChanTestSuite) TestLargeMemoryItems() {
	maxMem := int64(100)
	out, in, _ := NewBoundedChan(100, maxMem, func(b []byte) int64 { return int64(len(b)) })

	go func() {
		// Send a large item
		largeItem := make([]byte, 50)
		in <- largeItem

		// Send another that pushes over limit
		in <- make([]byte, 60)
		close(in)
	}()

	// First item should come out
	val := <-out
	s.Assert().Len(val, 50)

	// Get the second
	val2 := <-out
	s.Assert().Len(val2, 60)

	// Channel should close
	_, ok := <-out
	s.Assert().False(ok)
}

func (s *boundedChanTestSuite) TestManySmallItems() {
	out, in, _ := NewBoundedChan(5, 10000, func(int) int64 { return 1 })

	// Send 100 items through the channel
	go func() {
		for i := range 100 {
			in <- i
		}
		close(in)
	}()

	// Receive all items in order
	received := 0
	for item := range out {
		s.Assert().Equal(received, item)
		received++
	}

	s.Assert().Equal(100, received)
}

func (s *boundedChanTestSuite) TestEmptyBuffer() {
	out, in, _ := NewBoundedChan(10, 1000, func(int) int64 { return 1 })

	// Send and receive immediately
	in <- 42
	val := <-out
	s.Assert().Equal(42, val)

	// Close channel
	close(in)

	// Output should close
	_, ok := <-out
	s.Assert().False(ok)
}

func (s *boundedChanTestSuite) TestMemoryAndCountLimitsTogether() {
	// Limit to 3 items and 50 bytes
	out, in, _ := NewBoundedChan(3, 50, func(i int) int64 { return int64(i) })

	go func() {
		// Send items: 5, 10, 15 (total 30 bytes, 3 items)
		in <- 5
		in <- 10
		in <- 15

		// Send 20: total would be 50 (at limit)
		in <- 20

		// Should be drained now since count=4 > 3
		// Send 5 more
		in <- 5
		close(in)
	}()

	// Collect all items
	items := []int{}
	for item := range out {
		items = append(items, item)
	}

	// All 5 items should come through
	s.Assert().Len(items, 5)
}

func (s *boundedChanTestSuite) TestZeroItemEdgeCases() {
	out, in, _ := NewBoundedChan(10, 1000, func(int) int64 {
		// Return 0 for size
		return 0
	})

	// With zero-size items, count limit is enforced (not memory)
	// Send in goroutine while receiving in main
	go func() {
		for i := range 100 {
			in <- i
		}
		close(in)
	}()

	// Should receive all items
	received := 0
	for range out {
		received++
	}

	s.Assert().Equal(100, received)
}

func (s *boundedChanTestSuite) TestConcurrentStatsReads() {
	// Verify stats function is safe for concurrent reads while channel operates.
	// This tests atomic-safe access to both ringbuf.Len() and curMem.
	out, in, stats := NewBoundedChan(10, 1000, func(i int) int64 { return int64(i) })

	done := make(chan struct{})
	statsCounter := startStatsReaders(stats, done)
	produceItems(in)
	received := consumeItems(out)
	s.Assert().Equal(100, received)

	close(done)
	s.Assert().
		Positive(statsCounter.Load(), "stats reader should have executed and seen valid data")
}

// startStatsReaders launches multiple goroutines that concurrently read stats.
func startStatsReaders(stats func() BoundedChanStats, done chan struct{}) *atomic.Int64 {
	statsCounter := &atomic.Int64{}
	for range 5 {
		go statsReaderLoop(stats, done, statsCounter)
	}
	return statsCounter
}

// statsReaderLoop continuously reads stats until done is closed.
func statsReaderLoop(stats func() BoundedChanStats, done chan struct{}, counter *atomic.Int64) {
	for {
		select {
		case <-done:
			return
		default:
			snap := stats()
			if isValidSnapshot(snap) {
				counter.Add(1)
			}
			runtime.Gosched()
		}
	}
}

// isValidSnapshot verifies that a stats snapshot is within expected bounds.
func isValidSnapshot(snap BoundedChanStats) bool {
	return snap.BufferedItems >= 0 && snap.BufferedItems <= snap.MaxItems &&
		snap.BufferedBytes >= 0 && snap.BufferedBytes <= snap.MaxBytes
}

// produceItems sends 100 items to the channel then closes it.
func produceItems(in chan<- int) {
	go func() {
		for i := 1; i <= 100; i++ {
			in <- i
		}
		close(in)
	}()
}

// consumeItems receives all items from the channel and returns the count.
func consumeItems(out <-chan int) int {
	received := 0
	for range out {
		received++
	}
	return received
}

func (s *boundedChanTestSuite) TestStatsAccuracy() {
	// Verify stats reflect actual buffer state at snapshot time.
	out, in, stats := NewBoundedChan(5, 1000, func(i int) int64 { return int64(i) })

	// Send 3 items: size 1, 2, 3 = 6 total.
	// Because out is not being read yet, once these sends complete the worker
	// cannot drain buffered items any further, so the snapshot is deterministic.
	in <- 1
	in <- 2
	in <- 3

	snap := stats()
	s.Assert().Equal(3, snap.BufferedItems)
	s.Assert().Equal(int64(6), snap.BufferedBytes)
	s.Assert().Equal(5, snap.MaxItems)
	s.Assert().Equal(int64(1000), snap.MaxBytes)

	// Close and drain remaining
	close(in)
	for range out {
	}

	snap = stats()
	s.Assert().Zero(snap.BufferedItems)
	s.Assert().Equal(int64(0), snap.BufferedBytes)
}

func (s *boundedChanTestSuite) TestSizeComputedOncePerItem() {
	// Verify that size is computed once on receipt and reused on drain.
	// Count size() invocations directly so the test deterministically fails if
	// size() is called more than once per item.
	var sizeCalls atomic.Int64
	sizeFunc := func(i int) int64 {
		sizeCalls.Add(1)
		return int64(i + 1)
	}

	out, in, stats := NewBoundedChan(10, 1000, sizeFunc)

	// Send items
	go func() {
		for i := range 10 {
			in <- i
		}
		close(in)
	}()

	// Receive all items
	received := 0
	for range out {
		received++
	}

	s.Assert().Equal(10, received)
	s.Assert().EqualValues(10, sizeCalls.Load(), "size() should be called exactly once per item")

	// Verify final stats: no items buffered, no bytes buffered
	snap := stats()
	s.Assert().Equal(0, snap.BufferedItems, "all items should be drained")
	s.Assert().
		Equal(int64(0), snap.BufferedBytes, "all bytes should be accounted for (curMem should be 0)")
}

// pollStats polls until the given predicate is true, checking stats repeatedly.
func pollStats(
	t *testing.T,
	stats func() BoundedChanStats,
	predicate func(BoundedChanStats) bool,
) BoundedChanStats {
	var snap BoundedChanStats

	require.Eventually(
		t,
		func() bool {
			snap = stats()
			return predicate(snap)
		},
		time.Minute,
		time.Millisecond,
	)

	return snap
}

func (s *boundedChanTestSuite) TestCountBoundIsInclusive() {
	// Verify that maxCount limit is inclusive: we can buffer exactly maxCount items without draining.
	out, in, stats := NewBoundedChan(3, 1000, func(int) int64 { return 1 })

	sentDone := make(chan struct{})
	go func() {
		// Send exactly 3 items; they should all stay buffered (not drained)
		in <- 1
		in <- 2
		in <- 3
		// Signal that sending is complete; goroutine exits immediately after
		close(sentDone)
	}()

	// Poll until items are buffered or we know sending is done
	snap := pollStats(s.T(), stats, func(st BoundedChanStats) bool { return st.BufferedItems == 3 })
	s.Assert().Equal(3, snap.BufferedItems, "should be able to buffer exactly maxCount items")

	// Wait for sending to complete, then close input and drain
	<-sentDone
	close(in)
	for range out {
	}

	snap = stats()
	s.Assert().Zero(snap.BufferedItems)
}

func (s *boundedChanTestSuite) TestMemoryBelowLimitStaysBuffered() {
	// Items totaling strictly less than maxMem should all stay buffered.
	// 10 + 20 + 29 = 59 < maxMem = 60: no forced drain.
	out, in, stats := NewBoundedChan(100, 60, func(i int) int64 { return int64(i) })

	sentDone := make(chan struct{})
	go func() {
		in <- 10
		in <- 20
		in <- 29
		close(sentDone)
	}()

	snap := pollStats(s.T(), stats, func(st BoundedChanStats) bool { return st.BufferedItems == 3 })
	s.Assert().Equal(3, snap.BufferedItems, "should have 3 items")
	s.Assert().Equal(int64(59), snap.BufferedBytes, "items below maxMem should stay buffered")

	<-sentDone
	close(in)
	for range out {
	}

	snap = stats()
	s.Assert().Equal(int64(0), snap.BufferedBytes)
}
