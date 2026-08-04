package release

import "github.com/deliri/primitive/v2026/core"

// decodeStructure applies one package-owned JSON admission contract to every
// Release wire structure. Core owns the global structural scanner bounds;
// Release narrows document extent and the largest compiler-owned array
// cardinality. Every owning array type applies its exact count separately.
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
		ArrayItemMaximum:     linkerAssignmentMaximumCount,
	})
	if err != nil {
		return zero, jsonError(err)
	}
	return value, nil
}

// decodeDependencyStructure applies the dependency document's own admission
// contract. The customer-visible dependency document is the one Release wire
// structure whose array cardinality is the module ceiling rather than the
// linker-assignment ceiling, so it declares both bounds separately instead of
// widening every other Release document.
func decodeDependencyStructure[T any](data []byte) (T, error) {
	var zero T
	maximum, err := core.NewByteCount(dependencyDocumentExtentMaximum)
	if err != nil {
		return zero, jsonError(err)
	}
	value, err := core.DecodeStrictJSONStructure[T](data, core.StrictJSONLimits{
		DocumentMaximumBytes: maximum,
		NestingDepthMaximum:  core.JSONNestingDepthMaximum,
		ObjectFieldMaximum:   core.JSONObjectFieldCountMaximum,
		ArrayItemMaximum:     BuildDependencyMaximumCount,
	})
	if err != nil {
		return zero, jsonError(err)
	}
	return value, nil
}
