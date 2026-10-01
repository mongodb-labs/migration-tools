// Package expr provides convenience types and functions for building
// aggregation expressions.
//
// This yields two major advantages over using [bson.D] or [bson.M]
// directly:
//   - simpler syntax
//   - auto-completion (i.e., via gopls)
//
// Guiding principles are:
//   - Prefer [1]any for unary operators (e.g., $bsonSize).
//   - Prefer [2]any for binary operators whose arguments don’t benefit
//     from naming (e.g., $eq).
//   - Prefer struct types for operators with named parameters AND for
//     operators whose documentation gives names, even if those names aren’t
//     sent to the server.
//   - Struct field names should match the server’s.
//   - Use functions sparingly, e.g., for “tuple” operators like $in.
//   - Use Go type `any` for arbitrary expressions.
//
// Every expression type has a D() method that returns its [bson.D]
// representation and implements [bson.Marshaler] through that method.
//
// This library doesn’t cover all expressions for now. Expand as is convenient.
package expr

import "go.mongodb.org/mongo-driver/v2/bson"

// BSONSize is the $bsonSize operator.
type BSONSize [1]any

var _ bson.Marshaler = BSONSize{}

func (b BSONSize) D() bson.D {
	return bson.D{{"$bsonSize", b[0]}}
}

func (b BSONSize) MarshalBSON() ([]byte, error) {
	return bson.Marshal(b.D())
}

// ---------------------------------------------

// TypeOf is the $type operator.
//
// NB: helpers.TypeIs() is often more convenient.
type TypeOf [1]any

var _ bson.Marshaler = TypeOf{}

func (t TypeOf) D() bson.D {
	return bson.D{{"$type", t[0]}}
}

func (t TypeOf) MarshalBSON() ([]byte, error) {
	return bson.Marshal(t.D())
}

// ---------------------------------------------

// Not is the $not operator.
type Not [1]any

var _ bson.Marshaler = Not{}

func (n Not) D() bson.D {
	return bson.D{{"$not", n[0]}}
}

func (n Not) MarshalBSON() ([]byte, error) {
	return bson.Marshal(n.D())
}

// ---------------------------------------------

// And is the $and operator.
type And []any

var _ bson.Marshaler = And{}

func (a And) D() bson.D {
	return bson.D{{"$and", []any(a)}}
}

func (a And) MarshalBSON() ([]byte, error) {
	return bson.Marshal(a.D())
}

// ---------------------------------------------

// Or is the $or operator.
type Or []any

var _ bson.Marshaler = Or{}

func (o Or) D() bson.D {
	return bson.D{{"$or", []any(o)}}
}

func (o Or) MarshalBSON() ([]byte, error) {
	return bson.Marshal(o.D())
}

// ---------------------------------------------

// MergeObjects is the $mergeObjects operator.
type MergeObjects []any

var _ bson.Marshaler = MergeObjects{}

func (m MergeObjects) D() bson.D {
	return bson.D{{"$mergeObjects", []any(m)}}
}

func (m MergeObjects) MarshalBSON() ([]byte, error) {
	return bson.Marshal(m.D())
}

// ---------------------------------------------

// GetField is the $getField operator.
type GetField struct {
	Input, Field any
}

var _ bson.Marshaler = GetField{}

func (gf GetField) D() bson.D {
	return bson.D{
		{"$getField", bson.D{
			{"input", gf.Input},
			{"field", gf.Field},
		}},
	}
}

func (gf GetField) MarshalBSON() ([]byte, error) {
	return bson.Marshal(gf.D())
}

// ---------------------------------------------

// Let is the $let operator.
type Let struct {
	Vars, In any
}

var _ bson.Marshaler = Let{}

func (l Let) D() bson.D {
	return bson.D{
		{"$let", bson.D{
			{"vars", l.Vars},
			{"in", l.In},
		}},
	}
}

func (l Let) MarshalBSON() ([]byte, error) {
	return bson.Marshal(l.D())
}
