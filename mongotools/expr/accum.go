// Expressions for accumulation operators, i.e., the operators usable
// only within a $group’s or $bucket’s fields or similar contexts.

package expr

import "go.mongodb.org/mongo-driver/v2/bson"

// Sum is the $sum accumulator.
type Sum [1]any

var _ bson.Marshaler = Sum{}

func (s Sum) D() bson.D {
	return bson.D{{"$sum", s[0]}}
}

func (s Sum) MarshalBSON() ([]byte, error) {
	return bson.Marshal(s.D())
}

//----------------------------------------------------------------------

// Push is the $push accumulator.
type Push [1]any

var _ bson.Marshaler = Push{}

func (p Push) D() bson.D {
	return bson.D{{"$push", p[0]}}
}

func (p Push) MarshalBSON() ([]byte, error) {
	return bson.Marshal(p.D())
}

//----------------------------------------------------------------------

// Max is the $max accumulator.
type Max [1]any

var _ bson.Marshaler = Max{}

func (m Max) D() bson.D {
	return bson.D{{"$max", m[0]}}
}

func (m Max) MarshalBSON() ([]byte, error) {
	return bson.Marshal(m.D())
}

//----------------------------------------------------------------------

// First is the $first accumulator.
type First [1]any

var _ bson.Marshaler = First{}

func (f First) D() bson.D {
	return bson.D{{"$first", f[0]}}
}

func (f First) MarshalBSON() ([]byte, error) {
	return bson.Marshal(f.D())
}

// ----------------------------------------------------------------------

// FirstN is the $firstN accumulator.
type FirstN struct {
	N     any
	Input any
}

var _ bson.Marshaler = FirstN{}

func (t FirstN) D() bson.D {
	return bson.D{
		{"$firstN", bson.D{
			{"n", t.N},
			{"input", t.Input},
		}},
	}
}

func (t FirstN) MarshalBSON() ([]byte, error) {
	return bson.Marshal(t.D())
}

//----------------------------------------------------------------------

// TopN is the $topN accumulator.
type TopN struct {
	N      any
	SortBy bson.D
	Output any
}

var _ bson.Marshaler = TopN{}

func (t TopN) D() bson.D {
	return bson.D{
		{"$topN", bson.D{
			{"n", t.N},
			{"sortBy", t.SortBy},
			{"output", t.Output},
		}},
	}
}

func (t TopN) MarshalBSON() ([]byte, error) {
	return bson.Marshal(t.D())
}
