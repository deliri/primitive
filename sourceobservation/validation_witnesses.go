package sourceobservation

import "github.com/deliri/primitive/v2026/core"

var (
	_ core.Validatable = BuildContext{}
	_ core.Validatable = BuildSelection{}
	_ core.Validatable = Declaration{}
	_ core.Validatable = Import{}
	_ core.Validatable = Effect{}
	_ core.Validatable = Reference{}
	_ core.Validatable = FileReference{}
	_ core.Validatable = PackageReference{}
	_ core.Validatable = FileMembership{}
	_ core.Validatable = PackageMembership{}
	_ core.Validatable = File{}
	_ core.Validatable = Package{}
	_ core.Validatable = Project{}
	_ core.Validatable = Summary{}

	_ core.ValidatedJSONMarshaler = ContextID{}
	_ core.ValidatedJSONMarshaler = Language{}
	_ core.ValidatedJSONMarshaler = Symbol{}
	_ core.ValidatedJSONMarshaler = ImportPath{}
	_ core.ValidatedJSONMarshaler = EffectName{}
	_ core.ValidatedJSONMarshaler = Toolchain{}
	_ core.ValidatedJSONMarshaler = GeneratedState(0)
	_ core.ValidatedJSONMarshaler = SelectionState(0)
	_ core.ValidatedJSONMarshaler = DeclarationKind(0)
	_ core.ValidatedJSONMarshaler = ReferenceKind(0)
	_ core.ValidatedJSONMarshaler = FileReference{}
	_ core.ValidatedJSONMarshaler = PackageReference{}
	_ core.ValidatedJSONMarshaler = FileMembership{}
	_ core.ValidatedJSONMarshaler = PackageMembership{}
	_ core.ValidatedJSONMarshaler = File{}
	_ core.ValidatedJSONMarshaler = Package{}
	_ core.ValidatedJSONMarshaler = Project{}
	_ core.ValidatedJSONMarshaler = Summary{}
)
