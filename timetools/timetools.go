package timetools

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/ccoveille/go-safecast/v2"
	"golang.org/x/exp/constraints"
)

type timeNumber interface {
	constraints.Integer | constraints.Float
}

const (
	minDuration = time.Duration(math.MinInt64)
	maxDuration = time.Duration(math.MaxInt64)
)

// ToDuration multiplies the given count & duration, with proper handling
// of numeric types. Returns an error on overflow.
func ToDuration[T timeNumber](count T, unit time.Duration) (time.Duration, error) {
	countAsDuration, err := safecast.Convert[time.Duration](count)
	if err != nil {
		return 0, fmt.Errorf("%v cannot be %T: %w", count, time.Duration(0), err)
	}

	if unit <= 0 {
		return 0, fmt.Errorf("invalid time unit (%s): must be positive", unit)
	}

	// If the count is an integer value, then just convert to a Duration.
	// NB: This is what we want even if the Go type is a float.
	if T(countAsDuration) == count {
		return intToDuration(countAsDuration, unit)
	}

	// The count is a float, which takes a different workflow.
	result := float64(count) * float64(unit)

	// Check for NaN or Inf—these can't be meaningfully converted to a duration
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return 0, errors.New("overflow: duration multiplication produces infinity or NaN")
	}

	// Let float precision handle the rest; Go's type conversion will handle values
	// at the boundaries naturally
	return time.Duration(result), nil
}

func intToDuration(count, unit time.Duration) (time.Duration, error) {
	if count >= 0 {
		// The normal case: both positive

		maxCount := maxDuration / unit
		if count > maxCount {
			return 0, fmt.Errorf("integer overflow: %d * %s", count, unit)
		}
	} else {
		// Less normal: count negative, unit positive

		minCount := minDuration / unit

		if count < minCount {
			return 0, fmt.Errorf("integer underflow: %d * %s", count, unit)
		}
	}

	return unit * count, nil
}
