package ringbuf

// RingBuf is a generic, fixed-capacity, array-backed ring buffer.
// It is not safe for concurrent use.
type RingBuf[T any] struct {
	buf   []T
	head  int
	tail  int
	count int
}

// New allocates a RingBuf with the given fixed capacity.
// This is the only allocation; subsequent operations are zero-copy.
func New[T any](capacity int) *RingBuf[T] {
	return &RingBuf[T]{buf: make([]T, capacity)}
}

// Len returns the number of items currently in the buffer.
func (r *RingBuf[T]) Len() int {
	return r.count
}

// Cap returns the maximum capacity of the buffer.
func (r *RingBuf[T]) Cap() int {
	return len(r.buf)
}

// Push adds an item to the back of the buffer.
// Panics if the buffer is full.
func (r *RingBuf[T]) Push(item T) {
	if r.count >= len(r.buf) {
		panic("ringbuf: push on full buffer")
	}
	r.buf[r.tail] = item
	r.tail = (r.tail + 1) % len(r.buf)
	r.count++
}

// Peek returns the oldest item in the buffer without removing it.
// Panics if the buffer is empty.
func (r *RingBuf[T]) Peek() T {
	if r.count == 0 {
		panic("ringbuf: peek on empty buffer")
	}
	return r.buf[r.head]
}

// Pop removes the oldest item from the buffer.
// The removed slot is zeroed to allow GC to collect the item.
// Panics if the buffer is empty.
func (r *RingBuf[T]) Pop() {
	if r.count == 0 {
		panic("ringbuf: pop on empty buffer")
	}
	var zero T
	r.buf[r.head] = zero // release reference for GC
	r.head = (r.head + 1) % len(r.buf)
	r.count--
}
