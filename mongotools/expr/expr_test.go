package expr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// dAndMarshaler is implemented by every expression type in this
// package.
type dAndMarshaler interface {
	D() bson.D
	bson.Marshaler
}

// assertExpr checks that expr’s D() and MarshalBSON() agree with each
// other and with want.
func assertExpr(t *testing.T, expr dAndMarshaler, want bson.D) {
	t.Helper()

	wantBytes, err := bson.Marshal(want)
	require.NoError(t, err)

	dBytes, err := bson.Marshal(expr.D())
	require.NoError(t, err)
	assert.Equal(t, wantBytes, dBytes, "D() mismatch")

	marshaled, err := expr.MarshalBSON()
	require.NoError(t, err)
	assert.Equal(t, wantBytes, marshaled, "MarshalBSON() mismatch")
}

func mustMarshal(t *testing.T, val any) []byte {
	t.Helper()

	bytes, err := bson.Marshal(val)
	require.NoError(t, err)
	return bytes
}

func TestEq(t *testing.T) {
	assertExpr(t, Eq{"age", 21}, bson.D{
		{"$eq", bson.A{"age", 21}},
	})
}

func TestGt(t *testing.T) {
	assertExpr(t, Gt{"count", 0}, bson.D{
		{"$gt", bson.A{"count", 0}},
	})
}

func TestNe(t *testing.T) {
	assertExpr(t, Ne{"status", "inactive"}, bson.D{
		{"$ne", bson.A{"status", "inactive"}},
	})
}

func TestLt(t *testing.T) {
	assertExpr(t, Lt{"count", 10}, bson.D{
		{"$lt", bson.A{"count", 10}},
	})
}

func TestLte(t *testing.T) {
	assertExpr(t, Lte{"count", 10}, bson.D{
		{"$lte", bson.A{"count", 10}},
	})
}

func TestGte(t *testing.T) {
	assertExpr(t, Gte{"count", 10}, bson.D{
		{"$gte", bson.A{"count", 10}},
	})
}

func TestIn(t *testing.T) {
	// With literal values.
	assert.Equal(
		t,
		mustMarshal(t, bson.D{{"$in", bson.A{"status", bson.A{"a", "b"}}}}),
		mustMarshal(t, In("status", "a", "b")),
	)

	// With a spread Go slice.
	haystack := []string{"a", "b"}
	assert.Equal(
		t,
		mustMarshal(t, bson.D{{"$in", bson.A{"status", haystack}}}),
		mustMarshal(t, In("status", haystack...)),
	)
}

func TestBSONSize(t *testing.T) {
	assertExpr(t, BSONSize{"$ROOT"}, bson.D{
		{"$bsonSize", "$ROOT"},
	})
}

func TestLiteral(t *testing.T) {
	assertExpr(t, Literal{bson.D{{"$gte", 1}}}, bson.D{
		{"$literal", bson.D{{"$gte", 1}}},
	})
}

func TestType(t *testing.T) {
	assertExpr(t, Type{"$value"}, bson.D{
		{"$type", "$value"},
	})
}

// TestBSONTypeValues checks the BSONType constants against the
// canonical values from the $type operator’s documentation.
func TestBSONTypeValues(t *testing.T) {
	cases := map[BSONType]string{
		BSONTypeDouble:              "double",
		BSONTypeString:              "string",
		BSONTypeObject:              "object",
		BSONTypeArray:               "array",
		BSONTypeBinData:             "binData",
		BSONTypeUndefined:           "undefined",
		BSONTypeObjectID:            "objectId",
		BSONTypeBool:                "bool",
		BSONTypeDate:                "date",
		BSONTypeNull:                "null",
		BSONTypeRegex:               "regex",
		BSONTypeDBPointer:           "dbPointer",
		BSONTypeJavaScript:          "javascript",
		BSONTypeSymbol:              "symbol",
		BSONTypeJavaScriptWithScope: "javascriptWithScope",
		BSONTypeInt:                 "int",
		BSONTypeTimestamp:           "timestamp",
		BSONTypeLong:                "long",
		BSONTypeDecimal:             "decimal",
		BSONTypeMinKey:              "minKey",
		BSONTypeMaxKey:              "maxKey",
		BSONTypeNumber:              "number",
	}

	for curType, curValue := range cases {
		assert.Equal(t, BSONType(curValue), curType)
	}
}

func TestNot(t *testing.T) {
	assertExpr(t, Not{Type{"$value"}}, bson.D{
		{"$not", bson.D{{"$type", "$value"}}},
	})
}

func TestAnd(t *testing.T) {
	assertExpr(t, And{Type{"$a"}, Type{"$b"}}, bson.D{
		{"$and", bson.A{bson.D{{"$type", "$a"}}, bson.D{{"$type", "$b"}}}},
	})
}

func TestOr(t *testing.T) {
	assertExpr(t, Or{Type{"$a"}, Type{"$b"}}, bson.D{
		{"$or", bson.A{bson.D{{"$type", "$a"}}, bson.D{{"$type", "$b"}}}},
	})
}

func TestMergeObjects(t *testing.T) {
	assertExpr(t, MergeObjects{"$a", "$b"}, bson.D{
		{"$mergeObjects", bson.A{"$a", "$b"}},
	})
}

func TestGetField(t *testing.T) {
	// With Input.
	assertExpr(t, GetField{Input: "$doc", Field: "myField"}, bson.D{
		{"$getField", bson.D{
			{"input", "$doc"},
			{"field", "myField"},
		}},
	})

	// Input is omitted when nil.
	assertExpr(t, GetField{Field: "myField"}, bson.D{
		{"$getField", bson.D{
			{"field", "myField"},
		}},
	})
}

func TestLet(t *testing.T) {
	assertExpr(t, Let{Vars: bson.D{{"x", 1}}, In: "$$x"}, bson.D{
		{"$let", bson.D{
			{"vars", bson.D{{"x", 1}}},
			{"in", "$$x"},
		}},
	})
}

func TestReduce(t *testing.T) {
	assertExpr(t, Reduce{
		Input:        "$array",
		InitialValue: 0,
		In:           "$$value",
	}, bson.D{
		{"$reduce", bson.D{
			{"input", "$array"},
			{"initialValue", 0},
			{"in", "$$value"},
		}},
	})
}

func TestCond(t *testing.T) {
	assertExpr(t, Cond{If: Eq{"$a", 1}, Then: "yes", Else: "no"}, bson.D{
		{"$cond", bson.D{
			{"if", bson.D{{"$eq", bson.A{"$a", 1}}}},
			{"then", "yes"},
			{"else", "no"},
		}},
	})
}

func TestIfNull(t *testing.T) {
	// Single input expression.
	assertExpr(t, IfNull{"$rated", "Not Rated"}, bson.D{
		{"$ifNull", bson.A{"$rated", "Not Rated"}},
	})

	// Multiple input expressions.
	assertExpr(t, IfNull{"$critic", "$viewer", 0}, bson.D{
		{"$ifNull", bson.A{"$critic", "$viewer", 0}},
	})
}

func TestNor(t *testing.T) {
	assertExpr(t, Nor{Eq{"$a", 1}, Eq{"$b", 2}}, bson.D{
		{"$nor", bson.A{
			bson.D{{"$eq", bson.A{"$a", 1}}},
			bson.D{{"$eq", bson.A{"$b", 2}}},
		}},
	})
}

func TestSwitch(t *testing.T) {
	branches := bson.A{
		bson.D{{"case", bson.D{{"$eq", bson.A{"$a", 1}}}}, {"then", "one"}},
		bson.D{{"case", bson.D{{"$eq", bson.A{"$a", 2}}}}, {"then", "two"}},
	}

	// With Default.
	assertExpr(t, Switch{
		Branches: []SwitchCase{
			{Case: Eq{"$a", 1}, Then: "one"},
			{Case: Eq{"$a", 2}, Then: "two"},
		},
		Default: "other",
	}, bson.D{
		{"$switch", bson.D{
			{"branches", branches},
			{"default", "other"},
		}},
	})

	// Default is omitted when nil.
	assertExpr(t, Switch{
		Branches: []SwitchCase{
			{Case: Eq{"$a", 1}, Then: "one"},
			{Case: Eq{"$a", 2}, Then: "two"},
		},
	}, bson.D{
		{"$switch", bson.D{
			{"branches", branches},
		}},
	})
}

func TestMap(t *testing.T) {
	// With As.
	assertExpr(t, Map{Input: "$array", As: "elem", In: "$$elem"}, bson.D{
		{"$map", bson.D{
			{"input", "$array"},
			{"as", "elem"},
			{"in", "$$elem"},
		}},
	})

	// As is omitted when nil.
	assertExpr(t, Map{Input: "$array", In: "$$this"}, bson.D{
		{"$map", bson.D{
			{"input", "$array"},
			{"in", "$$this"},
		}},
	})
}

func TestFilter(t *testing.T) {
	cond := Eq{"$$elem", 1}

	// With As and Limit.
	assertExpr(t, Filter{
		Input: "$array",
		As:    "elem",
		Cond:  cond,
		Limit: 3,
	}, bson.D{
		{"$filter", bson.D{
			{"input", "$array"},
			{"cond", bson.D{{"$eq", bson.A{"$$elem", 1}}}},
			{"as", "elem"},
			{"limit", 3},
		}},
	})

	// As and Limit are omitted when nil.
	assertExpr(t, Filter{Input: "$array", Cond: cond}, bson.D{
		{"$filter", bson.D{
			{"input", "$array"},
			{"cond", bson.D{{"$eq", bson.A{"$$elem", 1}}}},
		}},
	})
}
