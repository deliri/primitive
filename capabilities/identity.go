package capabilities

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// IdentityCount is the exact number of compiler-owned real-world capability
// identities. Bounds in consuming contracts derive from this authority.
const IdentityCount = int(effectLimit - 1)

// Identity is the stable wire-safe identity of one real-world capability.
// Effect remains the execution taxonomy; Identity is its validated authored
// policy representation.
type Identity struct {
	effect Effect
}

// IdentityForEffect converts one compiler-owned effect into its authored
// capability identity.
func IdentityForEffect(effect Effect) (Identity, error) {
	identity := Identity{effect: effect}
	if err := identity.Validate(); err != nil {
		return Identity{}, err
	}
	return identity, nil
}

// ParseIdentity admits the stable doctrine name of one capability.
func ParseIdentity(value string) (Identity, error) {
	for effect := EffectFilesystem; effect < effectLimit; effect++ {
		if effect.String() == value {
			return IdentityForEffect(effect)
		}
	}
	return Identity{}, contractError("capability identity is outside the admitted domain")
}

func (i Identity) Validate() error { return i.effect.Validate() }

func (i Identity) IsValid() bool { return i.Validate() == nil }

func (i Identity) String() string {
	if !i.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return i.effect.String()
}

// Effect returns the execution taxonomy value represented by the identity.
func (i Identity) Effect() (Effect, error) {
	if err := i.Validate(); err != nil {
		return EffectUnknown, err
	}
	return i.effect, nil
}

func (i Identity) MarshalJSON() ([]byte, error) {
	if err := i.Validate(); err != nil {
		return nil, err
	}
	encoded, err := core.MarshalCanonicalJSONString(i.String())
	if err != nil {
		return nil, errors.Join(core.ErrCapabilitiesContract, err)
	}
	return encoded, nil
}

func (i *Identity) UnmarshalJSON(data []byte) error {
	if i == nil {
		return contractError("capability identity receiver is nil")
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(core.ErrCapabilitiesContract, err)
	}
	candidate, err := ParseIdentity(value)
	if err != nil {
		return err
	}
	*i = candidate
	return nil
}

var _ core.ValidatedJSONMarshaler = Identity{}
