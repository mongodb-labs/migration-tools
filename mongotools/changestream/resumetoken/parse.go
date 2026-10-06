// Package resumetoken exposes tooling around change stream resume tokens.
package resumetoken

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/mongodb-labs/migration-tools/bsontools"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type TokenType byte

const (
	rtTimeStampType = 130

	// TokenTypeEvent is for resume tokens that represent events.
	TokenTypeEvent TokenType = 128

	// TokenTypeHighWaterMark is for high-water-mark resume tokens. Such tokens
	// are how change streams advance even without any change events.
	TokenTypeHighWaterMark TokenType = 0
)

// Parsed represents the parse of a resume token. It’s incomplete for now.
type Parsed struct {
	Timestamp bson.Timestamp
	Version   byte
	TokenType TokenType
}

// Parse returns a Parsed struct with information about the resume token.
// Currently this supports only v1 and v2 resume tokens, not v0.
func Parse(rt bson.Raw) (Parsed, error) {
	dataString, err := bsontools.RawLookup[string](rt, "_data")
	if err != nil {
		return Parsed{}, fmt.Errorf("reading resume token %#q (%v): %w", "_data", rt, err)
	}
	reader := bufio.NewReader(hex.NewDecoder(strings.NewReader(dataString)))

	if err := assertKeyStringType(reader); err != nil {
		return Parsed{}, err
	}

	var p Parsed

	p.Timestamp, err = readTimestamp(reader)
	if err != nil {
		return Parsed{}, err
	}

	afterTS := [5]byte{}
	if _, err := io.ReadFull(reader, afterTS[:]); err != nil {
		return Parsed{}, fmt.Errorf("read bytes after timestamp: %w", err)
	}

	// The following are specific byte sequences that come from the server’s
	// internal key string format for BSON values. See the
	// $_internalKeyStringValue aggregation operator for more context on the
	// key string format.

	p.Version, err = parseVersion(afterTS[:2])
	if err != nil {
		return Parsed{}, err
	}

	p.TokenType, err = parseTokenType(afterTS[2:])
	if err != nil {
		return Parsed{}, err
	}

if _, err := io.Copy(io.Discard, reader); err != nil {
	return Parsed{}, fmt.Errorf("decode resume token data: %w", err)
}

return p, nil
}

func readTimestamp(reader io.Reader) (bson.Timestamp, error) {
	tsBytes := [8]byte{}
	if _, err := io.ReadFull(reader, tsBytes[:]); err != nil {
		return bson.Timestamp{}, fmt.Errorf("read timestamp: %w", err)
	}

	return bson.Timestamp{
		T: binary.BigEndian.Uint32(tsBytes[:4]),
		I: binary.BigEndian.Uint32(tsBytes[4:]),
	}, nil
}

func parseVersion(versionBytes []byte) (byte, error) {
	switch {
	case bytes.HasPrefix(versionBytes, []byte{0x2b, 0x02}):
		return 1, nil
	case bytes.HasPrefix(versionBytes, []byte{0x2b, 0x04}):
		return 2, nil
	}

	return 0, fmt.Errorf("unexpected resume token version bytes: %x", versionBytes)
}

func parseTokenType(typeBytes []byte) (TokenType, error) {
	switch {
	case bytes.HasPrefix(typeBytes, []byte{0x29}):
		return TokenTypeHighWaterMark, nil
	case bytes.HasPrefix(typeBytes, []byte{0x2c, 0x01, 0x00}):
		return TokenTypeEvent, nil
	}

	return 0, fmt.Errorf("unexpected token type bytes: %x", typeBytes)
}

func assertKeyStringType(hexReader io.ByteReader) error {
	typeIdent, err := readType(hexReader)
	if err != nil {
		return fmt.Errorf("read key string type byte: %w", err)
	}
	if typeIdent != rtTimeStampType {
		return fmt.Errorf(
			"unexpected key string type: %v (expected %v)",
			typeIdent,
			rtTimeStampType,
		)
	}
	return nil
}

func readType(reader io.ByteReader) (uint8, error) {
	var t uint8
	t, err := reader.ReadByte()
	if err != nil {
		return 0, fmt.Errorf("failed to read type byte: %w", err)
	}
	return t, nil
}
