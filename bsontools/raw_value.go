package bsontools

import (
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type bsonCastRecipient interface {
	// BSON types:
	bson.Raw | bson.RawArray | bson.DateTime | bson.Decimal128 |
		bson.Timestamp | bson.ObjectID | bson.Binary | bson.Regex |

		// Go types:
		bool | string | int32 | int64 | float64 | time.Time
}

type cannotCastErr struct {
	gotBSONType bson.Type
	toGoType    any
}

func (ce cannotCastErr) Error() string {
	return fmt.Sprintf("cannot cast BSON %s to %T", ce.gotBSONType, ce.toGoType)
}

// RawValueTo is a bit like bson.UnmarshalValue, but it’s much faster because
// it avoids reflection. The downside is that only certain types are supported.
//
// This enforces strict type equivalence. For example, it won’t coerce a float
// to an int.
//
// Example usage:
//
//	str, err := RawValueTo[string](rv)
func RawValueTo[T bsonCastRecipient](in bson.RawValue) (T, error) {
	var zero T

	switch any(zero).(type) {
	case bson.Raw:
		if doc, isDoc := in.DocumentOK(); isDoc {
			return any(doc).(T), nil
		}
	case bson.RawArray:
		if arr, ok := in.ArrayOK(); ok {
			return any(arr).(T), nil
		}
	case bson.DateTime:
		if i64, ok := in.DateTimeOK(); ok {
			return any(bson.DateTime(i64)).(T), nil
		}
	case bson.Decimal128:
		if dec, ok := in.Decimal128OK(); ok {
			return any(dec).(T), nil
		}
	case bson.Timestamp:
		if t, i, ok := in.TimestampOK(); ok {
			return any(bson.Timestamp{t, i}).(T), nil
		}
	case bson.ObjectID:
		if oid, ok := in.ObjectIDOK(); ok {
			return any(oid).(T), nil
		}
	case bson.Binary:
		if subtype, buf, ok := in.BinaryOK(); ok {
			return any(bson.Binary{subtype, buf}).(T), nil
		}
	case bson.Regex:
		if pattern, opts, ok := in.RegexOK(); ok {
			return any(bson.Regex{pattern, opts}).(T), nil
		}
	case bool:
		if val, ok := in.BooleanOK(); ok {
			return any(val).(T), nil
		}
	case string:
		if str, ok := in.StringValueOK(); ok {
			return any(str).(T), nil
		}
	case int32:
		if val, ok := in.Int32OK(); ok {
			return any(val).(T), nil
		}
	case int64:
		if val, ok := in.Int64OK(); ok {
			return any(val).(T), nil
		}
	case float64:
		if val, ok := in.DoubleOK(); ok {
			return any(val).(T), nil
		}
	case time.Time:
		if val, ok := in.TimeOK(); ok {
			return any(val).(T), nil
		}
	default:
		panic(fmt.Sprintf("Unrecognized Go type: %T (missing case?)", zero))
	}

	return zero, cannotCastErr{in.Type, zero}
}
