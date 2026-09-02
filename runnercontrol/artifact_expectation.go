package runnercontrol

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/standard"
)

// ArtifactExpectation is the compiler-owned description of one file an
// experiment may leave for its caller to retain. Path is rooted in the runner
// workspace, not interpreted relative to ambient process state.
type ArtifactExpectation struct {
	Path         standard.SourcePath `json:"path"`
	MediaType    core.HTTPMediaType  `json:"media_type"`
	MaximumBytes core.ByteCount      `json:"maximum_bytes"`
	Kind         ArtifactKind        `json:"kind"`
	Required     bool                `json:"required"`
}

func (e ArtifactExpectation) Validate() error {
	return errors.Join(e.Kind.Validate(), e.Path.Validate(), e.MediaType.Validate(), e.MaximumBytes.Validate())
}

func validateArtifactExpectations(values []ArtifactExpectation) error {
	if len(values) > ArtifactManifestMaximumEntries {
		return core.ErrPrimitiveContract
	}
	previous := ""
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return err
		}
		path := values[index].Path.String()
		if index > 0 && previous >= path {
			return core.ErrPrimitiveContract
		}
		previous = path
	}
	return nil
}

var _ core.Validatable = ArtifactExpectation{}
