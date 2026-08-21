package fuzzfinder

import (
	json "encoding/json/v2"

	"github.com/deliri/primitive/v2026/core"
)

var (
	_ core.ValidatedJSONMarshaler = ArtifactKind(0)

	_ core.Validatable = CacheFormatUnknown
	_ core.OffWireEnum = CacheFormatUnknown
	_ core.Validatable = RetentionLimit{}
	_ core.Validatable = FindRequest{}
	_ core.Validatable = ArtifactUnknown
	_ core.Validatable = GeneratedName{}
	_ core.Validatable = ObservationUnknown
	_ core.OffWireEnum = ObservationUnknown
	_ core.Validatable = Observation{}

	_ json.Marshaler   = ArtifactUnknown
	_ json.Unmarshaler = (*ArtifactKind)(nil)
)
