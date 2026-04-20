package synctools

import (
	"sync/atomic"
)

// AtomicMax stores a concurrently-tracked max value.
// Multiple goroutines can safely write & read from this struct.
type AtomicMax[T any] struct {
	ptr      atomic.Pointer[T]
	comparer func(a, b T) int
}

// NewAtomicMax initializes the tracker.
// The comparer function works the same way as cmp.Compare.
func NewAtomicMax[T any](
	initial T,
	comparer func(a, b T) int,
) *AtomicMax[T] {
	am := &AtomicMax[T]{
		comparer: comparer,
	}
	val := initial
	am.ptr.Store(&val)
	return am
}

// Update concurrently checks and sets the new max if it is greater.
func (a *AtomicMax[T]) Update(val T) {
	for {
		currPtr := a.ptr.Load()

		// Dereference currPtr to get the actual value, then call comparer.
		if currPtr != nil && a.comparer(*currPtr, val) >= 0 {
			return
		}

		// Allocate a new unique memory address for the CAS operation
		newVal := val
		if a.ptr.CompareAndSwap(currPtr, &newVal) {
			return
		}
	}
}

// Get returns the current maximum value.
func (a *AtomicMax[T]) Get() T {
	currPtr := a.ptr.Load()
	if currPtr == nil {
		var zero T
		return zero
	}
	return *currPtr
}
