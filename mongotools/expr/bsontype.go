package expr

// Type is a BSON type name as used in $type expressions. See the
// $type operator’s documentation for the canonical list of names:
// https://www.mongodb.com/docs/manual/reference/operator/aggregation/type/
type Type string

const (
	TypeDouble              Type = "double"
	TypeString              Type = "string"
	TypeObject              Type = "object"
	TypeArray               Type = "array"
	TypeBinData             Type = "binData"
	TypeUndefined           Type = "undefined"
	TypeObjectID            Type = "objectId"
	TypeBool                Type = "bool"
	TypeDate                Type = "date"
	TypeNull                Type = "null"
	TypeRegex               Type = "regex"
	TypeDBPointer           Type = "dbPointer"
	TypeJavaScript          Type = "javascript"
	TypeSymbol              Type = "symbol"
	TypeJavaScriptWithScope Type = "javascriptWithScope"
	TypeInt                 Type = "int"
	TypeTimestamp           Type = "timestamp"
	TypeLong                Type = "long"
	TypeDecimal             Type = "decimal"
	TypeMinKey              Type = "minKey"
	TypeMaxKey              Type = "maxKey"

	// TypeNumber is an alias that matches TypeDouble, TypeInt,
	// TypeLong, and TypeDecimal.
	TypeNumber Type = "number"
)
