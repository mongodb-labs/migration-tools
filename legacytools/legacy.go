package legacytools

import (
	"github.com/samber/lo"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/topology"
)

var dbWireVersion = map[string]int32{
	"4.0": 7,
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
