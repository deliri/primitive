package fuzzfinder

import (
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const (
	artifactCorpusToken          = "fuzz-corpus"
	artifactCrasherToken         = "fuzz-crasher"
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

func artifactKindTokens() [artifactKindLimit]string {
	return [...]string{
		ArtifactCorpus:  artifactCorpusToken,
		ArtifactCrasher: artifactCrasherToken,
	}
}

func (k ArtifactKind) token() (string, error) {
	if k <= ArtifactUnknown || k >= artifactKindLimit {
		return "", contractError(errors.New("artifact kind is outside the closed domain"))
	}
	token := artifactKindTokens()[k]
	if token == "" {
		return "", contractError(errors.New("artifact kind has no contract"))
	}
	return token, nil
}

// Validate rejects values outside the corpus/crasher domain.
func (k ArtifactKind) Validate() error {
	_, err := k.token()
	return err
}

// IsValid reports membership in the corpus/crasher domain.
func (k ArtifactKind) IsValid() bool {
	return k.Validate() == nil
}

// String returns the exact wire token or a diagnostic unknown token.
func (k ArtifactKind) String() string {
	token, err := k.token()
	if err != nil {
		return core.UnknownEnumDiagnostic
	}
	return token
}

// ParseArtifactKind parses one exact wire token.
func ParseArtifactKind(token string) (ArtifactKind, error) {
	for kind := ArtifactUnknown + 1; kind < artifactKindLimit; kind++ {
		if kind.String() == token {
			return kind, nil
		}
	}
	return ArtifactUnknown, formatError(errors.New("artifact kind token is unsupported"))
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
