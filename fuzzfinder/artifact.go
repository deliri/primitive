package fuzzfinder

import (
	"encoding/json"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const (
	artifactCorpusToken          = "fuzz-corpus"
	artifactCrasherToken         = "fuzz-crasher"
	artifactUnknownToken         = "unknown"
	artifactKindJSONMaximumBytes = 64
)

// ArtifactKind is the closed wire identity of a corpus or crasher artifact.
type ArtifactKind uint8

const (
	ArtifactUnknown ArtifactKind = iota
	ArtifactCorpus
	ArtifactCrasher
	artifactKindLimit
)

// Validate rejects values outside the corpus/crasher domain.
func (k ArtifactKind) Validate() error {
	if k <= ArtifactUnknown || k >= artifactKindLimit {
		return contractError(errors.New("artifact kind is outside the closed domain"))
	}
	return nil
}

// IsValid reports membership in the corpus/crasher domain.
func (k ArtifactKind) IsValid() bool {
	return k > ArtifactUnknown && k < artifactKindLimit
}

// String returns the exact wire token or a diagnostic unknown token.
func (k ArtifactKind) String() string {
	switch k {
	case ArtifactCorpus:
		return artifactCorpusToken
	case ArtifactCrasher:
		return artifactCrasherToken
	default:
		return artifactUnknownToken
	}
}

// ParseArtifactKind parses one exact wire token.
func ParseArtifactKind(token string) (ArtifactKind, error) {
	switch token {
	case artifactCorpusToken:
		return ArtifactCorpus, nil
	case artifactCrasherToken:
		return ArtifactCrasher, nil
	default:
		return ArtifactUnknown, formatError(errors.New("artifact kind token is unsupported"))
	}
}

// MarshalJSON emits the exact artifact-kind token.
func (k ArtifactKind) MarshalJSON() ([]byte, error) {
	if err := k.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(k.String())
}

// UnmarshalJSON admits one bounded artifact-kind string and preserves the
// receiver on rejection.
func (k *ArtifactKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return contractError(errors.New("artifact kind receiver is nil"))
	}
	if len(data) == 0 || len(data) > artifactKindJSONMaximumBytes {
		return contractError(errors.Join(core.ErrJSONContract, errors.New("artifact kind JSON extent is invalid")))
	}
	// Admission is core's rule, not this package's. Restating it here as a quote
	// scan plus a bare json.Unmarshal accepted documents that every other typed
	// enum in the repository refuses, and silently replaced unpaired surrogates
	// before the domain parse could see them.
	token, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return contractError(err)
	}
	parsed, err := ParseArtifactKind(token)
	if err != nil {
		return err
	}
	*k = parsed
	return nil
}
