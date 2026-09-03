package sourceobservation

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

type fileWire File
type packageWire Package
type projectWire Project
type fileMembershipWire FileMembership
type packageMembershipWire PackageMembership
type summaryWire Summary

func (r FileReference) MarshalJSON() ([]byte, error) {
	return marshalRecord(fileReferenceWire(r), r.Validate)
}

func (r *FileReference) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.Join(core.ErrJSONContract, contractError(errors.New("source file reference receiver is nil")))
	}
	wire, err := core.DecodeStrictJSONStructure[fileReferenceWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrJSONContract, core.ErrSourceObservationContract, err)
	}
	candidate := FileReference(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*r = candidate
	return nil
}

func (r PackageReference) MarshalJSON() ([]byte, error) {
	return marshalRecord(packageReferenceWire(r), r.Validate)
}

func (r *PackageReference) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.Join(core.ErrJSONContract, contractError(errors.New("source package reference receiver is nil")))
	}
	wire, err := core.DecodeStrictJSONStructure[packageReferenceWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrJSONContract, core.ErrSourceObservationContract, err)
	}
	candidate := PackageReference(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*r = candidate
	return nil
}

func (f File) MarshalJSON() ([]byte, error) {
	return marshalRecord(fileWire(f), f.Validate)
}

func (f *File) UnmarshalJSON(data []byte) error {
	if f == nil {
		return errors.Join(core.ErrJSONContract, contractError(errors.New("source file observation receiver is nil")))
	}
	wire, err := core.DecodeStrictJSONStructure[fileWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrJSONContract, core.ErrSourceObservationContract, err)
	}
	candidate := File(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*f = candidate
	return nil
}

func (p Package) MarshalJSON() ([]byte, error) {
	return marshalRecord(packageWire(p), p.Validate)
}

func (p *Package) UnmarshalJSON(data []byte) error {
	if p == nil {
		return errors.Join(core.ErrJSONContract, contractError(errors.New("source package observation receiver is nil")))
	}
	wire, err := core.DecodeStrictJSONStructure[packageWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrJSONContract, core.ErrSourceObservationContract, err)
	}
	candidate := Package(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*p = candidate
	return nil
}

func (p Project) MarshalJSON() ([]byte, error) {
	return marshalRecord(projectWire(p), p.Validate)
}

func (p *Project) UnmarshalJSON(data []byte) error {
	if p == nil {
		return errors.Join(core.ErrJSONContract, contractError(errors.New("source project observation receiver is nil")))
	}
	wire, err := core.DecodeStrictJSONStructure[projectWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrJSONContract, core.ErrSourceObservationContract, err)
	}
	candidate := Project(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*p = candidate
	return nil
}

// MarshalJSON emits one canonical file-membership index document.
func (m FileMembership) MarshalJSON() ([]byte, error) {
	return marshalRecord(fileMembershipWire(m), m.Validate)
}

// UnmarshalJSON admits one strict file-membership index without mutating the
// receiver on rejection.
func (m *FileMembership) UnmarshalJSON(data []byte) error {
	if m == nil {
		return errors.Join(core.ErrJSONContract, contractError(errors.New("source file membership receiver is nil")))
	}
	wire, err := core.DecodeStrictJSONStructure[fileMembershipWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrJSONContract, core.ErrSourceObservationContract, err)
	}
	candidate := FileMembership(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*m = candidate
	return nil
}

// MarshalJSON emits one canonical package-membership index document.
func (m PackageMembership) MarshalJSON() ([]byte, error) {
	return marshalRecord(packageMembershipWire(m), m.Validate)
}

// UnmarshalJSON admits one strict package-membership index without mutating
// the receiver on rejection.
func (m *PackageMembership) UnmarshalJSON(data []byte) error {
	if m == nil {
		return errors.Join(core.ErrJSONContract, contractError(errors.New("source package membership receiver is nil")))
	}
	wire, err := core.DecodeStrictJSONStructure[packageMembershipWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrJSONContract, core.ErrSourceObservationContract, err)
	}
	candidate := PackageMembership(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*m = candidate
	return nil
}

// MarshalJSON emits one validated canonical verified-project summary.
func (s Summary) MarshalJSON() ([]byte, error) {
	return marshalRecord(summaryWire(s), s.Validate)
}

// UnmarshalJSON admits one strict verified-project summary without mutating
// the receiver on rejection.
func (s *Summary) UnmarshalJSON(data []byte) error {
	if s == nil {
		return errors.Join(core.ErrJSONContract, contractError(errors.New("source observation summary receiver is nil")))
	}
	wire, err := core.DecodeStrictJSONStructure[summaryWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrJSONContract, core.ErrSourceObservationContract, err)
	}
	candidate := Summary(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*s = candidate
	return nil
}

func marshalRecord[T any](wire T, validate func() error) ([]byte, error) {
	if err := validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONDocument(wire)
}

func (f File) ObservationDigest() (core.SHA256Digest, error) {
	encoded, err := f.MarshalJSON()
	if err != nil {
		return core.SHA256Digest{}, err
	}
	return core.SHA256Of(encoded), nil
}

func (p Package) ObservationDigest() (core.SHA256Digest, error) {
	encoded, err := p.MarshalJSON()
	if err != nil {
		return core.SHA256Digest{}, err
	}
	return core.SHA256Of(encoded), nil
}

func (p Project) ObservationDigest() (core.SHA256Digest, error) {
	encoded, err := p.MarshalJSON()
	if err != nil {
		return core.SHA256Digest{}, err
	}
	return core.SHA256Of(encoded), nil
}
