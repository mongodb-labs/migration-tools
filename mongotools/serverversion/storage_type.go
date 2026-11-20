package serverversion

// StorageType represents the storage engine a ServerVersion runs on: either classic attached
// storage or disaggregated storage (DSC).
type StorageType string

const (
	// AttachedStorage is the classic (non-disaggregated) storage engine.
	AttachedStorage StorageType = "AttachedStorage"
	// DisaggregatedStorage is the disaggregated (DSC) storage engine.
	DisaggregatedStorage StorageType = "DisaggregatedStorage"
)

// IsDisagg returns true if this is the disaggregated storage type.
func (st StorageType) IsDisagg() bool {
	return st == DisaggregatedStorage
}

// Suffix returns the storage engine's suffix in ServerVersion strings, e.g. the "dsc" in
// "9.1-dsc". It returns an empty string for attached storage, which has no suffix.
func (st StorageType) Suffix() string {
	if st == DisaggregatedStorage {
		return "dsc"
	}
	return ""
}

// String returns the name of the storage type, e.g. "AttachedStorage".
func (st StorageType) String() string {
	return string(st)
}
