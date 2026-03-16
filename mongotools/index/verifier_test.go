package index

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/mongodb-labs/migration-tools/option"
	"github.com/mongodb-labs/migration-tools/testtools"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wI2L/jsondiff"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var verifierCompareCases = []struct {
	label        string
	src          bson.D
	dst          bson.D
	expectedDiff option.Option[SpecDiff]
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
		expectedDiff: option.Some(SpecDiff{
			JSONPatch: jsondiff.Patch{
				{
					Value:    "-1",
					OldValue: "1",
					Type:     "replace",
					Path:     "/key/foo/$numberInt",
				},
			},
		}),
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
		expectedDiff: option.Some(SpecDiff{
			JSONPatch: jsondiff.Patch{
				{
					OldValue: "1.1",
					Type:     "remove",
					Path:     "/key/foo/$numberDouble",
				},
				{
					Value: "1",
					Type:  "add",
					Path:  "/key/foo/$numberInt",
				},
			},
		}),
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
		expectedDiff: option.Some(SpecDiff{
			FieldOrderDiffers: []string{"key"},
		}),
	},
}

// TestVerifierCompareIndexSpecs contains tests adapted from
// Migration Verifier’s historical index comparison logic.
func TestVerifierCompareIndexSpecs(t *testing.T) {
	for _, curCase := range verifierCompareCases {
		diffOpt, err := DescribeSpecDifferences(
			lo.Must(bson.Marshal(curCase.src)),
			lo.Must(bson.Marshal(curCase.dst)),
		)
		require.NoError(t, err)

		if curCase.expectedDiff.IsNone() {
			assert.Zero(t, diffOpt, "specs should match")
		} else {
			if assert.NotZero(t, diffOpt, "specs should mismatch") {
				got, err := testtools.CloneExported(diffOpt.MustGet())
				require.NoError(t, err)

				assert.Empty(
					t,
					cmp.Diff(
						curCase.expectedDiff.MustGet(),
						got,
						cmpopts.IgnoreUnexported(jsondiff.Operation{}),
						cmpopts.IgnoreUnexported(SpecDiff{}),
					),
					"should have expected mismatch",
				)
			}
		}
	}
}
