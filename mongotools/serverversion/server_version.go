// Package serverversion provides a canonical representation of Server versions, along with version pair
// types and helpers for working with the set of versions a product supports.
//
// The package is shared between the DB Tools and mongosync repos. Anything that is specific to one
// product - pinned server/resmoke commits, the map of supported version pairs, Evergreen project
// IDs, and so on - belongs in that product's repo, not here. The SupportedPairs type in this
// package lets each product define its own pair map and get generic helper methods for it.
package serverversion

import (
	"cmp"
	"encoding"
	"fmt"
	"strconv"
	"strings"

	"github.com/samber/lo"
)

// ServerVersion represents a major and minor version combination for the server. This should
// be used as the canonical way to represent server versions.
//
// Only the `Unmarshal*` methods take a pointer, which they need to do since they populate the
// struct given as the receiver.
type ServerVersion struct {
	major, minor int
	storage      StorageType
}

var _ encoding.TextMarshaler = &ServerVersion{}
var _ encoding.TextUnmarshaler = &ServerVersion{}

// New returns a ServerVersion for the given major and minor versions running on attached storage.
func New(major, minor int) ServerVersion {
	return ServerVersion{
		major:   major,
		minor:   minor,
		storage: AttachedStorage,
	}
}

// NewWithStorage returns a ServerVersion for the given major and minor versions running on the
// given storage type.
func NewWithStorage(major, minor int, storage StorageType) ServerVersion {
	return ServerVersion{
		major:   major,
		minor:   minor,
		storage: storage,
	}
}

const serverVersionParseErrorStr = `failed to parse %#q as ServerVersion:` +
	` version must have the format: major.minor(.patch)(-storage)`

// Parse creates a Server version from a string of the format "major.minor.*".
//
// This doesn't check whether the version is supported. You can use `SupportedPairs.Contains` to do
// that after parsing.
func Parse(str string) (ServerVersion, error) {
	storageSplit := strings.Split(str, "-")

	numSplit := strings.Split(storageSplit[0], ".")
	anyNumsAreEmpty := len(lo.Compact(numSplit)) != len(numSplit)

	if len(numSplit) < 2 || anyNumsAreEmpty {
		return ServerVersion{}, fmt.Errorf(serverVersionParseErrorStr, str)
	}

	major, err := strconv.Atoi(numSplit[0])
	if err != nil {
		return ServerVersion{}, fmt.Errorf("major: "+serverVersionParseErrorStr+": %w", str, err)
	}
	minor, err := strconv.Atoi(numSplit[1])
	if err != nil {
		return ServerVersion{}, fmt.Errorf("minor: "+serverVersionParseErrorStr+": %w", str, err)
	}

	switch len(storageSplit) {
	case 0:
		panic("cannot happen")
	case 1:
		return ServerVersion{
			major:   major,
			minor:   minor,
			storage: AttachedStorage,
		}, nil
	case 2:
		storage := storageSplit[1]
		if storage != DisaggregatedStorage.Suffix() {
			return ServerVersion{}, fmt.Errorf(
				serverVersionParseErrorStr+", unrecognized storage engine: %#q",
				str,
				storage,
			)
		}

		return ServerVersion{
			major:   major,
			minor:   minor,
			storage: DisaggregatedStorage,
		}, nil
	default:
		return ServerVersion{}, fmt.Errorf(serverVersionParseErrorStr, str)
	}
}

// FromSlice creates a server version from an integer slice, e.g. the `versionArray` field of the
// buildInfo command. Note: Don't use this in new code. This mostly exists for compatibility with
// the `buildinfo`-style version arrays used by existing callers.
func FromSlice(arr []int, storageType StorageType) ServerVersion {
	return ServerVersion{major: arr[0], minor: arr[1], storage: storageType}
}

func (v ServerVersion) IsEmpty() bool {
	return v == ServerVersion{}
}

func (v ServerVersion) IsDisagg() bool {
	return v.storage.IsDisagg()
}

// StorageType returns the storage type of this ServerVersion.
func (v ServerVersion) StorageType() StorageType {
	return v.storage
}

// Compare implements the standard `Compare` API for the `ServerVersion` type. After major/minor
// versions, this sorts ASC clusters before DSC clusters.
//
// Note that this storage-type ordering exists only to produce a consistent sort order for lists
// of versions. The LT/LTE/GT/GTE/Equals methods all ignore storage type.
func (v ServerVersion) Compare(other ServerVersion) int {
	return cmp.Or(
		cmp.Compare(v.major, other.major),
		cmp.Compare(v.minor, other.minor),
		cmp.Compare(v.storage, other.storage),
	)
}

// compareNoStorage compares only the major and minor versions, ignoring storage type.
func (v ServerVersion) compareNoStorage(other ServerVersion) int {
	return cmp.Or(
		cmp.Compare(v.major, other.major),
		cmp.Compare(v.minor, other.minor),
	)
}

// LT returns true if the current version is less than the given version, ignoring storage type:
// a 9.1 ASC cluster and a 9.1 DSC cluster compare equal.
func (v ServerVersion) LT(other ServerVersion) bool {
	return v.compareNoStorage(other) < 0
}

// LTE returns true if the current version is less than or equal to the given version, ignoring
// storage type: a 9.1 ASC cluster and a 9.1 DSC cluster compare equal.
func (v ServerVersion) LTE(other ServerVersion) bool {
	return v.compareNoStorage(other) <= 0
}

// GT returns true if the current version is greater than the given version, ignoring storage
// type: a 9.1 ASC cluster and a 9.1 DSC cluster compare equal.
func (v ServerVersion) GT(other ServerVersion) bool {
	return v.compareNoStorage(other) > 0
}

// GTE returns true if the current version is greater than or equal to the given version,
// ignoring storage type: a 9.1 ASC cluster and a 9.1 DSC cluster compare equal.
func (v ServerVersion) GTE(other ServerVersion) bool {
	return v.compareNoStorage(other) >= 0
}

// Equals returns true if the two versions represent the same Server version, ignoring their
// patch version and storage type.
func (v ServerVersion) Equals(other ServerVersion) bool {
	return v.compareNoStorage(other) == 0
}

func (v ServerVersion) Major() int {
	return v.major
}

func (v ServerVersion) Minor() int {
	return v.minor
}

// VString returns a v-prefixed string, e.g. "v50" or "v91dsc".
func (v ServerVersion) VString() string {
	return fmt.Sprintf("v%d%d%s", v.major, v.minor, v.storage.Suffix())
}

// VStringNoStorage returns a v-prefixed string without any storage engine suffix, e.g. "v91" for
// both a 9.1 ASC and a 9.1 DSC version.
func (v ServerVersion) VStringNoStorage() string {
	return fmt.Sprintf("v%d%d", v.major, v.minor)
}

// String returns a decimal string with “-dsc” appended for DSC clusters.  e.g. "5.0" or "9.1-dsc".
func (v ServerVersion) String() string {
	ret := v.StringNoStorage()

	storSuffix := v.storage.Suffix()

	if len(storSuffix) > 0 {
		ret = fmt.Sprintf("%s-%s", ret, storSuffix)
	}

	return ret
}

// StringNoStorage is like String but omits the storage engine.
//
// This is useful to interface with other systems (e.g. mongodb-runner) which don’t understand the
// "-dsc" suffix on DSC versions.
func (v ServerVersion) StringNoStorage() string {
	return fmt.Sprintf("%d.%d", v.major, v.minor)
}

// StringNoDot returns a string without dot, e.g. "50" or "91-dsc".
func (v ServerVersion) StringNoDot() string {
	return strings.ReplaceAll(v.String(), ".", "")
}

// MarshalText implements encoding.TextMarshaler, which means that we encode versions as strings for
// JSON.
func (v ServerVersion) MarshalText() ([]byte, error) {
	return []byte(v.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler, which means that we decode versions as strings
// from JSON.
func (v *ServerVersion) UnmarshalText(text []byte) error {
	version, err := Parse(string(text))
	if err != nil {
		return err
	}

	*v = version
	return err
}
