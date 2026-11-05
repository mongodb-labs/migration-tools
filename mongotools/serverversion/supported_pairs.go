package serverversion

import (
	"maps"
	"slices"
	"strings"

	mapset "github.com/deckarep/golang-set/v3"
)

// SupportedPairs maps each supported source ServerVersion to the set of destination versions it
// supports. Each product defines its own value; the methods on this type provide generic helpers
// for working with that product's supported versions.
type SupportedPairs map[ServerVersion]mapset.Set[ServerVersion]

// LatestSource returns the latest server version that can be a migration source. If ASC & DSC
// versions of the same major/minor pair are supported, this returns the DSC version.
func (pairs SupportedPairs) LatestSource() ServerVersion {
	sources := slices.SortedFunc(
		maps.Keys(pairs),
		ServerVersion.Compare,
	)

	return sources[len(sources)-1]
}

// Latest returns the latest-supported server version (source or destination). If ASC & DSC versions
// of the same major/minor pair are supported, this returns the DSC version.
func (pairs SupportedPairs) Latest() ServerVersion {
	serverVersions := pairs.SupportedVersions()
	slices.Reverse(serverVersions)

	return serverVersions[0]
}

// LatestPair returns the greatest supported `VersionPair`, i.e. the last entry of `Pairs`. The
// source and destination versions are not necessarily equal, and are not necessarily the latest
// supported versions overall: a product may add support for a new version as a destination
// before adding it as a source, in which case that version cannot pair with itself yet.
//
// It panics if there are no supported pairs.
func (pairs SupportedPairs) LatestPair() VersionPair {
	allPairs := pairs.Pairs()

	return allPairs[len(allPairs)-1]
}

// SupportedVersions returns a slice of all supported server versions (as either source or
// destination), sorted in ascending version order.
func (pairs SupportedPairs) SupportedVersions() []ServerVersion {
	allVersions := mapset.NewSet[ServerVersion]()

	for src, dsts := range pairs {
		allVersions.Add(src)
		allVersions.Append(dsts.ToSlice()...)
	}

	sorted := allVersions.ToSlice()
	slices.SortFunc(sorted, ServerVersion.Compare)

	return sorted
}

// SupportedVersionsSet returns a set of all supported server versions.
func (pairs SupportedPairs) SupportedVersionsSet() mapset.Set[ServerVersion] {
	return mapset.NewSet(pairs.SupportedVersions()...)
}

// SupportedVersionsString returns a space-separated string of the SupportedVersions, suitable for
// use in an error message.
func (pairs SupportedPairs) SupportedVersionsString() string {
	sv := pairs.SupportedVersions()
	strs := make([]string, len(sv))
	for i, v := range sv {
		strs[i] = v.String()
	}

	return strings.Join(strs, " ")
}

// Contains returns whether the given version is supported (as either a source or a destination).
func (pairs SupportedPairs) Contains(v ServerVersion) bool {
	return pairs.SupportedVersionsSet().Contains(v)
}

// Pairs returns a slice of all supported version pairs, sorted first by source version and then by
// destination version, in ascending version order.
func (pairs SupportedPairs) Pairs() []VersionPair {
	var allPairs []VersionPair

	for src, dsts := range pairs {
		for _, dst := range dsts.ToSlice() {
			allPairs = append(allPairs, VersionPair{SrcVersion: src, DstVersion: dst})
		}
	}

	slices.SortFunc(allPairs, VersionPair.Compare)

	return allPairs
}

// PairsSet returns a set of all supported version pairs.
func (pairs SupportedPairs) PairsSet() mapset.Set[VersionPair] {
	return mapset.NewSet(pairs.Pairs()...)
}
