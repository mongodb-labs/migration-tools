package expr

import (
	"slices"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Slice is the $slice operator. Position is optional; since it is
// positional in $slice’s argument list, it is only sent when non-nil.
type Slice struct {
	Array    any
	Position *any
	N        any
}

var _ bson.Marshaler = Slice{}

func (s Slice) D() bson.D {
	args := []any{s.Array, s.N}
	if s.Position != nil {
		args = slices.Insert(args, 1, *s.Position)
	}

	return bson.D{{"$slice", args}}
}

func (s Slice) MarshalBSON() ([]byte, error) {
	return bson.Marshal(s.D())
}

// ----------------------------

// ArrayElemAt is the $arrayElemAt operator.
type ArrayElemAt struct {
	Array any
	Index any
}

var _ bson.Marshaler = ArrayElemAt{}

func (a ArrayElemAt) D() bson.D {
	return bson.D{{"$arrayElemAt", bson.A{
		a.Array,
		a.Index,
	}}}
}

func (a ArrayElemAt) MarshalBSON() ([]byte, error) {
	return bson.Marshal(a.D())
}

// ---------------------------------------------

// Reduce is the $reduce operator.
type Reduce struct {
	Input, InitialValue, In any
}

var _ bson.Marshaler = Reduce{}

func (r Reduce) D() bson.D {
	return bson.D{
		{"$reduce", bson.D{
			{"input", r.Input},
			{"initialValue", r.InitialValue},
			{"in", r.In},
		}},
	}
}

func (r Reduce) MarshalBSON() ([]byte, error) {
	return bson.Marshal(r.D())
}

// ---------------------------------------------

// Map is the $map operator.
type Map struct {
	Input, As, In any
}

var _ bson.Marshaler = Map{}

func (m Map) D() bson.D {
	return bson.D{
		{"$map", bson.D{
			{"input", m.Input},
			{"as", m.As},
			{"in", m.In},
		}},
	}
}

func (m Map) MarshalBSON() ([]byte, error) {
	return bson.Marshal(m.D())
}

// ---------------------------------------------

// Filter is the $filter operator. As and Limit are optional; they are
// omitted from the expression when nil.
type Filter struct {
	Input, As, Cond, Limit any
}

var _ bson.Marshaler = Filter{}

func (f Filter) D() bson.D {
	d := bson.D{
		{"input", f.Input},
		{"cond", f.Cond},
	}

	if f.As != nil {
		d = append(d, bson.E{"as", f.As})
	}

	if f.Limit != nil {
		d = append(d, bson.E{"limit", f.Limit})
	}
	return bson.D{{"$filter", d}}
}

func (f Filter) MarshalBSON() ([]byte, error) {
	return bson.Marshal(f.D())
}
