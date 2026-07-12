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

func getResumeTokenHexString(rt bson.Raw) (string, error) {
	// TODO optimize

	// The resume token is a BSON document with a single field, "_data",
	// whose value is a hex-encoded string.
	dataStr, err := bsontools.RawLookup[string](rt, "_data")
	if err != nil {
		return "", fmt.Errorf("parse resume token to string: %w", err)
	}

	return dataStr, nil
}
