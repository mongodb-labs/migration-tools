package future

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFuture(t *testing.T) {
	future, setter := New[int]()

	assert.Panics(t, func() { future.Get() },
		"Get() should panic before the value is set",
	)

	select {
	case <-future.Ready():
		require.Fail(t, "should not be ready")
	case <-time.NewTimer(time.Millisecond).C:
	}

	setter(123)

	select {
	case <-future.Ready():
	case <-time.NewTimer(time.Millisecond).C:
		require.Fail(t, "should be ready")
	}

	assert.Equal(
		t,
		123,
		future.Get(),
		"Get() should return the value",
	)

	assert.Equal(
		t,
		123,
		future.Get(),
		"Get() should return the value a 2nd time",
	)
}

func TestFutureNil(t *testing.T) {
	future, setter := New[error]()

	select {
	case <-future.Ready():
		require.Fail(t, "should not be ready")
	case <-time.NewTimer(time.Millisecond).C:
	}

	setter(nil)

	select {
	case <-future.Ready():
	case <-time.NewTimer(time.Millisecond).C:
		require.Fail(t, "should be ready")
	}

	assert.Nil(t, future.Get())
}
