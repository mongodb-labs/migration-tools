package changestream

import (
	"fmt"

	"github.com/mongodb-labs/migration-tools/bsontools"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	// Type identifier for a resume token’s timestamp
	// (cf. https://github.com/mongodb-js/mongodb-resumetoken-decoder/blob/2d64962d194a5b99bb28ad1da6e7f1e26f6db0b7/src/keystringdecoder.ts#L20)
	rtTimeStampType uint8 = 130
)

// getResumeTokenHexBytes returns the hex string bytes of a resume token’s
// "_data" field without allocating a string.
func getResumeTokenHexBytes(rt bson.Raw) ([]byte, error) {
	rv, err := rt.LookupErr("_data")
	if err != nil {
		return nil, fmt.Errorf("parse resume token: %w", err)
	}

	b, err := bsontools.RawValueToStringBytes(rv)
	if err != nil {
		return nil, fmt.Errorf("parse resume token: %w", err)
	}

	return b, nil
}
