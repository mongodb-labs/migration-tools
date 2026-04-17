package testtools

import (
	"errors"
	"reflect"

	"github.com/samber/lo"
)

// CloneExported deeply clones a struct (or pointer to a struct),
// recursively leaving all unexported fields as their zero values.
func CloneExported[T any](input T) (T, error) {
	var zero T

	v := reflect.ValueOf(input)

	// Fast fail if it's not a struct or pointer to a struct at the top level
	isPtr := v.Kind() == reflect.Pointer
	if isPtr {
		if v.IsNil() {
			return input, nil // Return as-is if it's a nil pointer
		}
		if v.Elem().Kind() != reflect.Struct {
			return zero, errors.New("input must be a struct or a pointer to a struct")
		}
	} else if v.Kind() != reflect.Struct {
		return zero, errors.New("input must be a struct or a pointer to a struct")
	}

	cloned := cloneValue(v)

	return lo.Must(reflect.TypeAssert[T](cloned)), nil
}

// cloneValue recursively walks through a reflect.Value and copies it,
// zeroing out unexported struct fields along the way.
//
//nolint:cyclop
func cloneValue(v reflect.Value) reflect.Value {
	if !v.IsValid() {
		return v
	}

	switch v.Kind() {
	case reflect.Ptr:
		if v.IsNil() {
			return v
		}
		// Create a new pointer of the same type and recurse into the element
		clone := reflect.New(v.Type().Elem())
		clone.Elem().Set(cloneValue(v.Elem()))
		return clone

	case reflect.Struct:
		t := v.Type()
		clone := reflect.New(t).Elem()
		for i := range t.NumField() {
			field := t.Field(i)
			if field.IsExported() {
				// Recurse into the exported field
				clone.Field(i).Set(cloneValue(v.Field(i)))
			}
		}
		return clone

	case reflect.Slice:
		if v.IsNil() {
			return v
		}
		// Create a new slice and recurse into each element
		clone := reflect.MakeSlice(v.Type(), v.Len(), v.Cap())
		for i := range v.Len() {
			clone.Index(i).Set(cloneValue(v.Index(i)))
		}
		return clone

	case reflect.Map:
		if v.IsNil() {
			return v
		}
		// Create a new map and recurse into both keys and values
		clone := reflect.MakeMap(v.Type())
		for _, key := range v.MapKeys() {
			clone.SetMapIndex(cloneValue(key), cloneValue(v.MapIndex(key)))
		}
		return clone

	case reflect.Interface:
		if v.IsNil() {
			return v
		}
		// Recurse into the underlying concrete value
		return cloneValue(v.Elem())

	default:
		// For basic primitives (int, string, bool, etc.), just return the value
		return v
	}
}
