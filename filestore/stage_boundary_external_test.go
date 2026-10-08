package filestore_test

// Fixture decisions use typed boundary kinds; human-readable subtest names
// never select the execution or refusal being tested.
type stageBoundaryKind uint8

const (
	stageBoundaryUnknown stageBoundaryKind = iota
	stageBoundaryNilCustody
	stageBoundaryZeroCustody
	stageBoundaryCopiedCustody
	stageBoundarySettledCustody
	stageBoundaryClosedNativeFile
	stageBoundaryNilContext
	stageBoundaryCanceledContext
	stageBoundaryExpiredContext
	stageBoundaryExtendingPrefix
)
