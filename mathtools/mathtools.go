package mathtools

import (
	"github.com/mongodb-labs/migration-tools/option"
	"github.com/samber/lo"
	"golang.org/x/exp/constraints"
)

// RealNumber is implemented by any real-number type.
type RealNumber interface {
	constraints.Integer | constraints.Float
}

// Mean returns the arithmetic mean of all numbers in the slice,
// or None if the slice is empty.
func Mean[T RealNumber](nums []T) option.Option[float64] {
	if len(nums) == 0 {
		return option.None[float64]()
	}

	mean := float64(lo.Sum(nums)) / float64(len(nums))
	return option.Some(mean)
}
