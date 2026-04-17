package synctools

import (
	"sync/atomic"

	"github.com/mongodb-labs/migration-tools/ringbuf"
	"github.com/samber/lo"
)

// bufferedItem pairs an item with its precomputed size.
// Size is computed once on receipt and reused on drain to ensure curMem invariants.
type bufferedItem[T any] struct {
	item T
	size int64
}

// BoundedChanStats holds snapshot statistics about a BoundedChan’s internal state.
type BoundedChanStats struct {
	BufferedItems int // Number of items currently buffered
	MaxItems      int // Maximum items allowed

	BufferedBytes int64 // Total bytes of buffered items
	MaxBytes      int64 // Maximum bytes allowed
}

// NewBoundedChan is like a plain channel but also lets you limit the amount of
// memory to which the channel members refer. For example, instead of a plain
// chan []byte, which limits only by len(chan), use NewBoundedChan to limit
// both by len *and* the buffers’ total size.
//
// This returns separate read-from & write-to channels. (Similar to io.Pipe(),
// but with channels). The size function computes a single item’s size.
//
// NOTE: The returned channels are unbuffered and WILL NOT reflect current
// usage. Use the returned stats function for lock-free stats queries.
//
// This panics if either maxCount or maxTotalSize is nonpositive.
func NewBoundedChan[T any](
	maxCount int,
	maxTotalSize int64,
	size func(T) int64,
) (<-chan T, chan<- T, func() BoundedChanStats) {
	lo.Assertf(
		maxCount > 0,
		"maxCount (%d) must be positive",
		maxCount,
	)

	lo.Assertf(
		maxTotalSize > 0,
		"maxTotalSize (%d) must be positive",
		maxTotalSize,
	)

	lo.Assertf(
		size != nil,
		"size must not be nil",
	)

	in := make(chan T)
	out := make(chan T)

	w := &boundedChanWorker[T]{
		in:       in,
		out:      out,
		buf:      ringbuf.New[bufferedItem[T]](maxCount),
		size:     size,
		maxCount: maxCount,
		maxMem:   maxTotalSize,
	}

	go w.run()

	statsFn := func() BoundedChanStats {
		return BoundedChanStats{
			BufferedItems: w.buf.Len(),
			BufferedBytes: w.curMem.Load(),
			MaxItems:      maxCount,
			MaxBytes:      maxTotalSize,
		}
	}

	return out, in, statsFn
}

type boundedChanWorker[T any] struct {
	in       <-chan T
	out      chan<- T
	buf      *ringbuf.RingBuf[bufferedItem[T]]
	curMem   atomic.Int64 // atomic for lock-free stats queries
	size     func(T) int64
	maxCount int
	maxMem   int64
}

func (w *boundedChanWorker[T]) itemSize(item T) int64 {
	size := w.size(item)

	lo.Assertf(
		size >= 0,
		"bounded channel item size (%d) must be nonnegative",
		size,
	)

	return size
}

// push computes size once and stores it with the item.
func (w *boundedChanWorker[T]) push(item T) {
	sz := w.itemSize(item)
	w.buf.Push(bufferedItem[T]{item: item, size: sz})
	w.curMem.Add(sz)
}

// pop removes the next item from the buffer and updates curMem using the precomputed size.
func (w *boundedChanWorker[T]) pop() {
	entry := w.buf.Peek()
	w.curMem.Add(-entry.size)
	w.buf.Pop()
}

func (w *boundedChanWorker[T]) run() {
	defer close(w.out)

	for {
		if !w.processItems() {
			return
		}

		if !w.drainExcess() {
			return
		}
	}
}

// Returns false to indicate that all work is done: the input channel
// is closed, and the buffer is drained.
func (w *boundedChanWorker[T]) processItems() bool {
	if w.buf.Len() == 0 {
		return w.receiveItem()
	}
	return w.receiveOrSend()
}

// Returns false to indicate that the input channel is closed.
func (w *boundedChanWorker[T]) receiveItem() bool {
	item, ok := <-w.in
	if !ok {
		return false
	}
	w.push(item)
	return true
}

// Returns false to indicate that all work is done: the input channel
// is closed, and the buffer is drained.
func (w *boundedChanWorker[T]) receiveOrSend() bool {
	// If at or over the count limit, must drain (send) before receiving more.
	// This keeps the buffer within capacity without needing extra space.
	if w.buf.Len() >= w.maxCount {
		return w.drainOne()
	}
	return w.receiveOrSendNormal()
}

// drainOne sends one buffered item and returns true.
func (w *boundedChanWorker[T]) drainOne() bool {
	entry := w.buf.Peek()
	w.out <- entry.item
	w.pop()
	return true
}

// receiveOrSendNormal handles the normal case when below the count limit.
func (w *boundedChanWorker[T]) receiveOrSendNormal() bool {
	select {
	case itemIn, ok := <-w.in:
		return w.handleReceive(itemIn, ok)
	case w.out <- w.buf.Peek().item:
		w.pop()
		return true
	}
}

// handleReceive processes a received item or input close.
func (w *boundedChanWorker[T]) handleReceive(item T, ok bool) bool {
	if !ok {
		w.flushRemaining()
		return false
	}
	w.push(item)
	return true
}

func (w *boundedChanWorker[T]) flushRemaining() {
	for w.buf.Len() > 0 {
		entry := w.buf.Peek()
		w.out <- entry.item
		w.pop()
	}
}

func (w *boundedChanWorker[T]) drainExcess() {
	for w.shouldDrain() {
		w.drainOne()
	}
}

// shouldDrain returns true if the buffer exceeds either limit.
func (w *boundedChanWorker[T]) shouldDrain() bool {
	return w.buf.Len() > 0 && (w.buf.Len() > w.maxCount || w.curMem.Load() > w.maxMem)
}
