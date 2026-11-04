package serverversion

import (
	"cmp"
	"fmt"
)

// VersionPair represents a pair of source and destination Server versions, for example for a
// migration or for a passthrough test matrix.
type VersionPair struct {
	SrcVersion, DstVersion ServerVersion
}

// Equals returns true if the two `VersionPair` values are the same.
func (vp VersionPair) Equals(other VersionPair) bool {
	return vp.SrcVersion.Equals(other.SrcVersion) && vp.DstVersion.Equals(other.DstVersion)
}

// Compare returns an integer for use in sorting functions.
func (vp VersionPair) Compare(other VersionPair) int {
	return cmp.Or(
		vp.SrcVersion.Compare(other.SrcVersion),
		vp.DstVersion.Compare(other.DstVersion),
	)
}

// UnderscoreString returns a string like "v50_to_v60".
func (vp VersionPair) UnderscoreString() string {
	return fmt.Sprintf("%s_to_%s", vp.SrcVersion.VString(), vp.DstVersion.VString())
}
