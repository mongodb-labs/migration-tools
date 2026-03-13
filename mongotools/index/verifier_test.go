package index

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// TestVerifierCompareIndexSpecs contains tests adapted from
// Migration Verifier’s historical index comparison logic.
func TestVerifierCompareIndexSpecs(t *testing.T) {
	cases := []struct {
		label       string
		src         bson.D
		dst         bson.D
		shouldMatch bool
	}{
		{
			label: "simple",
			src: bson.D{
				{"v", 2},
				{"name", "testIndex"},
				{"key", bson.M{"foo": 123}},
			},
			dst: bson.D{
				{"v", 2},
				{"name", "testIndex"},
				{"key", bson.M{"foo": 123}},
			},
			shouldMatch: true,
		},

		{
			label: "ignore `ns` field",
			src: bson.D{
				{"v", 2},
				{"name", "testIndex"},
				{"key", bson.M{"foo": 123}},
				{"ns", "foo.bar"},
			},
			dst: bson.D{
				{"v", 2},
				{"name", "testIndex"},
				{"key", bson.M{"foo": 123}},
			},
			shouldMatch: true,
		},

		{
			label: "ignore number types",
			src: bson.D{
				{"v", 2},
				{"name", "testIndex"},
				{"key", bson.M{"foo": 123}},
			},
			dst: bson.D{
				{"v", 2},
				{"name", "testIndex"},
				{"key", bson.M{"foo": float64(123)}},
			},
			shouldMatch: true,
		},

		{
			label: "ignore number types in key",
			src: bson.D{
				{"v", 2},
				{"name", "testIndex"},
				{"key", bson.D{
					{"foo.bar", float64(123)},
					{"baz", int32(-2)},
				}},
			},
			dst: bson.D{
				{"v", 2},
				{"name", "testIndex"},
				{"key", bson.D{
					{"foo.bar", int32(123)},
					{"baz", int64(-2)},
				}},
			},
			shouldMatch: true,
		},

		{
			label: "find number differences",
			src: bson.D{
				{"v", 2},
				{"name", "testIndex"},
				{"key", bson.M{"foo": 1}},
			},
			dst: bson.D{
				{"v", 2},
				{"name", "testIndex"},
				{"key", bson.M{"foo": -1}},
			},
			shouldMatch: false,
		},

		{
			label: "find number differences with float",
			src: bson.D{
				{"v", 2},
				{"name", "testIndex"},
				{"key", bson.M{"foo": 1.1}},
			},
			dst: bson.D{
				{"v", 2},
				{"name", "testIndex"},
				{"key", bson.M{"foo": 1}},
			},
			shouldMatch: false,
		},

		{
			label: "key order differences",
			src: bson.D{
				{"v", 2},
				{"name", "testIndex"},
				{"key", bson.D{{"foo", 1}, {"bar", 1}}},
			},
			dst: bson.D{
				{"v", 2},
				{"name", "testIndex"},
				{"key", bson.D{{"bar", 1}, {"foo", 1}}},
			},
			shouldMatch: false,
		},
	}

	for _, curCase := range cases {
		diffOpt, err := DescribeSpecDifferences(
			lo.Must(bson.Marshal(curCase.src)),
			lo.Must(bson.Marshal(curCase.dst)),
		)
		require.NoError(t, err)

		if curCase.shouldMatch {
			assert.Zero(t, diffOpt, curCase.label)
		} else {
			assert.NotZero(t, diffOpt, curCase.label)
		}
	}
}
