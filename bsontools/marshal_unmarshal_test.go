package bsontools

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestMarshalUnmarshal(t *testing.T) {
	docs := []bson.D{
		{},
		{{"foo", int32(12)}},
		{{"foo", int64(12)}},
		{{"", bson.A{
			bson.Symbol("123123"),
			bson.MinKey{},
			bson.MaxKey{},
			bson.JavaScript("heyhey"),
			"ohh yeah",
			bson.Undefined{},
			nil,
			bson.NewDateTimeFromTime(time.Now()),
			//time.Now(),
			bson.Binary{12, []byte{0, 1, 2}},
			bson.NewObjectID(),
			bson.Timestamp{234234, 345345},
			bson.Regex{"the pattern", "opts"},
		}}},
		{
			{"yeah", true},
			{"aa", bson.D{}},
			{"bb", bson.A{int32(12), 23.34, int64(234)}},
			{"dec", bson.NewDecimal128(234234, 345345)},
		},
	}

	raw := bson.Raw{}

	for _, someDoc := range docs {
		raw = raw[:0]

		t.Logf("cur doc: %+v", someDoc)

		raw, err := MarshalD(raw, someDoc)
		require.NoError(t, err)

		fromLib, err := bson.Marshal(someDoc)
		require.NoError(t, err)

		assert.Equal(t, bson.Raw(fromLib), raw, "output should be same")

		rt, err := UnmarshalRaw(fromLib)
		require.NoError(t, err)

		var rtFromLib bson.D
		err = bson.Unmarshal(fromLib, &rtFromLib)
		require.NoError(t, err)

		assert.Equal(t, rtFromLib, rt)
	}
}
