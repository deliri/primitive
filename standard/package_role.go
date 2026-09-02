package standard

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const PackageRoleDeclarationMaximumBytes = SourcePathMaximumBytes + 128

// PackageRoleDeclaration is the minimal hand-authored package contract an
// observer may read before the package's complete explanatory knowledge is ready.
// The same declaration supplies PackageKnowledge.AuthorRole; generated source
// facts never infer or replace it.
type PackageRoleDeclaration struct {
	Path SourcePath       `json:"path"`
	Role core.PackageRole `json:"role"`
}

func (d PackageRoleDeclaration) Validate() error {
	return contractJoin(d.Path.Validate(), d.Role.Validate())
}

type packageRoleDeclarationWire PackageRoleDeclaration

func (d PackageRoleDeclaration) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(packageRoleDeclarationWire(d))
	if err != nil {
		return nil, jsonError(err)
	}
	if len(encoded) > PackageRoleDeclarationMaximumBytes {
		return nil, jsonError(core.ErrStandardContract)
	}
	return encoded, nil
}

func (d *PackageRoleDeclaration) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("nil standard package role declaration receiver"))
	}
	limits, err := packageRoleDeclarationJSONLimits()
	if err != nil {
		return jsonError(err)
	}
	wire, err := core.DecodeStrictJSONStructure[packageRoleDeclarationWire](data, limits)
	if err != nil {
		return jsonError(err)
	}
	candidate := PackageRoleDeclaration(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

func packageRoleDeclarationJSONLimits() (core.StrictJSONLimits, error) {
	maximum, err := core.NewByteCount(PackageRoleDeclarationMaximumBytes)
	if err != nil {
		return core.StrictJSONLimits{}, err
	}
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = maximum
	return limits, nil
}
