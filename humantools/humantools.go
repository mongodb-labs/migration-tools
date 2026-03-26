// Package humantools exports various functions that are useful in “humanizing”
// numbers in various contexts. In large part it derives from & extends
// github.com/dustin/go-humanize; see that package for additional functionality.
package humantools

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/ccoveille/go-safecast/v2"
	"github.com/dustin/go-humanize"
	"github.com/samber/lo"
	"golang.org/x/exp/constraints"
)

// DataUnit signifies some unit of data.
type DataUnit string

const (
	Bytes DataUnit = "bytes"
	KiB   DataUnit = "KiB"
	MiB   DataUnit = "MiB"
	GiB   DataUnit = "GiB"
	TiB   DataUnit = "TiB"
	PiB   DataUnit = "PiB"

	// Anything larger than the above seems like overkill.
)

var unitSize = map[DataUnit]uint64{
	KiB: humanize.KiByte,
	MiB: humanize.MiByte,
	GiB: humanize.GiByte,
	TiB: humanize.TiByte,
	PiB: humanize.PiByte,
}

type realNum interface {
	constraints.Float | constraints.Integer
}

// num16Plus is like realNum, but it excludes 8-bit int/uint.
type num16Plus interface {
	constraints.Float |
		~uint | ~uint16 | ~uint32 | ~uint64 |
		~int | ~int16 | ~int32 | ~int64
}

// FmtReal provides a standard formatting of real numbers, with trailing decimal
// zeros removed.
//
// It may be useful to define an application-wide precision & wrap this
// function with that.
func FmtReal[T realNum](num T, precision uint) string {
	switch any(num).(type) {
	case float32, float64:
		return fmtFloat(num, precision)
	case uint64, uint:
		// Uints that can’t be int64 need to be formatted as floats.
		if uint64(num) > math.MaxInt64 {
			return fmtFloat(num, precision)
		}

		// Any other uint* type can be an int, which we format below.
	default:
		// Formatted below.
	}

	return humanize.Comma(int64(num))
}

// DurationToDHMS stringifies `duration` as, e.g., "1h 22m 3.23s".
// It’s a lot like Duration.String(), but with spaces between,
// the lowest unit shown is always the second, and this rounds to
// the nearest hundredth of a second. For durations >= 24h, days are
// shown instead of unbounded hours, e.g., "2d 3h 22m 3.23s"; in that
// case hours, minutes, and seconds are always included even if zero.
// Negative durations are formatted with a leading "-".
func DurationToDHMS(duration time.Duration) string {
	if duration < 0 {
		return "-" + DurationToDHMS(-duration)
	}

	days := int(math.Floor(duration.Hours() / 24))
	hours := int(math.Floor(duration.Hours())) % 24
	minutes := int(math.Floor(duration.Minutes())) % 60
	secs := math.Mod(duration.Seconds(), 60)

	str := FmtReal(secs, 2) + "s"

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %s", days, hours, minutes, str)
	}

	if hours > 0 {
		str = fmt.Sprintf("%dh %dm %s", hours, minutes, str)
	} else if minutes > 0 {
		str = fmt.Sprintf("%dm %s", minutes, str)
	}

	return str
}

// FmtBytes is a convenience that combines BytesToUnit with FindBestUnit.
// Use it to format a single count of bytes.
func FmtBytes[T num16Plus](count T, precision uint) string {
	unit := FindBestUnit(count)
	return BytesToUnit(count, unit, precision) + " " + string(unit)
}

// FindBestUnit gives the “best” DataUnit for the given `count` of bytes.
//
// You can then give that DataUnit to BytesToUnit() to format
// multiple byte counts to the same unit.
func FindBestUnit[T num16Plus](count T) DataUnit {
	// humanize.IBytes() does most of what we want but lacks the
	// flexibility to specify a precision. It’s not complicated to
	// implement here anyway.

	if count < T(humanize.KiByte) {
		return Bytes
	}

	// If the log2 is, e.g., 32.05, we want to use 2^30, i.e., GiB.
	log2 := math.Log2(float64(count))

	// Convert log2 to the next-lowest multiple of 10.
	unitNum := 10 * uint64(math.Floor(log2/10))

	// Now find that power of 2, which we can compare against
	// the values of the unitSize map (above).
	unitNum = 1 << unitNum

	// Just in case, someday, exbibytes become relevant …
	var biggestSize uint64
	var biggestUnit DataUnit

	for unit, size := range unitSize {
		if size == unitNum {
			return unit
		}

		if size > biggestSize {
			biggestSize = size
			biggestUnit = unit
		}
	}

	return biggestUnit
}

// BytesToUnit returns a stringified number that represents `count`
// in the given `unit`. For example, count=1024 and unit=KiB would
// return “1”.
func BytesToUnit[T num16Plus](count T, unit DataUnit, precision uint) string {
	// Ideally go-humanize could do this for us,
	// but as of this writing it can’t.
	// https://github.com/dustin/go-humanize/issues/111

	// We could put Bytes into the unitSize map above and handle it
	// the same as other units, but that would entail int/float
	// conversion and possibly risk little rounding errors. We might
	// as well keep it simple where we can.

	if unit == Bytes {
		return FmtReal(count, precision)
	}

	myUnitSize, exists := unitSize[unit]

	if !exists {
		panic(fmt.Sprintf("Missing unit in unitSize: %s", unit))
	}

	return FmtReal(float64(count)/float64(myUnitSize), precision)
}

// FmtPercent returns a stringified percentage without a trailing `%`,
// formatted as per FmtFloat(). FmtPercent also ensures that any
// percentage less than 100% is reported as something less; e.g.,
// 99.999997 doesn’t get rounded up to 100.
func FmtPercent[T, U realNum](numerator T, denominator U, precision uint) string {
	lo.Assert(denominator != 0, "denominator must be nonzero")

	ratio := 100 * float64(numerator) / float64(denominator)

	if ratio < 100 {
		// Round, but clamp so we never return exactly “100”.
		rounded := roundFloat(ratio, precision)
		if rounded >= 100 {
			return "99." + strings.Repeat("9", safecast.MustConvert[int](precision))
		}
		return FmtReal(rounded, precision)
	}

	return FmtReal(ratio, precision)
}

func fmtFloat[T realNum](num T, precision uint) string {
	return humanize.Commaf(roundFloat(float64(num), precision))
}

func roundFloat(val float64, precision uint) float64 {
	ratio := math.Pow10(safecast.MustConvert[int](precision))
	return math.Round(val*ratio) / ratio
}
