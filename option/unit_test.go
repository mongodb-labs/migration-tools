package option

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func Test_Option_BSON(t *testing.T) {
	type MyType struct {
		IsNone          Option[int]
		IsNoneOmitEmpty Option[int] `bson:",omitempty"`
		IsSome          Option[bool]
	}

	type MyTypePtrs struct {
		IsNone          *int
		IsNoneOmitEmpty *int `bson:",omitempty"`
		IsSome          *bool
	}

	t.Run(
		"marshal pointer, unmarshal Option",
		func(t *testing.T) {

			bytes, err := bson.Marshal(MyTypePtrs{
				IsNoneOmitEmpty: pointerTo(234),
				IsSome:          pointerTo(false),
			})
			require.NoError(t, err)

			rt := MyType{}
			require.NoError(t, bson.Unmarshal(bytes, &rt))

			assert.Equal(t,
				MyType{
					IsNoneOmitEmpty: Some(234),
					IsSome:          Some(false),
				},
				rt,
			)
		},
	)

	t.Run(
		"marshal Option, unmarshal pointer",
		func(t *testing.T) {

			bytes, err := bson.Marshal(MyType{
				IsNoneOmitEmpty: Some(234),
				IsSome:          Some(false),
			})
			require.NoError(t, err)

			rt := MyTypePtrs{}
			require.NoError(t, bson.Unmarshal(bytes, &rt))

			assert.Equal(t,
				MyTypePtrs{
					IsNoneOmitEmpty: pointerTo(234),
					IsSome:          pointerTo(false),
				},
				rt,
			)
		},
	)

	t.Run(
		"round-trip bson.D",
		func(t *testing.T) {
			simpleDoc := bson.D{
				{"a", None[int]()},
				{"b", Some(123)},
			}

			bytes, err := bson.Marshal(simpleDoc)
			require.NoError(t, err)

			rt := bson.D{}
			require.NoError(t, bson.Unmarshal(bytes, &rt))

			assert.Equal(t,
				bson.D{{"a", nil}, {"b", int32(123)}},
				rt,
			)
		},
	)

	t.Run(
		"round-trip struct",
		func(t *testing.T) {
			myThing := MyType{None[int](), None[int](), Some(true)}

			bytes, err := bson.Marshal(&myThing)
			require.NoError(t, err)

			// Unmarshal to a bson.D to test `omitempty`.
			rtDoc := bson.D{}
			require.NoError(t, bson.Unmarshal(bytes, &rtDoc))

			keys := make([]string, 0)
			for _, el := range rtDoc {
				keys = append(keys, el.Key)
			}

			assert.ElementsMatch(t,
				[]string{"isnone", "issome"},
				keys,
			)

			rtStruct := MyType{}
			require.NoError(t, bson.Unmarshal(bytes, &rtStruct))
			assert.Equal(t,
				myThing,
				rtStruct,
			)
		},
	)
}

func Test_Option_JSON(t *testing.T) {
	type MyType struct {
		IsNone  Option[int]
		Omitted Option[int]
		IsSome  Option[bool]
	}

	type MyTypePtrs struct {
		IsNone  *int
		Omitted *int
		IsSome  *bool
	}

	t.Run(
		"marshal pointer, unmarshal Option",
		func(t *testing.T) {

			bytes, err := json.Marshal(MyTypePtrs{
				IsNone: pointerTo(234),
				IsSome: pointerTo(false),
			})
			require.NoError(t, err)

			rt := MyType{}
			require.NoError(t, json.Unmarshal(bytes, &rt))

			assert.Equal(t,
				MyType{
					IsNone: Some(234),
					IsSome: Some(false),
				},
				rt,
			)
		},
	)

	t.Run(
		"marshal Option, unmarshal pointer",
		func(t *testing.T) {

			bytes, err := json.Marshal(MyType{
				IsNone: Some(234),
				IsSome: Some(false),
			})
			require.NoError(t, err)

			rt := MyTypePtrs{}
			require.NoError(t, json.Unmarshal(bytes, &rt))

			assert.Equal(t,
				MyTypePtrs{
					IsNone: pointerTo(234),
					IsSome: pointerTo(false),
				},
				rt,
			)
		},
	)

	t.Run(
		"round-trip bson.D",
		func(t *testing.T) {
			simpleDoc := bson.D{
				{"a", None[int]()},
				{"b", Some(123)},
			}

			bytes, err := json.Marshal(simpleDoc)
			require.NoError(t, err)

			rt := bson.D{}
			require.NoError(t, json.Unmarshal(bytes, &rt))

			assert.Equal(t,
				bson.D{{"a", nil}, {"b", float64(123)}},
				rt,
			)
		},
	)

	t.Run(
		"round-trip struct",
		func(t *testing.T) {
			myThing := MyType{None[int](), None[int](), Some(true)}

			bytes, err := json.Marshal(&myThing)
			require.NoError(t, err)

			rtStruct := MyType{}
			require.NoError(t, json.Unmarshal(bytes, &rtStruct))
			assert.Equal(t,
				myThing,
				rtStruct,
			)
		},
	)
}

func Test_Option_NoNilSome(t *testing.T) {
	assertPanics(t, (chan int)(nil))
	assertPanics(t, (func())(nil))
	assertPanics(t, any(nil))
	assertPanics(t, map[int]any(nil))
	assertPanics(t, []any(nil))
	assertPanics(t, (*any)(nil))
}

func Test_Option_Pointer(t *testing.T) {
	opt := Some(123)
	ptr := opt.ToPointer()
	*ptr = 1234

	assert.Equal(t,
		Some(123),
		opt,
		"ToPointer() sholuldn’t let caller alter Option value",
	)

	opt2 := FromPointer(ptr)
	*ptr = 2345
	assert.Equal(t,
		Some(1234),
		opt2,
		"FromPointer() sholuldn’t let caller alter Option value",
	)
}

func Test_Option(t *testing.T) {

	//nolint:testifylint  // None is, in fact, the expected value.
	assert.Equal(t,
		None[int](),
		Option[int]{},
		"zero value is None",
	)

	//nolint:testifylint
	assert.Equal(t, Some(1), Some(1), "same internal value")
	assert.NotEqual(t, Some(1), Some(2), "different internal value")

	foo := "foo"
	fooPtr := Some(foo).ToPointer()

	assert.Equal(t, &foo, fooPtr)

	assert.Equal(t, Some(foo), FromPointer(fooPtr))

	assert.Equal(t,
		foo,
		Some(foo).OrZero(),
	)

	assert.Equal(t,
		"",
		None[string]().OrZero(),
	)

	assert.Equal(t,
		"elf",
		None[string]().OrElse("elf"),
	)

	val, has := Some(123).Get()
	assert.True(t, has)
	assert.Equal(t, 123, val)

	val, has = None[int]().Get()
	assert.False(t, has)
	assert.Equal(t, 0, val)

	some := Some(456)
	assert.True(t, some.IsSome())
	assert.False(t, some.IsNone())

	none := None[int]()
	assert.False(t, none.IsSome())
	assert.True(t, none.IsNone())
}

func Test_Option_IfNonZero(t *testing.T) {
	assertIfNonZero(t, 0, 1)
	assertIfNonZero(t, "", "a")
	assertIfNonZero(t, []int(nil), []int{})
	assertIfNonZero(t, map[int]int(nil), map[int]int{})
	assertIfNonZero(t, any(nil), any(0))
	assertIfNonZero(t, bson.D(nil), bson.D{})

	type myStruct struct {
		name string
	}

	assertIfNonZero(t, myStruct{}, myStruct{"foo"})
}

func assertIfNonZero[T any](t *testing.T, zeroVal, nonZeroVal T) {
	noneOpt := IfNotZero(zeroVal)
	someOpt := IfNotZero(nonZeroVal)

	assert.Equal(t, None[T](), noneOpt)
	assert.Equal(t, Some(nonZeroVal), someOpt)
}

func pointerTo[T any](val T) *T {
	return &val
}

func assertPanics[T any](t *testing.T, val T) {
	t.Helper()

	assert.Panics(
		t,
		func() { Some(val) },
		"Some(%T)",
		val,
	)

	assert.Panics(
		t,
		func() { FromPointer(&val) },
		"FromPointer(&%T)",
		val,
	)
}
