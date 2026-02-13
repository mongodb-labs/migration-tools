package mongotools

import mapset "github.com/deckarep/golang-set/v2"

var validIndexOptions = mapset.NewSet(
	"2dsphereIndexVersion",
	"background",
	"bits",
	"bucketSize",
	"coarsestIndexedLevel",
	"collation",
	"default_language",
	"expireAfterSeconds",
	"finestIndexedLevel",
	"key",
	"language_override",
	"max",
	"min",
	"name",
	"ns",
	"partialFilterExpression",
	"sparse",
	"storageEngine",
	"textIndexVersion",
	"unique",
	"v",
	"weights",
	"wildcardProjection",
)

func GetValidIndexOptions() mapset.Set[string] {
	return validIndexOptions.Clone()
}
