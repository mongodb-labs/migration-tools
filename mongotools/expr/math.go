package expr

import "go.mongodb.org/mongo-driver/v2/bson"

// Subtract is the $subtract operator.
type Subtract [2]any

var _ bson.Marshaler = Subtract{}

func (s Subtract) D() bson.D {
	return bson.D{{"$subtract", [2]any(s)}}
}

func (s Subtract) MarshalBSON() ([]byte, error) {
	return bson.Marshal(s.D())
}
