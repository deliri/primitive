package version

import (
	"bytes"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const gitTagPrefix = "v"

// TagTextMaximumBytes follows from the prefix, three uint32 decimal
// coordinates (at most ten digits each), and two separators.
const TagTextMaximumBytes = len(gitTagPrefix) + 3*10 + 2

// A tag is ASCII. Each byte can occupy at most six bytes as a JSON Unicode
// escape; the two quotes are framing. Whitespace is not part of this bound.
const tagJSONTokenMaximumBytes = 2 + 6*TagTextMaximumBytes

// Tag is the canonical Git tag derived from a Release.
type Tag struct {
	release Release
}

// ParseTag admits one canonical v-prefixed release tag observed at an external
// boundary. A project's own tag is derived from Release.Tag instead.
func ParseTag(text string) (Tag, error) {
	if len(text) > TagTextMaximumBytes {
		return Tag{}, errors.Join(core.ErrReleaseContract, errors.New("project release tag exceeds coordinate representation"))
	}
	if len(text) < 2 || text[0] != gitTagPrefix[0] {
		return Tag{}, errors.Join(core.ErrReleaseContract, errors.New("project release tag is not v-prefixed"))
	}
	var coordinates core.ReleaseVersion
	if err := coordinates.UnmarshalText([]byte(text[1:])); err != nil {
		return Tag{}, errors.Join(core.ErrReleaseContract, err)
	}
	tag := Tag{release: Release{version: coordinates}}
	if err := tag.Validate(); err != nil {
		return Tag{}, err
	}
	return tag, nil
}

// Validate proves that the tag names a valid release.
func (t Tag) Validate() error {
	if err := t.release.Validate(); err != nil {
		return errors.Join(core.ErrReleaseContract, err)
	}
	return nil
}

// Release returns the typed release named by the tag.
func (t Tag) Release() Release {
	if t.Validate() != nil {
		return Release{}
	}
	return t.release
}

// String returns the exact v-prefixed Git tag.
func (t Tag) String() string {
	if t.Validate() != nil {
		return ""
	}
	return gitTagPrefix + t.release.String()
}

func (t Tag) MarshalText() ([]byte, error) {
	if err := t.Validate(); err != nil {
		return nil, err
	}
	return []byte(t.String()), nil
}

func (t *Tag) UnmarshalText(text []byte) error {
	if t == nil {
		return errors.Join(core.ErrReleaseContract, errors.New("project release tag receiver is nil"))
	}
	if len(text) > TagTextMaximumBytes {
		return errors.Join(core.ErrReleaseContract, errors.New("project release tag exceeds coordinate representation"))
	}
	parsed, err := ParseTag(string(text))
	if err != nil {
		return err
	}
	*t = parsed
	return nil
}

func (t Tag) MarshalJSON() ([]byte, error) {
	if err := t.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(t.String())
}

func (t *Tag) UnmarshalJSON(data []byte) error {
	if t == nil {
		return errors.Join(core.ErrJSONContract, core.ErrReleaseContract, errors.New("project release tag receiver is nil"))
	}
	token := bytes.Trim(data, " \t\r\n")
	if len(token) > tagJSONTokenMaximumBytes {
		return errors.Join(core.ErrJSONContract, core.ErrReleaseContract, errors.New("project release tag token exceeds coordinate representation"))
	}
	text, err := core.DecodeJSONStringToken(token)
	if err != nil {
		return errors.Join(core.ErrReleaseContract, err)
	}
	parsed, err := ParseTag(text)
	if err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*t = parsed
	return nil
}

var (
	_ core.Validatable            = Tag{}
	_ core.ValidatedJSONMarshaler = Tag{}
)
