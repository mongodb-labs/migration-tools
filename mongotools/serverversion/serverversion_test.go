package serverversion_test

import (
	"encoding/json"
	"testing"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/mongodb-labs/migration-tools/mongotools/serverversion"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These stand in for the well-known version constants that each product defines in its own repo,
// e.g. `var V60 = serverversion.New(6, 0)`.
var (
	v44    = serverversion.New(4, 4)
	v50    = serverversion.New(5, 0)
	v60    = serverversion.New(6, 0)
	v70    = serverversion.New(7, 0)
	v80    = serverversion.New(8, 0)
	v90    = serverversion.New(9, 0)
	v91DSC = serverversion.NewWithStorage(9, 1, serverversion.DisaggregatedStorage)
)

// testPairs is a small stand-in for a product's supported version pairs map.
var testPairs = serverversion.SupportedPairs{
	v50: mapset.NewSet(v60, v70),
	v60: mapset.NewSet(v60, v70, v80),
	v70: mapset.NewSet(v70, v80),
	v80: mapset.NewSet(v80, v91DSC),
}

type withPointer struct {
	Version *serverversion.ServerVersion `json:"version"`
}

type withoutPointer struct {
	Version serverversion.ServerVersion `json:"version"`
}

func TestMarshalText(t *testing.T) {
	for _, tc := range []struct {
		version serverversion.ServerVersion
		want    string
	}{
		{v44, "4.4"},
		{v91DSC, "9.1-dsc"},
	} {
		text, err := tc.version.MarshalText()
		require.NoError(t, err)
		assert.Equal(t, tc.want, string(text))
	}
}

func TestUnmarshalText(t *testing.T) {
	t.Run("non-pointer", func(t *testing.T) {
		var v struct {
			Version serverversion.ServerVersion `json:"version"`
		}
		err := json.Unmarshal([]byte(`{"version":"4.4"}`), &v)
		require.NoError(t, err)
		assert.Equal(t, v44, v.Version)
	})

	t.Run("pointer", func(t *testing.T) {
		var v struct {
			Version *serverversion.ServerVersion `json:"version"`
		}
		err := json.Unmarshal([]byte(`{"version":"9.1-dsc"}`), &v)
		require.NoError(t, err)
		assert.Equal(t, v91DSC, *v.Version)
	})

	t.Run("invalid", func(t *testing.T) {
		var v struct {
			Version *serverversion.ServerVersion `json:"version"`
		}
		err := json.Unmarshal([]byte(`{"version":"4"}`), &v)
		assert.Error(t, err)
	})
}

func TestMarshalJSON(t *testing.T) {
	t.Run("non-pointer", func(t *testing.T) {
		v := withoutPointer{v44}
		data, err := json.Marshal(v)
		require.NoError(t, err)
		assert.JSONEq(t, `{"version":"4.4"}`, string(data))
	})

	t.Run("pointer", func(t *testing.T) {
		v := withPointer{&v44}
		data, err := json.Marshal(v)
		require.NoError(t, err)
		assert.JSONEq(t, `{"version":"4.4"}`, string(data))
	})
}

func TestParseAnyServerVersion(t *testing.T) {
	for _, tc := range []struct {
		str  string
		want serverversion.ServerVersion
	}{
		{"6.0", v60},
		{"6.0.17", v60},
		{"9.1-dsc", v91DSC},
		{"9.1", serverversion.New(9, 1)},
	} {
		v, err := serverversion.Parse(tc.str)
		require.NoError(t, err, tc.str)
		assert.Equal(t, tc.want, v, tc.str)
	}

	for _, str := range []string{"4", "6", "9.1-dsc-dsc", "9.1-bogus", "x.y"} {
		_, err := serverversion.Parse(str)
		assert.Error(t, err, str)
	}
}

// Parsing and support-checking are separate concerns: parse with ParseAnyServerVersion, then
// check support with SupportedPairs.Contains.
func TestParseThenCheckSupport(t *testing.T) {
	supported, err := serverversion.Parse("6.0")
	require.NoError(t, err)
	assert.True(t, testPairs.Contains(supported))

	// A version can parse successfully but still be unsupported.
	unsupported, err := serverversion.Parse("4.4")
	require.NoError(t, err)
	assert.False(t, testPairs.Contains(unsupported))
}

func TestSupportedPairsVersionHelpers(t *testing.T) {
	assert.True(t, testPairs.Contains(v70))
	assert.False(t, testPairs.Contains(v44))

	assert.Equal(
		t,
		[]serverversion.ServerVersion{
			v50,
			v60,
			v70,
			v80,
			v91DSC,
		},
		testPairs.SupportedVersions(),
	)

	assert.Equal(t, "5.0 6.0 7.0 8.0 9.1-dsc", testPairs.SupportedVersionsString())

	// The latest source must itself be a key in the pairs map.
	assert.Contains(t, testPairs, testPairs.LatestSource())
	assert.Equal(t, v80, testPairs.LatestSource())
	assert.Equal(t, v91DSC, testPairs.Latest())

	// The latest pair must actually be a supported pair. Note that Latest() (v91DSC) is not
	// pairable with itself here — it only appears as a destination of v80.
	assert.Equal(
		t,
		serverversion.VersionPair{SrcVersion: v80, DstVersion: v91DSC},
		testPairs.LatestPair(),
	)
	assert.True(t, testPairs.PairsSet().Contains(testPairs.LatestPair()))
}

func TestSupportedPairsPairs(t *testing.T) {
	pairs := testPairs.Pairs()

	// The pairs must be sorted first by source version, then by destination version.
	for i := 1; i < len(pairs); i++ {
		assert.Negative(t, pairs[i-1].Compare(pairs[i]))
	}

	assert.Contains(
		t,
		pairs,
		serverversion.VersionPair{SrcVersion: v60, DstVersion: v70},
	)
	assert.Equal(t, 9, testPairs.PairsSet().Cardinality())
}

func TestServerVersionStrings(t *testing.T) {
	assert.Equal(t, "v91dsc", v91DSC.VString())
	assert.Equal(t, "v91", v91DSC.VStringNoStorage())
	assert.Equal(t, "9.1-dsc", v91DSC.String())
	assert.Equal(t, "9.1", v91DSC.StringNoStorage())
	assert.Equal(t, "91-dsc", v91DSC.StringNoDot())

	assert.Equal(t, "v50", v50.VString())
	assert.Equal(t, "5.0", v50.String())
	assert.Equal(t, "50", v50.StringNoDot())
}

func TestServerVersionCompare(t *testing.T) {
	assert.Negative(t, v50.Compare(v60))
	assert.Positive(t, v80.Compare(v70))

	// The LT/LTE/GT/GTE methods follow the same ordering as Compare: true when the comparison
	// of the receiver against the argument holds.
	assert.True(t, v80.GT(v70))
	assert.True(t, v80.GTE(v70))
	assert.True(t, v80.GTE(v80))
	assert.True(t, v80.LTE(v80))
	assert.False(t, v70.GTE(v80))
	assert.True(t, v70.LT(v80))
	assert.True(t, v70.LTE(v80))

	// Major/minor take precedence over storage type, and ASC sorts before DSC.
	assert.Negative(t, v90.Compare(v91DSC))
	assert.Negative(t, serverversion.New(9, 1).Compare(v91DSC))
	assert.Zero(
		t,
		serverversion.New(9, 1).
			Compare(serverversion.NewWithStorage(9, 1, serverversion.AttachedStorage)),
	)

	assert.True(
		t,
		v91DSC.Equals(serverversion.NewWithStorage(9, 1, serverversion.DisaggregatedStorage)),
	)
	// The comparison methods ignore storage type, so a 9.1 ASC cluster and a 9.1 DSC cluster
	// compare equal.
	assert.True(t, v91DSC.Equals(serverversion.New(9, 1)))
	assert.False(t, v91DSC.GT(serverversion.New(9, 1)))
	assert.True(t, v91DSC.GTE(serverversion.New(9, 1)))
	assert.True(t, serverversion.New(9, 1).GTE(v91DSC))
	assert.True(t, serverversion.New(9, 1).LTE(v91DSC))
	assert.True(t, v50.LT(v60))

	assert.True(t, v91DSC.IsDisagg())
	assert.Equal(t, serverversion.DisaggregatedStorage, v91DSC.StorageType())
	assert.Equal(t, 9, v91DSC.Major())
	assert.Equal(t, 1, v91DSC.Minor())
}

func TestFromSlice(t *testing.T) {
	assert.Equal(t, v60, serverversion.FromSlice([]int{6, 0, 17}, serverversion.AttachedStorage))
	assert.Equal(
		t,
		v91DSC,
		serverversion.FromSlice([]int{9, 1}, serverversion.DisaggregatedStorage),
	)
}
