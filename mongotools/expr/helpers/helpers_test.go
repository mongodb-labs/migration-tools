package helpers

import (
	"testing"

	"github.com/mongodb-labs/migration-tools/mongotools/expr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func mustMarshal(t *testing.T, val any) []byte {
	t.Helper()

	bytes, err := bson.Marshal(val)
	require.NoError(t, err)
	return bytes
}

// assertExpr checks that expr’s D() and MarshalBSON() agree with each
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

func TestExists(t *testing.T) {
	assertExpr(t, Exists{"$myField"}, bson.D{
		{"$not", bson.D{
			{"$eq", bson.A{
				"missing",
				bson.D{{"$type", "$myField"}},
			}},
		}},
	})
}

func TestStringHasPrefix(t *testing.T) {
	assertExpr(t, StringHasPrefix{FieldRef: "$name", Prefix: "pre"}, bson.D{
		{"$eq", bson.A{
			0,
			bson.D{{"$indexOfCP", bson.A{"$name", "pre", 0, 1}}},
		}},
	})
}

func TestTypeIs(t *testing.T) {
	cases := []struct {
		label string
		ref   any
		types []expr.Type
		want  bson.D
	}{
		{
			label: "single type",
			ref:   "$myField",
			types: []expr.Type{expr.TypeLong},
			want: bson.D{
				{"$in", bson.A{
					bson.D{{"$type", "$myField"}},
					bson.A{expr.TypeLong},
				}},
			},
		},
		{
			label: "multiple types",
			ref:   "$myField",
			types: []expr.Type{expr.TypeLong, expr.TypeString, expr.TypeNull},
			want: bson.D{
				{"$in", bson.A{
					bson.D{{"$type", "$myField"}},
					bson.A{expr.TypeLong, expr.TypeString, expr.TypeNull},
				}},
			},
		},
		{
			label: "ref is an expression",
			ref:   expr.Concat{"$firstName", " ", "$lastName"},
			types: []expr.Type{expr.TypeString},
			want: bson.D{
				{"$in", bson.A{
					bson.D{{"$type", bson.D{
						{"$concat", bson.A{"$firstName", " ", "$lastName"}},
					}}},
					bson.A{expr.TypeString},
				}},
			},
		},
	}

	for _, curCase := range cases {
		t.Run(curCase.label, func(t *testing.T) {
			assert.Equal(
				t,
				mustMarshal(t, curCase.want),
				mustMarshal(t, TypeIs(curCase.ref, curCase.types...)),
			)
		})
	}
}
