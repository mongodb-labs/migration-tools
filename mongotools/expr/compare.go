package expr

import "go.mongodb.org/mongo-driver/v2/bson"

// Eq is the $eq operator.
type Eq [2]any

var _ bson.Marshaler = Eq{}

func (e Eq) D() bson.D {
	return bson.D{{"$eq", [2]any(e)}}
}

func (e Eq) MarshalBSON() ([]byte, error) {
	return bson.Marshal(e.D())
}

// ---------------------------------------------

// Gt is the $gt operator.
type Gt [2]any

var _ bson.Marshaler = Gt{}

func (g Gt) D() bson.D {
	return bson.D{{"$gt", [2]any(g)}}
}

func (g Gt) MarshalBSON() ([]byte, error) {
	return bson.Marshal(g.D())
}

// ---------------------------------------------

// In is the $in operator. The variadic haystack allows literal values
// (In("x", "a", "b", "c")) as well as spreading a Go slice
// (In("x", mySlice...)).
func In[T any](needle any, haystack ...T) bson.D {
	return bson.D{{"$in", bson.A{needle, haystack}}}
}

// ---------------------------------------------
