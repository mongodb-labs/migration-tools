package random

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUint64(t *testing.T) {
	t.Run("generates without error", func(t *testing.T) {
		val, err := Uint64()
		require.NoError(t, err)
		// We can't assert the value, but we can assert it exists
		assert.NotNil(t, val)
	})

	t.Run("values are highly unique", func(t *testing.T) {
		// Generate 100 random numbers and ensure they aren't all the same.
		// The odds of a collision in 100 draws of a uint64 are astronomically low.
		seen := make(map[uint64]struct{})
		for range 100 {
			val, err := Uint64()
			require.NoError(t, err)
			seen[val] = struct{}{}
		}

		assert.Greater(t, len(seen), 1, "expected to generate multiple unique random numbers")
	})
}

func TestFloat64(t *testing.T) {
	t.Run("strictly within bounds [0.0, 1.0)", func(t *testing.T) {
		// Run 1000 iterations to ensure edge cases or bit-shifts don't bleed out of bounds
		for range 1000 {
			val, err := Float64()
			require.NoError(t, err)

			assert.GreaterOrEqual(t, val, 0.0, "value should be >= 0.0")
			assert.Less(t, val, 1.0, "value should be < 1.0")
		}
	})

	t.Run("generates varying values", func(t *testing.T) {
		val1, err := Float64()
		require.NoError(t, err)

		val2, err := Float64()
		require.NoError(t, err)

		assert.NotEqual(t, val1, val2, "subsequent calls should generally produce different values")
	})
}

func TestUint64N(t *testing.T) {
	t.Run("error on zero", func(t *testing.T) {
		val, err := Uint64N(0)
		require.Error(t, err)
		assert.Equal(t, uint64(0), val)
		assert.Contains(t, err.Error(), "must be > 0")
	})

	t.Run("power of two boundary checks", func(t *testing.T) {
		// 16 is a power of 2, so it hits your optimized mask path: n & (n - 1) == 0
		var n uint64 = 16

		for range 1000 {
			val, err := Uint64N(n)
			require.NoError(t, err)
			assert.Less(t, val, n, "value should be strictly less than n")
		}
	})

	t.Run("non-power of two boundary checks", func(t *testing.T) {
		// 100 is not a power of 2, so it forces the rejection sampling loop path
		var n uint64 = 100

		for range 1000 {
			val, err := Uint64N(n)
			require.NoError(t, err)
			assert.Less(t, val, n, "value should be strictly less than n")
		}
	})

	t.Run("exact maximum uint64 bound", func(t *testing.T) {
		// Test the absolute maximum boundary of a uint64 (which is not a power of 2)
		// We just want to ensure it doesn't panic or endlessly loop here.
		_, err := Uint64N(math.MaxUint64)
		require.NoError(t, err)
	})
}
