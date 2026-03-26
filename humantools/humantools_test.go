package humantools

import (
	"math"
	"testing"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/stretchr/testify/assert"
)

func TestDurationToDHMS(t *testing.T) {
	tests := []struct {
		millis uint
		dhms   string
	}{
		// sub-minute
		{0, "0s"},
		{500, "0.5s"},
		{1000, "1s"},
		{1234, "1.23s"},
		{1500, "1.5s"},
		{59000, "59s"},
		{59990, "59.99s"},
		// minutes
		{60000, "1m 0s"},
		{90500, "1m 30.5s"},
		{3599000, "59m 59s"},
		{3599990, "59m 59.99s"},
		// hours
		{86399000, "23h 59m 59s"},
		{86399500, "23h 59m 59.5s"},
		// days — hours, minutes, seconds always shown
		{86400000, "1d 0h 0m 0s"},
		{86400000 + 3661500, "1d 1h 1m 1.5s"},
		{7 * 86400000, "7d 0h 0m 0s"},
		{70 * 86400000, "70d 0h 0m 0s"},
		{70*86400000 + 3*3600000 + 22*60000 + 3230, "70d 3h 22m 3.23s"},
	}

	for _, tt := range tests {
		dur := time.Duration(tt.millis) * time.Millisecond
		dhms := DurationToDHMS(dur)
		assert.Equal(t, tt.dhms, dhms, "%dms -> %s", tt.millis, tt.dhms)
	}

	negativeTests := []struct {
		millis int
		dhms   string
	}{
		{-1000, "-1s"},
		{-1500, "-1.5s"},
		{-90500, "-1m 30.5s"},
		{-86399500, "-23h 59m 59.5s"},
		{-86400000, "-1d 0h 0m 0s"},
		{-(86400000 + 3661500), "-1d 1h 1m 1.5s"},
	}

	for _, tt := range negativeTests {
		dur := time.Duration(tt.millis) * time.Millisecond
		dhms := DurationToDHMS(dur)
		assert.Equal(t, tt.dhms, dhms, "%dms -> %s", tt.millis, tt.dhms)
	}
}

func TestFmtPercent(t *testing.T) {
	assert.Equal(
		t,
		"23.45",
		FmtPercent(uint(2_345_111), uint(10_000_000)),
		"numeric precision is as expected (uint)",
	)

	assert.Equal(
		t,
		"23.45",
		FmtPercent(int(2_345_111), uint(10_000_000)),
		"numeric precision is as expected (int / uint)",
	)

	assert.Equal(
		t,
		"23.45",
		FmtPercent(float64(2_345_111), uint(10_000_000)),
		"numeric precision is as expected (float64 / uint)",
	)

	bigNum := uint(99999999999999)
	assert.NotEqual(
		t,
		"100",
		FmtPercent(bigNum, 1+bigNum),
		"No false \"100 percent\" should happen",
	)

	assert.Equal(
		t,
		"100",
		FmtPercent(uint(10), uint(10)),
		"equal numerator and denominator is exactly 100",
	)

	assert.Equal(
		t,
		"0",
		FmtPercent(uint(0), uint(100)),
		"zero numerator",
	)

	assert.Equal(
		t,
		"110",
		FmtPercent(uint(11), uint(10)),
		"numerator exceeding denominator is over 100",
	)
}

func TestFmtReal(t *testing.T) {
	intTests := []struct {
		num    int64
		result string
	}{
		{0, "0"},
		{42, "42"},
		{1_234_567, "1,234,567"},
		{-1_234_567, "-1,234,567"},
		{math.MaxInt64, "9,223,372,036,854,775,807"},
	}

	for _, tt := range intTests {
		assert.Equal(t, tt.result, FmtReal(tt.num), "%d", tt.num)
	}

	floatTests := []struct {
		num    float64
		result string
	}{
		{0.0, "0"},
		{3.14159, "3.14"},
		{1234.567, "1,234.57"},
		{1.5, "1.5"},
		{1_234_567.89, "1,234,567.89"},
	}

	for _, tt := range floatTests {
		assert.Equal(t, tt.result, FmtReal(tt.num), "%f", tt.num)
	}

	// uint64 values exceeding MaxInt64 would overflow int64; verify they
	// are formatted as a positive number.
	big := uint64(math.MaxInt64) + 1
	assert.NotContains(
		t,
		FmtReal(big),
		"-",
		"large uint64 should not produce a negative result",
	)
}

func TestFmtBytes(t *testing.T) {
	tests := []struct {
		bytes uint64
		want  string
	}{
		{0, "0 bytes"},
		{1, "1 bytes"},
		{1023, "1,023 bytes"},
		{1024, "1 KiB"},
		{humanize.MiByte, "1 MiB"},
		{humanize.GiByte, "1 GiB"},
		{humanize.TiByte, "1 TiB"},
		{humanize.PiByte, "1 PiB"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, FmtBytes(tt.bytes), "%d bytes", tt.bytes)
	}
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
		output := BytesToUnit(tt.bytes, tt.unit)
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
		{1023, Bytes},
		{1024, KiB},
		{1204, KiB},
		{1234567, MiB},
		{humanize.GiByte, GiB},
		{humanize.TiByte, TiB},
		{humanize.PiByte, PiB},

		// go-humanize supports Exbibytes; we might as well ensure
		// good behavior if we somehow receive a byte total that big.
		{humanize.EiByte, PiB},
	}

	for _, tt := range tests {
		unit := FindBestUnit(tt.bytes)
		assert.Equal(t, tt.unit, unit, "%d should be %s", tt.bytes, tt.unit)
	}
}
