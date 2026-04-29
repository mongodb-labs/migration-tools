package legacytools

import (
	"github.com/samber/lo"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/topology"
)

// cf. https://github.com/mongodb/specifications/blob/b519824da64005cdf99ca680fc49c4e278af0ef3/source/wireversion-featurelist/wireversion-featurelist.md
var dbWireVersion = map[string]int32{
	"4.0": 7,
	"4.2": 8,
}

// SetDriverCompatibility alters the driver’s global state so that it will
// try to support an older DB version than it otherwise would.
//
// It goes without saying: HANDLE WITH CARE!
func SetDriverCompatibility(dbVersion string) {
	wireVersion, ok := dbWireVersion[dbVersion]
	lo.Assert(
		ok,
		"need wire version for db version %#q",
		dbVersion,
	)

	topology.MinSupportedMongoDBVersion = dbVersion
	topology.SupportedWireVersions.Min = wireVersion
}
