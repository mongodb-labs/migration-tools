// Package helpers exposes higher-level helpers that don’t map to a
// single aggregation operator, unlike the primitive operator wrappers
// in the expr package.
package helpers

import (
	"github.com/mongodb-labs/migration-tools/mongotools/expr"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Exists expresses “the referenced field exists”, regardless of its
// value, including null.
type Exists [1]any

var _ bson.Marshaler = Exists{}

func (e Exists) D() bson.D {
	return expr.Not{expr.Eq{"missing", expr.Type{e[0]}}}.D()
}

func (e Exists) MarshalBSON() ([]byte, error) {
	return bson.Marshal(e.D())
}

// ----------------------------

// TypeIs expresses whether ref’s BSON type matches any of the specified types.
func TypeIs(ref any, types ...expr.BSONType) bson.D {
	return expr.In(expr.Type{ref}, types...)
}

// ----------------------------

// StringHasPrefix parallels Go’s strings.HasPrefix.
type StringHasPrefix struct {
	FieldRef any
	Prefix   string
}

var _ bson.Marshaler = StringHasPrefix{}

func (sp StringHasPrefix) D() bson.D {
	return bson.D{
		{"$eq", bson.A{
			0,
			bson.D{{"$indexOfCP", bson.A{
				sp.FieldRef,
				sp.Prefix,
				0,
				1,
			}}},
		}},
	}
}

func (sp StringHasPrefix) MarshalBSON() ([]byte, error) {
	return bson.Marshal(sp.D())
}
