package expr

import "go.mongodb.org/mongo-driver/v2/bson"

// Cond is the $cond operator.
type Cond struct {
	If, Then, Else any
}

var _ bson.Marshaler = Cond{}

func (c Cond) D() bson.D {
	return bson.D{
		{"$cond", bson.D{
			{"if", c.If},
			{"then", c.Then},
			{"else", c.Else},
		}},
	}
}

func (c Cond) MarshalBSON() ([]byte, error) {
	return bson.Marshal(c.D())
}

// ---------------------------------------------

// Switch is the $switch operator.
type Switch struct {
	Branches []SwitchCase
	Default  any
}

var _ bson.Marshaler = Switch{}

// SwitchCase is one branch of a $switch expression.
type SwitchCase struct {
	Case any
	Then any
}

func (s Switch) D() bson.D {
	return bson.D{{"$switch", bson.D{
		{"branches", s.Branches},
		{"default", s.Default},
	}}}
}

func (s Switch) MarshalBSON() ([]byte, error) {
	return bson.Marshal(s.D())
}
