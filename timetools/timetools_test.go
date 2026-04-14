package timetools

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestToDurationWithFloats tests the CORRECT behavior for fractional durations.
func TestToDurationWithFloats(t *testing.T) {
	tests := []struct {
		name     string
		count    float64
		unit     time.Duration
		expected time.Duration
	}{
		{
			name:     "1.5 * 1 second",
			count:    1.5,
			unit:     time.Second,
			expected: time.Duration(1.5 * float64(time.Second)),
		},
		{
			name:     "2.5 * 1 hour",
			count:    2.5,
			unit:     time.Hour,
			expected: time.Duration(2.5 * float64(time.Hour)),
		},
		{
			name:     "0.001 * 1 second = 1 millisecond",
			count:    0.001,
			unit:     time.Second,
			expected: time.Duration(0.001 * float64(time.Second)),
		},
		{
			name:     "0.5 * 1 microsecond",
			count:    0.5,
			unit:     time.Microsecond,
			expected: time.Duration(0.5 * float64(time.Microsecond)),
		},
		{
			name:     "1.23456 * 1 second",
			count:    1.23456,
			unit:     time.Second,
			expected: time.Duration(1.23456 * float64(time.Second)),
		},
		{
			name:     "negative: -2.5 * 1 second",
			count:    -2.5,
			unit:     time.Second,
			expected: time.Duration(-2.5 * float64(time.Second)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ToDuration(tt.count, tt.unit)
			require.NoError(t, err, "should not overflow")
			assert.Equal(t, tt.expected, result, "float duration conversion")
		})
	}
}

// TestToDurationWithIntegers tests integer inputs.
func TestToDurationWithIntegers(t *testing.T) {
	tests := []struct {
		name     string
		count    any
		unit     time.Duration
		expected time.Duration
	}{
		{
			name:     "int: 5 * 1s",
			count:    int(5),
			unit:     time.Second,
			expected: 5 * time.Second,
		},
		{
			name:     "int64: 10 * 100ms",
			count:    int64(10),
			unit:     100 * time.Millisecond,
			expected: 1 * time.Second,
		},
		{
			name:     "int: 0 * 1s",
			count:    int(0),
			unit:     time.Second,
			expected: 0,
		},
		{
			name:     "int32: 3 * 1h",
			count:    int32(3),
			unit:     time.Hour,
			expected: 3 * time.Hour,
		},
		{
			name:     "negative: -5 * 1s",
			count:    int(-5),
			unit:     time.Second,
			expected: -5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result time.Duration
			var err error
			switch count := tt.count.(type) {
			case int:
				result, err = ToDuration(count, tt.unit)
			case int64:
				result, err = ToDuration(count, tt.unit)
			case int32:
				result, err = ToDuration(count, tt.unit)
			}

			require.NoError(t, err, "should not overflow")
			assert.Equal(t, tt.expected, result, "integer duration conversion")
		})
	}
}

// TestToDurationEdgeCases tests boundary and unusual inputs.
func TestToDurationEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		count    any
		unit     time.Duration
		expected time.Duration
	}{
		{
			name:     "large integer: 1000000 * 1s",
			count:    int(1000000),
			unit:     time.Second,
			expected: 1000000 * time.Second,
		},
		{
			name:     "float that looks like int: 5.0 * 1s",
			count:    5.0,
			unit:     time.Second,
			expected: 5 * time.Second,
		},
		{
			name:     "small float: 0.25 * 1 second",
			count:    0.25,
			unit:     time.Second,
			expected: time.Duration(0.25 * float64(time.Second)),
		},
		{
			name:     "float with microseconds: 1.5 * 100 microseconds",
			count:    1.5,
			unit:     100 * time.Microsecond,
			expected: time.Duration(1.5 * float64(100*time.Microsecond)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch count := tt.count.(type) {
			case int:
				result, err := ToDuration(count, tt.unit)
				require.NoError(t, err, "should not overflow")
				assert.Equal(t, tt.expected, result)
			case float64:
				result, err := ToDuration(count, tt.unit)
				require.NoError(t, err, "should not overflow")
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestToDurationConsistency verifies int and float paths produce consistent results
// when given equivalent values.
func TestToDurationConsistency(t *testing.T) {
	t.Run("int(5) and float64(5.0) should produce same result", func(t *testing.T) {
		intResult, intErr := ToDuration(int(5), time.Second)
		floatResult, floatErr := ToDuration(5.0, time.Second)

		assert.NoError(t, intErr, "should not overflow")
		assert.NoError(t, floatErr, "should not overflow")
		assert.Equal(t, intResult, floatResult,
			"integer and float versions of same value should match")
		assert.Equal(t, 5*time.Second, intResult)
	})
}

// TestToDurationOverflow tests integer overflow detection.
func TestToDurationOverflow(t *testing.T) {
	tests := []struct {
		name  string
		count any
		unit  time.Duration
	}{
		{
			name:  "large positive overflow: MaxInt32 * Hours",
			count: int32(math.MaxInt32),
			unit:  time.Hour,
		},
		{
			name:  "large negative underflow: MinInt32 * Hours",
			count: int32(math.MinInt32),
			unit:  time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			switch count := tt.count.(type) {
			case int32:
				_, err = ToDuration(count, tt.unit)
			}

			assert.Error(t, err, "should detect overflow/underflow")
		})
	}
}

// TestToDurationMinMaxInt64 tests edge cases with MinInt64 and MaxInt64.
func TestToDurationMinMaxInt64(t *testing.T) {
	tests := []struct {
		name        string
		count       int64
		unit        time.Duration
		expectError bool
	}{
		{
			name:        "MaxInt64 * 1 nanosecond: should fit",
			count:       math.MaxInt64,
			unit:        time.Duration(1),
			expectError: false,
		},
		{
			name:        "MaxInt64 * 2 nanoseconds: overflow",
			count:       math.MaxInt64,
			unit:        time.Duration(2),
			expectError: true,
		},
		{
			name:        "MinInt64 * 1 nanosecond: should fit",
			count:       math.MinInt64,
			unit:        time.Duration(1),
			expectError: false,
		},
		{
			name:        "MinInt64 * 2 nanoseconds: underflow",
			count:       math.MinInt64,
			unit:        time.Duration(2),
			expectError: true,
		},
		{
			name:        "MinInt64 / 2 * 2: should fit (within safe bounds)",
			count:       math.MinInt64 / 2,
			unit:        time.Duration(2),
			expectError: false,
		},
		{
			name:        "MaxInt64 / 2 * 2: should fit",
			count:       math.MaxInt64 / 2,
			unit:        time.Duration(2),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ToDuration(tt.count, tt.unit)
			if tt.expectError {
				assert.Error(t, err, "should detect overflow/underflow")
			} else {
				assert.NoError(t, err, "should not overflow")
			}
		})
	}
}

// TestToDurationInvalidUnit tests invalid time units.
func TestToDurationInvalidUnit(t *testing.T) {
	tests := []struct {
		name  string
		count int
		unit  time.Duration
	}{
		{
			name:  "zero unit",
			count: 5,
			unit:  0,
		},
		{
			name:  "negative unit",
			count: 5,
			unit:  -time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ToDuration(tt.count, tt.unit)
			assert.Error(t, err, "should reject invalid unit")
		})
	}
}

// TestToDurationFloatInvalid tests that invalid float values are rejected.
func TestToDurationFloatInvalid(t *testing.T) {
	tests := []struct {
		name  string
		count float64
		unit  time.Duration
	}{
		{
			name:  "positive infinity",
			count: math.Inf(1),
			unit:  time.Hour,
		},
		{
			name:  "negative infinity",
			count: math.Inf(-1),
			unit:  time.Hour,
		},
		{
			name:  "NaN",
			count: math.NaN(),
			unit:  time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ToDuration(tt.count, tt.unit)
			assert.Error(t, err, "should reject infinity and NaN")
		})
	}
}

// TestToDurationFloatPrecision tests that non-integer floats use float math.
func TestToDurationFloatPrecision(t *testing.T) {
	t.Run("float with fractional nanoseconds", func(t *testing.T) {
		// A non-integer float (1.5) bypasses integer overflow checks
		// and uses the float multiplication path
		result, err := ToDuration(1.5, time.Second)
		require.NoError(t, err)
		assert.Equal(t, time.Duration(1.5*float64(time.Second)), result)
	})
}
