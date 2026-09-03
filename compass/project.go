package compass

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
)

// ProjectNameMaximumBytes bounds one human-facing project identity.
const ProjectNameMaximumBytes = 128

// ProjectName is the human-facing identity of one source project.
type ProjectName struct {
	value string
}

// ParseProjectName admits one bounded project name.
func ParseProjectName(value string) (ProjectName, error) {
	name := ProjectName{value: value}
	if err := name.Validate(); err != nil {
		return ProjectName{}, err
	}
	return name, nil
}

// Validate rejects absent, padded, control-bearing, or oversized names.
func (n ProjectName) Validate() error {
	if n.value == "" || len(n.value) > ProjectNameMaximumBytes || !utf8.ValidString(n.value) {
		return contractError("project name is absent, oversized, or invalid UTF-8", nil)
	}
	if strings.TrimSpace(n.value) != n.value {
		return contractError("project name has surrounding whitespace", nil)
	}
	for _, current := range n.value {
		if unicode.IsControl(current) {
			return contractError("project name contains control text", nil)
		}
	}
	return nil
}

// String returns the admitted name, or empty text for an invalid value.
func (n ProjectName) String() string {
	if n.Validate() != nil {
		return ""
	}
	return n.value
}

// MarshalJSON emits the project name as one canonical JSON string.
func (n ProjectName) MarshalJSON() ([]byte, error) {
	if err := n.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(n.value)
}

// UnmarshalJSON admits a project name without mutating the receiver on refusal.
func (n *ProjectName) UnmarshalJSON(data []byte) error {
	if n == nil {
		return errors.Join(core.ErrJSONContract, contractError("project name receiver is nil", nil))
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(core.ErrCompassContract, err)
	}
	parsed, err := ParseProjectName(value)
	if err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*n = parsed
	return nil
}

// ReleaseCoordinates are the human-authored release values admitted by
// Compass. Version is their only projection into display text or a Git tag.
type ReleaseCoordinates struct {
	Major uint32 `json:"major"`
	Minor uint32 `json:"minor"`
	Patch uint32 `json:"patch"`
}

// Validate rejects the zero declaration. Minor and patch zero remain valid.
func (c ReleaseCoordinates) Validate() error {
	if c.Major == 0 {
		return contractError("release major coordinate is zero", nil)
	}
	return nil
}

// Project is the universal declaration present in every project Compass.
type Project struct {
	Name       ProjectName             `json:"name"`
	Module     gomodule.Path           `json:"module"`
	Repository core.RepositoryIdentity `json:"repository"`
	Release    ReleaseCoordinates      `json:"release"`
}

// Validate proves every shared identity and release coordinate.
func (p Project) Validate() error {
	if err := errors.Join(p.Name.Validate(), p.Module.Validate(), p.Repository.Validate(), p.Release.Validate()); err != nil {
		return contractError("project declaration is invalid", err)
	}
	return nil
}

func contractError(message string, cause error) error {
	return errors.Join(core.ErrCompassContract, errors.New(message), cause)
}

var (
	_ core.ValidatedJSONMarshaler = ProjectName{}
	_ core.Validatable            = ReleaseCoordinates{}
	_ core.Validatable            = Project{}
)
