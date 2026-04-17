package synctools

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type boundedChanTestSuite struct {
	suite.Suite
}

func TestBoundedChanTestSuite(t *testing.T) {
	suite.Run(t, &boundedChanTestSuite{})
}

func (s *boundedChanTestSuite) TestBasicSendReceive() {
	out, in, _ := NewBoundedChan(10, 1000, func(i int) int64 { return 1 })

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
	out, in, _ := NewBoundedChan(maxCount, 10000, func(i int) int64 { return 1 })

	// Send maxCount + 1 items in a goroutine to avoid deadlock
	go func() {
		for i := range maxCount + 1 {
			in <- int(i)
		}
		close(in)
	}()

	// When buffer hits limit and we send another, the first should be drained
	// Collect all outputs to verify they come in order
	items := []int{}
	for item := range out {
		items = append(items, item)
	}

	s.Assert().Len(items, int(maxCount)+1)
	// Verify items came in order
	for i := range items {
		s.Assert().Equal(i, items[i])
	}
}

func (s *boundedChanTestSuite) TestMemoryLimitEnforced() {
	maxMem := int64(100)
	out, in, _ := NewBoundedChan(1000, maxMem, func(i int) int64 { return int64(i) })

	// Send items in goroutine to avoid deadlock
	go func() {
		// The first four items total 85, which is under maxMem, so they should
		// all be buffered. The fifth item pushes the total to 105, exceeding
		// maxMem (10, 20, 30, 25, 20 = 105).
		itemsToSend := []int{10, 20, 30, 25, 20}
		for _, item := range itemsToSend {
			in <- item
		}
		close(in)
	}()

	// Collect items and verify first one comes out
	items := []int{}
	for item := range out {
		items = append(items, item)
	}

	// All items should come through in order
	s.Assert().Len(items, 5)
	s.Assert().Equal(10, items[0])
}

func (s *boundedChanTestSuite) TestInputChannelClosed() {
	out, in, _ := NewBoundedChan(10, 1000, func(i int) int64 { return 1 })

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
	out, in, _ := NewBoundedChan(5, 10000, func(i int) int64 { return 1 })

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
	out, in, _ := NewBoundedChan(10, 1000, func(i int) int64 { return 1 })

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
	out, in, _ := NewBoundedChan(10, 1000, func(i int) int64 {
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

	// Start goroutines constantly reading stats
	done := make(chan struct{})
	var statsCounter atomic.Int64
	for range 5 {
		go func() {
			for {
				select {
				case <-done:
					return
				default:
					snap := stats()
					// Just verify we can read safely; values will be stale snapshots
					if snap.BufferedItems >= 0 && snap.BufferedItems <= snap.MaxItems &&
						snap.BufferedBytes >= 0 && snap.BufferedBytes <= snap.MaxBytes {
						statsCounter.Add(1)
					}
					runtime.Gosched()
				}
			}
		}()
	}

	// Producer: send items
	go func() {
		for i := 1; i <= 100; i++ {
			in <- i
		}
		close(in)
	}()

	// Consumer: receive items
	received := 0
	for range out {
		received++
	}

	s.Assert().Equal(100, received)
	close(done)

	// Verify stats readers actually ran and saw valid snapshots
	s.Assert().
		Positive(statsCounter.Load(), "stats reader should have executed and seen valid data")
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
	// This test would fail if size() were called multiple times and returned different values.
	// We use a size function that returns a random int64 to simulate a mutable-state scenario.
	var rng uint64 = 12345 // seed
	sizeFunc := func(_ int) int64 {
		// Linear congruential generator for pseudo-random sizes
		rng = rng*1103515245 + 12345
		return int64((rng / 65536) % 100) // 0-99
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

	// Verify final stats: no items buffered, no bytes buffered
	snap := stats()
	s.Assert().Equal(0, snap.BufferedItems, "all items should be drained")
	s.Assert().
		Equal(int64(0), snap.BufferedBytes, "all bytes should be accounted for (curMem should be 0)")
}

func (s *boundedChanTestSuite) TestBoundsAreInclusive() {
	// Verify that limits are inclusive: we can buffer up to (and including) maxCount items
	// and up to (and including) maxMem bytes without automatic draining.

	// Test count limit inclusivity: maxCount=3 should allow exactly 3 items buffered
	out, in, stats := NewBoundedChan(3, 1000, func(i int) int64 { return 1 })

	go func() {
		// Send exactly 3 items; they should all stay buffered (not drained)
		in <- 1
		in <- 2
		in <- 3
		// Don't close yet; keep the channel open to prevent flushing
	}()

	// Give worker time to buffer items but don't read from out yet
	time.Sleep(50 * time.Millisecond)

	snap := stats()
	s.Assert().Equal(3, snap.BufferedItems, "should be able to buffer exactly maxCount items")

	// Now close and drain
	close(in)
	for range out {
	}

	snap = stats()
	s.Assert().Zero(snap.BufferedItems)

	// Test memory limit inclusivity: maxMem=60 should allow exactly 60 bytes buffered
	out2, in2, stats2 := NewBoundedChan(100, 60, func(i int) int64 { return int64(i) })

	go func() {
		// Send items: 10, 20, 30 = 60 bytes total; should stay buffered
		in2 <- 10
		in2 <- 20
		in2 <- 30
		// Don't close; keep open to prevent flushing
	}()

	// Give worker time to buffer
	time.Sleep(50 * time.Millisecond)

	snap2 := stats2()
	s.Assert().Equal(3, snap2.BufferedItems, "should have 3 items")
	s.Assert().
		Equal(int64(60), snap2.BufferedBytes, "should be able to buffer exactly maxMem bytes")

	// Now close and drain
	close(in2)
	for range out2 {
	}

	snap2 = stats2()
	s.Assert().Equal(int64(0), snap2.BufferedBytes)
}
