package mathtools

import (
	"testing"

	"github.com/mongodb-labs/migration-tools/option"
	"github.com/stretchr/testify/assert"
)

func TestMean(t *testing.T) {
	t.Run("empty slice returns None", func(t *testing.T) {
		assert.Equal(t, option.None[float64](), Mean([]int{}))
	})

	t.Run("single integer", func(t *testing.T) {
		assert.Equal(t, option.Some(7.0), Mean([]int{7}))
	})

	t.Run("multiple integers", func(t *testing.T) {
		assert.Equal(t, option.Some(2.0), Mean([]int{1, 2, 3}))
	})

	t.Run("non-integer result", func(t *testing.T) {
		assert.Equal(t, option.Some(1.5), Mean([]int{1, 2}))
	})

	t.Run("negative numbers", func(t *testing.T) {
		assert.Equal(t, option.Some(-2.0), Mean([]int{-1, -2, -3}))
	})

	t.Run("mixed positive and negative", func(t *testing.T) {
		assert.Equal(t, option.Some(0.0), Mean([]int{-3, -1, 1, 3}))
	})

	t.Run("float64 slice", func(t *testing.T) {
		assert.Equal(t, option.Some(2.5), Mean([]float64{1.5, 2.0, 4.0}))
	})

	t.Run("int64 slice", func(t *testing.T) {
		assert.Equal(t, option.Some(5.0), Mean([]int64{3, 5, 7}))
	})

	t.Run("large int64 does not overflow", func(t *testing.T) {
		const big int64 = 9_223_372_036_854_775_000
		assert.Equal(t, option.Some(float64(big)), Mean([]int64{big, big}))
	})
}
