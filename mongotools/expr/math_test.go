package expr

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestAbs(t *testing.T) {
	assertExpr(t, Abs{"$delta"}, bson.D{{"$abs", "$delta"}})
}

func TestAdd(t *testing.T) {
	assertExpr(t, Add{"$a", "$b", 1}, bson.D{
		{"$add", bson.A{"$a", "$b", 1}},
	})
}

func TestMod(t *testing.T) {
	assertExpr(t, Mod{"$hours", "$tasks"}, bson.D{
		{"$mod", bson.A{"$hours", "$tasks"}},
	})
}

func TestSubtract(t *testing.T) {
	assertExpr(t, Subtract{"$end", "$start"}, bson.D{
		{"$subtract", bson.A{"$end", "$start"}},
	})
}
