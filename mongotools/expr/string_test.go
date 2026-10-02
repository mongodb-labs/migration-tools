package expr

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestConcat(t *testing.T) {
	assertExpr(t, Concat{"$first", " ", "$last"}, bson.D{
		{"$concat", bson.A{"$first", " ", "$last"}},
	})
}

func TestSplit(t *testing.T) {
	assertExpr(t, Split{"$name", "-"}, bson.D{
		{"$split", bson.A{"$name", "-"}},
	})
}

func TestSubstrBytes(t *testing.T) {
	assertExpr(t, SubstrBytes{"$name", 0, 3}, bson.D{
		{"$substrBytes", bson.A{"$name", 0, 3}},
	})
}

func TestRegexMatch(t *testing.T) {
	// Without Options.
	assertExpr(t, RegexMatch{Input: "$name", Regex: "^pre"}, bson.D{
		{"$regexMatch", bson.D{
			{"input", "$name"},
			{"regex", "^pre"},
		}},
	})

	// With Options.
	assertExpr(t, RegexMatch{Input: "$name", Regex: "^pre", Options: "i"}, bson.D{
		{"$regexMatch", bson.D{
			{"input", "$name"},
			{"regex", "^pre"},
			{"options", "i"},
		}},
	})
}
