package release

import "github.com/deliri/primitive/v2026/core"

// decodeStructure applies one package-owned JSON admission contract to every
// Release wire structure. Core owns the global structural scanner bounds;
// Release narrows only document extent and the sole array cardinality.
func decodeStructure[T any](data []byte) (T, error) {
	var zero T
	maximum, err := core.NewByteCount(documentExtentMaximum)
	if err != nil {
		return zero, jsonError(err)
	}
	value, err := core.DecodeStrictJSONStructure[T](data, core.StrictJSONLimits{
		DocumentMaximumBytes: maximum,
		NestingDepthMaximum:  core.JSONNestingDepthMaximum,
		ObjectFieldMaximum:   core.JSONObjectFieldCountMaximum,
		ArrayItemMaximum:     TargetCount,
	})
	if err != nil {
		return zero, jsonError(err)
	}
	return value, nil
}
