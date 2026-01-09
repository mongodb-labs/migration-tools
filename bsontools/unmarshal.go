package bsontools

import (
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// UnmarshalRaw mimics bson.Unmarshal to a bson.D.
func UnmarshalRaw(raw bson.Raw) (bson.D, error) {
	els, err := raw.Elements()
	if err != nil {
		return nil, fmt.Errorf("extracting elements: %w", err)
	}

	d := bson.D(make([]bson.E, len(els)))

	for e, el := range els {
		key, err := el.KeyErr()
		if err != nil {
			return nil, fmt.Errorf("extracting field %d’s name: %w", e, err)
		}

		d[e].Key = key

		val, err := el.ValueErr()
		if err != nil {
			return nil, fmt.Errorf("extracting %#q value: %w", key, err)
		}

		d[e].Value, err = unmarshalValue(val)
		if err != nil {
			return nil, fmt.Errorf("extracting %#q value: %w", key, err)
		}
	}

	return d, nil
}

// UnmarshalArray is like UnmarshalRaw but for an array.
func UnmarshalArray(raw bson.RawArray) (bson.A, error) {
	vals, err := raw.Values()
	if err != nil {
		return nil, fmt.Errorf("extracting elements: %w", err)
	}

	a := make(bson.A, len(vals))

	for e, val := range vals {
		a[e], err = unmarshalValue(val)
		if err != nil {
			return nil, fmt.Errorf("unmarshaling element %d: %w", e, err)
		}
	}

	return a, nil
}

func unmarshalValue(val bson.RawValue) (any, error) {
	switch val.Type {
	case bson.TypeDouble:
		return val.Double(), nil
	case bson.TypeString:
		return val.StringValue(), nil
	case bson.TypeEmbeddedDocument:
		tVal, err := UnmarshalRaw(val.Value)
		if err != nil {
			return nil, fmt.Errorf("unmarshaling subdoc: %w", err)
		}

		return tVal, nil
	case bson.TypeArray:
		tVal, err := UnmarshalArray(val.Value)
		if err != nil {
			return nil, fmt.Errorf("unmarshaling array: %w", err)
		}

		return tVal, nil
	case bson.TypeBinary:
		subtype, bin := val.Binary()
		return bson.Binary{subtype, bin}, nil
	case bson.TypeUndefined:
		return bson.Undefined{}, nil
	case bson.TypeObjectID:
		return val.ObjectID(), nil
	case bson.TypeBoolean:
		return val.Boolean(), nil
	case bson.TypeDateTime:
		return bson.DateTime(val.DateTime()), nil
	case bson.TypeNull:
		return nil, nil
	case bson.TypeRegex:
		pattern, opts := val.Regex()
		return bson.Regex{pattern, opts}, nil
	case bson.TypeDBPointer:
		db, ptr := val.DBPointer()
		return bson.DBPointer{DB: db, Pointer: ptr}, nil
	case bson.TypeJavaScript:
		return bson.JavaScript(val.JavaScript()), nil
	case bson.TypeSymbol:
		return bson.Symbol(val.Symbol()), nil
	case bson.TypeCodeWithScope:
		code, scope := val.CodeWithScope()
		return bson.CodeWithScope{
			Code:  bson.JavaScript(code),
			Scope: scope,
		}, nil
	case bson.TypeInt32:
		return val.Int32(), nil
	case bson.TypeTimestamp:
		t, i := val.Timestamp()
		return bson.Timestamp{t, i}, nil
	case bson.TypeInt64:
		return val.Int64(), nil
	case bson.TypeDecimal128:
		return val.Decimal128(), nil
	case bson.TypeMaxKey:
		return bson.MaxKey{}, nil
	case bson.TypeMinKey:
		return bson.MinKey{}, nil
	default:
		panic(fmt.Sprintf("unknown BSON type: %d", val.Type))
	}
}
