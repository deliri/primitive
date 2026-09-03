package projectversion

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const gitTagPrefix = "v"

// Tag is the canonical Git tag derived from a project Release.
//
// Its representation is private so valid tags cannot disagree with the typed
// release coordinate that produced them.
type Tag struct {
	release Release
}

// ParseTag admits one canonical v-prefixed project release tag from an
// external boundary. Project-owned code should derive its own tag with
// Release.Tag instead of parsing a string it already controls.
func ParseTag(text string) (Tag, error) {
	if len(text) < 2 || text[0] != gitTagPrefix[0] {
		return Tag{}, errors.Join(core.ErrReleaseContract, errors.New("project release tag is not v-prefixed"))
	}
	var version core.ReleaseVersion
	if err := version.UnmarshalText([]byte(text[1:])); err != nil {
		return Tag{}, errors.Join(core.ErrReleaseContract, err)
	}
	tag := Tag{release: Release{version: version}}
	if err := tag.Validate(); err != nil {
		return Tag{}, err
	}
	return tag, nil
}

// Validate proves that the tag was derived from or decoded into a valid
// project release.
func (t Tag) Validate() error {
	if err := t.release.Validate(); err != nil {
		return errors.Join(core.ErrReleaseContract, err)
	}
	return nil
}

// Release returns the typed project release named by the tag.
func (t Tag) Release() Release {
	if t.Validate() != nil {
		return Release{}
	}
	return t.release
}

// String returns the exact v-prefixed Git tag, or empty text for an invalid
// zero value.
func (t Tag) String() string {
	if t.Validate() != nil {
		return ""
	}
	return gitTagPrefix + t.release.String()
}

// MarshalText implements encoding.TextMarshaler with the canonical Git tag.
func (t Tag) MarshalText() ([]byte, error) {
	if err := t.Validate(); err != nil {
		return nil, err
	}
	return []byte(t.String()), nil
}

// UnmarshalText accepts one canonical Git tag without mutating the receiver on
// rejection.
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

// MarshalJSON emits one canonical JSON string containing the Git tag.
func (t Tag) MarshalJSON() ([]byte, error) {
	if err := t.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(t.String())
}

// UnmarshalJSON accepts one canonical JSON string without mutating the
// receiver on rejection.
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

var _ core.Validatable = Tag{}
