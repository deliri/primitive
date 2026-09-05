package capabilities

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"slices"
)

// Classification preserves one catalog outcome independently of call syntax.
// Its zero value is invalid. Unresolved and contextual are explicit knowledge
// states and never carry invented effects.
type Classification struct {
	Operation   Operation
	Secondary   []Effect
	Disposition StandardSymbolDisposition
	Effect      Effect
}

func (c Classification) Validate() error {
	if err := errors.Join(c.Disposition.Validate(), c.Operation.Validate()); err != nil {
		return err
	}
	owner, err := c.Operation.effect()
	if err != nil {
		return err
	}
	if c.Operation != OperationUnavailable && (c.Disposition != StandardSymbolEffect || owner != c.Effect) {
		return contractError("replacement operation contradicts effect ownership")
	}
	if c.Disposition != StandardSymbolEffect {
		if c.Effect != EffectUnknown || len(c.Secondary) != 0 {
			return contractError("non-effect classification carries effect ownership")
		}
		return nil
	}
	if err := c.Effect.Validate(); err != nil {
		return err
	}
	return c.validateSecondary()
}

func (c Classification) validateSecondary() error {
	if len(c.Secondary) >= IdentityCount {
		return contractError("secondary effect count exceeds the effect domain")
	}
	for index, value := range c.Secondary {
		if err := value.Validate(); err != nil {
			return err
		}
		if value == c.Effect || slices.Contains(c.Secondary[:index], value) {
			return contractError("effect ownership is duplicated")
		}
	}
	return nil
}

const ClassificationJSONMaximumBytes = 1024

type classificationWire struct {
	Operation   Operation                 `json:"operation"`
	Effect      *Identity                 `json:"effect,omitempty"`
	Secondary   []Identity                `json:"secondary,omitempty"`
	Disposition StandardSymbolDisposition `json:"disposition"`
}

func (c Classification) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	wire := classificationWire{Disposition: c.Disposition, Operation: c.Operation}
	if c.Effect != EffectUnknown {
		identity, err := IdentityForEffect(c.Effect)
		if err != nil {
			return nil, err
		}
		wire.Effect = &identity
	}
	for _, effect := range c.Secondary {
		identity, err := IdentityForEffect(effect)
		if err != nil {
			return nil, err
		}
		wire.Secondary = append(wire.Secondary, identity)
	}
	return core.MarshalCanonicalJSONDocument(wire)
}

func (c *Classification) UnmarshalJSON(data []byte) error {
	if c == nil {
		return contractError("classification receiver is nil")
	}
	if len(data) > ClassificationJSONMaximumBytes {
		return contractError("classification exceeds byte ceiling")
	}
	limits := core.DefaultStrictJSONLimits()
	limits.ArrayItemMaximum = uint32(IdentityCount)
	wire, err := core.DecodeStrictJSONStructure[classificationWire](data, limits)
	if err != nil {
		return errors.Join(core.ErrCapabilitiesContract, err)
	}
	candidate := Classification{Disposition: wire.Disposition, Operation: wire.Operation}
	if wire.Effect != nil {
		candidate.Effect, err = wire.Effect.Effect()
		if err != nil {
			return err
		}
	}
	for _, identity := range wire.Secondary {
		effect, effectErr := identity.Effect()
		if effectErr != nil {
			return effectErr
		}
		candidate.Secondary = append(candidate.Secondary, effect)
	}
	if err := candidate.Validate(); err != nil {
		return err
	}
	*c = candidate
	return nil
}

func (d StandardSymbolDisposition) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return core.MarshalCanonicalJSONString(d.String())
}

func (d *StandardSymbolDisposition) UnmarshalJSON(data []byte) error {
	if d == nil {
		return contractError("symbol disposition receiver is nil")
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(core.ErrCapabilitiesContract, err)
	}
	for candidate := StandardSymbolPure; candidate <= StandardSymbolUnresolved; candidate++ {
		if value == candidate.String() {
			*d = candidate
			return nil
		}
	}
	return contractError("symbol disposition is outside the admitted domain")
}

// Equal compares the complete retained ownership contract.
func (c Classification) Equal(other Classification) bool {
	return c.Operation == other.Operation && c.Disposition == other.Disposition && c.Effect == other.Effect && slices.Equal(c.Secondary, other.Secondary)
}
func mergeClassification(result *Classification, candidate Classification) error {
	if err := candidate.Validate(); err != nil {
		return err
	}
	if candidate.Disposition == StandardSymbolUnresolved {
		return nil
	}
	if result.Disposition != StandardSymbolUnresolved && !result.Equal(candidate) {
		return contractError("symbol has contradictory catalog rules")
	}
	*result = candidate
	return nil
}
