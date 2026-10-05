package expr

import "go.mongodb.org/mongo-driver/v2/bson"

// Concat is the $concat operator.
type Concat []any

var _ bson.Marshaler = Concat{}

func (c Concat) D() bson.D {
	return bson.D{{"$concat", bson.A(c)}}
}

func (c Concat) MarshalBSON() ([]byte, error) {
	return bson.Marshal(c.D())
}

// ----------------------------

// Split is the $split operator.
type Split [2]any

var _ bson.Marshaler = Split{}

func (s Split) D() bson.D {
	return bson.D{{"$split", s[:]}}
}

func (s Split) MarshalBSON() ([]byte, error) {
	return bson.Marshal(s.D())
}

// ----------------------------

// SubstrBytes is the $substrBytes operator.
type SubstrBytes [3]any

var _ bson.Marshaler = SubstrBytes{}

func (s SubstrBytes) D() bson.D {
	return bson.D{{"$substrBytes", s[:]}}
}

func (s SubstrBytes) MarshalBSON() ([]byte, error) {
	return bson.Marshal(s.D())
}
