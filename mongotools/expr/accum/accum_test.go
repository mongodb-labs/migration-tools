package accum

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// assertExpr checks that e’s D() and MarshalBSON() agree with each
// other and with want.
func assertExpr(t *testing.T, e interface {
	D() bson.D
	bson.Marshaler
}, want bson.D) {
	t.Helper()

	wantBytes, err := bson.Marshal(want)
	require.NoError(t, err)

	dBytes, err := bson.Marshal(e.D())
	require.NoError(t, err)
	assert.Equal(t, wantBytes, dBytes, "D() mismatch")

	marshaled, err := e.MarshalBSON()
	require.NoError(t, err)
	assert.Equal(t, wantBytes, marshaled, "MarshalBSON() mismatch")
}

func TestSum(t *testing.T) {
	assertExpr(t, Sum{"$count"}, bson.D{{"$sum", "$count"}})
}

func TestPush(t *testing.T) {
	assertExpr(t, Push{"$value"}, bson.D{{"$push", "$value"}})
}

func TestAddToSet(t *testing.T) {
	assertExpr(t, AddToSet{"$item"}, bson.D{{"$addToSet", "$item"}})
}

func TestMax(t *testing.T) {
	assertExpr(t, Max{"$ts"}, bson.D{{"$max", "$ts"}})
}

func TestFirst(t *testing.T) {
	assertExpr(t, First{"$ts"}, bson.D{{"$first", "$ts"}})
}

func TestFirstN(t *testing.T) {
	assertExpr(t, FirstN{N: 3, Input: "$array"}, bson.D{
		{"$firstN", bson.D{
			{"n", 3},
			{"input", "$array"},
		}},
	})
}

func TestSetUnion(t *testing.T) {
	assertExpr(t, SetUnion{"$items"}, bson.D{{"$setUnion", "$items"}})
}

func TestTopN(t *testing.T) {
	assertExpr(t, TopN{
		N:      3,
		SortBy: bson.D{{"score", -1}},
		Output: "$name",
	}, bson.D{
		{"$topN", bson.D{
			{"n", 3},
			{"sortBy", bson.D{{"score", -1}}},
			{"output", "$name"},
		}},
	})
}
