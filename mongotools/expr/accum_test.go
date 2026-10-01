package expr

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestSum(t *testing.T) {
	assertExpr(t, Sum{"$count"}, bson.D{{"$sum", "$count"}})
}

func TestPush(t *testing.T) {
	assertExpr(t, Push{"$value"}, bson.D{{"$push", "$value"}})
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
