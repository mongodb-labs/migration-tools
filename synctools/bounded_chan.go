package synctools

import (
	"github.com/mongodb-labs/migration-tools/ringbuf"
	"github.com/samber/lo"
)

// NewBoundedChan creates channels with memory-bounded semantics: it enforces
// limits both of count and aggregate size. Returns a send channel and receive
// channel, similar to io.Pipe(). The size function computes a single item’s size.
//
// This panics if either maxCount or maxMem is nonpositive.
func NewBoundedChan[T any](
	maxCount int64,
	maxMem int64,
	size func(T) int64,
) (chan<- T, <-chan T) {
	lo.Assertf(
		maxCount > 0,
		"maxCount (%d) must be positive",
		maxCount,
	)

	lo.Assertf(
		maxMem > 0,
		"maxMem (%d) must be positive",
		maxMem,
	)

	in := make(chan T)
	out := make(chan T)

	w := &boundedChanWorker[T]{
		in:       in,
		out:      out,
		buf:      ringbuf.New[T](int(maxCount)),
		size:     size,
		maxCount: maxCount,
		maxMem:   maxMem,
	}

	go w.run()

	return in, out
}

type boundedChanWorker[T any] struct {
	in       <-chan T
	out      chan<- T
	buf      *ringbuf.RingBuf[T]
	curMem   int64
	size     func(T) int64
	maxCount int64
	maxMem   int64
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

func (w *boundedChanWorker[T]) processItems() bool {
	if w.buf.Len() == 0 {
		return w.receiveItem()
	}
	return w.receiveOrSend()
}

func (w *boundedChanWorker[T]) receiveItem() bool {
	item, ok := <-w.in
	if !ok {
		return false
	}
	w.buf.Push(item)
	w.curMem += w.size(item)
	return true
}

func (w *boundedChanWorker[T]) receiveOrSend() bool {
	select {
	case item, ok := <-w.in:
		if !ok {
			w.flushRemaining()
			return false
		}
		w.buf.Push(item)
		w.curMem += w.size(item)
		return true
	case w.out <- w.buf.Peek():
		w.curMem -= w.size(w.buf.Peek())
		w.buf.Pop()
		return true
	}
}

func (w *boundedChanWorker[T]) flushRemaining() {
	for w.buf.Len() > 0 {
		item := w.buf.Peek()
		w.out <- item
		w.curMem -= w.size(item)
		w.buf.Pop()
	}
}

func (w *boundedChanWorker[T]) drainExcess() bool {
	for w.buf.Len() > 0 && (int64(w.buf.Len()) >= w.maxCount || w.curMem >= w.maxMem) {
		item := w.buf.Peek()
		w.out <- item
		w.curMem -= w.size(item)
		w.buf.Pop()
	}
	return true
}
