package sourceclaim

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func marshalEnum(value string, validate func() error) ([]byte, error) {
	if err := validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(value)
}

func (m RequirementMode) MarshalJSON() ([]byte, error) { return marshalEnum(m.String(), m.Validate) }

func (m *RequirementMode) UnmarshalJSON(data []byte) error {
	if m == nil {
		return errors.Join(core.ErrJSONContract, contractError(errors.New("source claim requirement mode receiver is nil")))
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(core.ErrJSONContract, contractError(err))
	}
	for candidate := RequirementCompiler; candidate < requirementModeLimit; candidate++ {
		if candidate.String() == value {
			*m = candidate
			return nil
		}
	}
	return errors.Join(core.ErrJSONContract, contractError(errors.New("source claim requirement mode token is unsupported")))
}

func executionKindTokens() [executionKindLimit]string {
	return [...]string{"", "test", "race", "benchmark", "fuzz", "tool"}
}

func (k ExecutionKind) String() string {
	if k >= executionKindLimit {
		return ""
	}
	return executionKindTokens()[k]
}

func (k ExecutionKind) MarshalJSON() ([]byte, error) { return marshalEnum(k.String(), k.Validate) }

func (k *ExecutionKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return errors.Join(core.ErrJSONContract, contractError(errors.New("source claim execution kind receiver is nil")))
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(core.ErrJSONContract, contractError(err))
	}
	for candidate := ExecutionTest; candidate < executionKindLimit; candidate++ {
		if candidate.String() == value {
			*k = candidate
			return nil
		}
	}
	return errors.Join(core.ErrJSONContract, contractError(errors.New("source claim execution kind token is unsupported")))
}

func (p CompilerPredicate) MarshalJSON() ([]byte, error) { return marshalEnum(p.String(), p.Validate) }

func (p *CompilerPredicate) UnmarshalJSON(data []byte) error {
	if p == nil {
		return errors.Join(core.ErrJSONContract, contractError(errors.New("source claim compiler predicate receiver is nil")))
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(core.ErrJSONContract, contractError(err))
	}
	for candidate := CompilerSubjectPresent; candidate < compilerPredicateLimit; candidate++ {
		if candidate.String() == value {
			*p = candidate
			return nil
		}
	}
	return errors.Join(core.ErrJSONContract, contractError(errors.New("source claim compiler predicate token is unsupported")))
}

type claimWire Claim
type summaryWire Summary

// MarshalJSON emits one validated canonical atomic claim.
func (c Claim) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONDocument(claimWire(c))
}

// UnmarshalJSON admits only one strict validated atomic claim and preserves
// the receiver on rejection.
func (c *Claim) UnmarshalJSON(data []byte) error {
	if c == nil {
		return errors.Join(core.ErrJSONContract, contractError(errors.New("source claim receiver is nil")))
	}
	wire, err := core.DecodeStrictJSONStructure[claimWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrJSONContract, core.ErrSourceClaimContract, err)
	}
	candidate := Claim(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*c = candidate
	return nil
}

// MarshalJSON emits one validated canonical claim-stream summary.
func (s Summary) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONDocument(summaryWire(s))
}

// UnmarshalJSON admits one strict claim-stream summary without mutating the
// receiver on rejection.
func (s *Summary) UnmarshalJSON(data []byte) error {
	if s == nil {
		return errors.Join(core.ErrJSONContract, contractError(errors.New("source claim summary receiver is nil")))
	}
	wire, err := core.DecodeStrictJSONStructure[summaryWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrJSONContract, core.ErrSourceClaimContract, err)
	}
	candidate := Summary(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*s = candidate
	return nil
}

// Digest returns the digest of the canonical claim bytes used by sourceproof.
func (c Claim) Digest() (core.SHA256Digest, error) {
	encoded, err := c.MarshalJSON()
	if err != nil {
		return core.SHA256Digest{}, err
	}
	return core.SHA256Of(encoded), nil
}
