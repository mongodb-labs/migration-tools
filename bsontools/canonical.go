package bsontools

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/samber/lo"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// SortFields sorts a BSON document’s fields recursively.
func SortFields(in bson.Raw) (bson.Raw, error) {
	els, err := in.Elements()
	if err != nil {
		return nil, fmt.Errorf("parsing fields: %w", err)
	}

	for _, el := range els {
		bType := bson.Type(el[0])

		switch bType {
		case bson.TypeEmbeddedDocument:
			key, err := el.KeyErr()
			if err != nil {
				return nil, fmt.Errorf("getting field name: %w", err)
			}

			val, err := el.ValueErr()
			if err != nil {
				return nil, fmt.Errorf("getting %#q’s value: %w", key, err)
			}

			subdoc, err := RawValueTo[bson.Raw](val)
			if err != nil {
				return nil, fmt.Errorf("getting %#q as subdoc: %w", key, err)
			}

			newSubdoc, err := SortFields(subdoc)
			if err != nil {
				return nil, fmt.Errorf("sorting subdoc %#q: %w", key, err)
			}

			copy(el[2+len(key):], newSubdoc)
		case bson.TypeArray:
			key, err := el.KeyErr()
			if err != nil {
				return nil, fmt.Errorf("getting field name: %w", err)
			}

			val, err := el.ValueErr()
			if err != nil {
				return nil, fmt.Errorf("getting %#q’s value: %w", key, err)
			}

			array, err := RawValueTo[bson.RawArray](val)
			if err != nil {
				return nil, fmt.Errorf("getting %#q as array: %w", key, err)
			}

			for 
		}
	}

	newDoc := make(bson.Raw, 4, len(in))
	copy(newDoc, in[:4])

	slices.SortStableFunc(
		els,
		func(a, b bson.RawElement) int {
			return cmp.Compare(
				lo.Must(a.KeyErr()),
				lo.Must(b.KeyErr()),
			)
		},
	)

	for _, el := range els {
		newDoc = append(newDoc, el...)
	}

	newDoc = append(newDoc, 0)

	return newDoc, nil
}
