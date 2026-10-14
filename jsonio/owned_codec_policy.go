package jsonio

import "github.com/deliri/primitive/v2026/core"

// RootCodecPolicy separates declared struct admission from an explicit
// type-owned native JSON codec. Its zero arm preserves the strict default.
type RootCodecPolicy uint8

const (
	RootDeclaredFields RootCodecPolicy = iota
	RootOwnedMethods
)

func (p RootCodecPolicy) Validate() error {
	if p > RootOwnedMethods {
		return core.ErrJSONContract
	}
	return nil
}
func (p RootCodecPolicy) IsValid() bool { return p.Validate() == nil }
func (RootCodecPolicy) OffWireEnum()    {}
func (p RootCodecPolicy) String() string {
	switch p {
	case RootDeclaredFields:
		return "declared fields"
	case RootOwnedMethods:
		return "owned methods"
	default:
		return core.UnknownEnumDiagnostic
	}
}

// PointerNullPolicy controls decoding of declared pointer fields. Null roots
// and non-pointer fields remain refused in either arm. MarshalValidated also
// refuses emitted null under its strict arm; streaming writers leave emission
// policy to the validated document owner instead of inspecting a whole output.
type PointerNullPolicy uint8

const (
	PointerNullRefused PointerNullPolicy = iota
	PointerNullPermitted
)

func (p PointerNullPolicy) Validate() error {
	if p > PointerNullPermitted {
		return core.ErrJSONContract
	}
	return nil
}
func (p PointerNullPolicy) IsValid() bool { return p.Validate() == nil }
func (PointerNullPolicy) OffWireEnum()    {}
func (p PointerNullPolicy) String() string {
	switch p {
	case PointerNullRefused:
		return "pointer null refused"
	case PointerNullPermitted:
		return "pointer null permitted"
	default:
		return core.UnknownEnumDiagnostic
	}
}

// OwnedCodecPolicy admits one caller-owned whole value through Go. Default
// policy keeps unknown members, pointer nulls and opaque root codecs refused.
// RootOwnedMethods preserves native grammar checks before custom decoding, but
// field admission inside a custom method belongs to that method's typed owner;
// use a hook-free wire struct when Primitive must enforce every field policy.
type OwnedCodecPolicy struct {
	Members      MemberPolicy
	Root         RootCodecPolicy
	PointerNulls PointerNullPolicy
}

func (p OwnedCodecPolicy) Validate() error {
	if err := p.Members.Validate(); err != nil {
		return err
	}
	if err := p.Root.Validate(); err != nil {
		return err
	}
	return p.PointerNulls.Validate()
}

var (
	_ core.Validatable = OwnedCodecPolicy{}
	_ core.OffWireEnum = RootDeclaredFields
	_ core.OffWireEnum = PointerNullRefused
)
