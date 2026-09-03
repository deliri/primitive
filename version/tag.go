package version

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const gitTagPrefix = "v"

// Tag is the canonical Git tag derived from a Release.
type Tag struct {
	release Release
}

// ParseTag admits one canonical v-prefixed release tag observed at an external
// boundary. A project's own tag is derived from Release.Tag instead.
func ParseTag(text string) (Tag, error) {
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
	text, err := core.DecodeJSONStringToken(data)
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
