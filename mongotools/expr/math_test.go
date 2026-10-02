package expr

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestSubtract(t *testing.T) {
	assertExpr(t, Subtract{"$end", "$start"}, bson.D{
		{"$subtract", bson.A{"$end", "$start"}},
	})
}
