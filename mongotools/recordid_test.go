package mongotools

import (
	"slices"
	"testing"

	"github.com/mongodb-labs/migration-tools/bsontools"
	"github.com/samber/lo"
	"github.com/samber/lo/mutable"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestCompareRecordIDs(t *testing.T) {
	lesserGreaterPairs := [][2]bson.RawValue{
		{
			bsontools.ToRawValue("foo"),
			bsontools.ToRawValue("zz"),
		},
		{
			// binary strings are sorted in binary order, unlike in
			// BSON sorting where they’re sorted by length, then subtype,
			// and only thereafter in binary order.
			bsontools.ToRawValue(bson.Binary{Data: []byte("foo")}),
			bsontools.ToRawValue(bson.Binary{Data: []byte("zz")}),
		},
		{
			bsontools.ToRawValue(int64(999)),
			bsontools.ToRawValue(int64(1000)),
		},
	}

	for _, pair := range lesserGreaterPairs {
		shuffled := slices.Clone(pair[:])
		mutable.Shuffle(shuffled)

		slices.SortFunc(
			shuffled,
			func(a, b bson.RawValue) int {
				return lo.Must(CompareRecordIDs(a, b))
			},
		)

		assert.True(
			t,
			pair[0].Equal(shuffled[0]) && pair[1].Equal(shuffled[1]),
			"%v should sort before %v",
			pair[0],
			pair[1],
		)
	}
}
