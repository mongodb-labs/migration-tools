package humantools

import (
	"math"
	"testing"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/stretchr/testify/assert"
)

const precision = 2

func TestDurationToHMS(t *testing.T) {
	secTests := []struct {
		secs uint
		hms  string
	}{
		{1, "1s"},
		{59, "59s"},
		{60, "1m 0s"},
		{3599, "59m 59s"},
		{86399, "23h 59m 59s"},
		{86400, "24h 0m 0s"},
		{7 * 86400, "168h 0m 0s"},
		{70 * 86400, "1680h 0m 0s"},
	}

	for _, tt := range secTests {
		hms := DurationToHMS(time.Duration(tt.secs) * time.Second)
		assert.Equal(t, tt.hms, hms, "%.02f secs -> “%s”", tt.secs, tt.hms)
	}

	durationTests := []struct {
		dur time.Duration
		hms string
	}{
		{0, "0s"},
		{time.Duration(500) * time.Millisecond, "0.5s"},
		{time.Duration(1234) * time.Millisecond, "1.23s"},
	}

	for _, tt := range durationTests {
		hms := DurationToHMS(tt.dur)
		assert.Equal(t, tt.hms, hms, "%s -> “%s”", tt.dur, tt.hms)
	}
}

func TestFmtPercent(t *testing.T) {
	assert.Equal(
		t,
		"23.45",
		FmtPercent(uint(2_345_111), uint(10_000_000), precision),
		"numeric precision is as expected (uint)",
	)

	assert.Equal(
		t,
		"23.45",
		FmtPercent(int(2_345_111), uint(10_000_000), precision),
		"numeric precision is as expected (int / uint)",
	)

	assert.Equal(
		t,
		"23.45",
		FmtPercent(float64(2_345_111), uint(10_000_000), precision),
		"numeric precision is as expected (float64 / uint)",
	)

	bigNum := uint(99999999999999)
	assert.NotEqual(
		t,
		"100",
		FmtPercent(bigNum, 1+bigNum, precision),
		"No false “100 percent” should happen",
	)
}

func TestBytesToUnit(t *testing.T) {
	tests := []struct {
		bytes  uint64
		unit   DataUnit
		output string
	}{
		{1, Bytes, "1"},
		{2, Bytes, "2"},
		{1024, Bytes, "1,024"},
		{1024, KiB, "1"},
		{1124, KiB, "1.1"},
		{1124000, KiB, "1,097.66"},
		{math.MaxInt64, Bytes, "9,223,372,036,854,775,807"},
	}

	for _, tt := range tests {
		output := BytesToUnit(tt.bytes, tt.unit, precision)
		assert.Equal(
			t,
			tt.output, output,
			"%d bytes as %s", tt.bytes, tt.unit,
		)
	}
}

func TestByteConversion(t *testing.T) {
	tests := []struct {
		bytes uint64
		unit  DataUnit
	}{
		{0, Bytes},
		{1, Bytes},
		{1234567, MiB},
		{1204, KiB},

		// go-humanize supports Exbibytes; we might as well ensure
		// good behavior if we somehow receive a byte total that big.
		{humanize.EiByte, PiB},
	}

	for _, tt := range tests {
		unit := FindBestUnit(tt.bytes)
		assert.Equal(t, tt.unit, unit, "%d should be %s", tt.bytes, tt.unit)
	}
}
