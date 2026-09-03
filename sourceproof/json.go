package sourceproof

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

type proofEnum interface {
	State | EvidenceKind
	Validate() error
	String() string
}

func marshalProofEnum[T proofEnum](value T) ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(value.String())
}

func (s State) MarshalJSON() ([]byte, error)        { return marshalProofEnum(s) }
func (k EvidenceKind) MarshalJSON() ([]byte, error) { return marshalProofEnum(k) }

func (s *State) UnmarshalJSON(data []byte) error {
	return unmarshalProofEnum(data, s, [...]State{StateProven, stateLimit})
}

func (k *EvidenceKind) UnmarshalJSON(data []byte) error {
	return unmarshalProofEnum(data, k, [...]EvidenceKind{EvidenceSourceObservation, evidenceKindLimit})
}

func unmarshalProofEnum[T proofEnum](data []byte, destination *T, bounds [2]T) error {
	if destination == nil {
		return errors.Join(core.ErrJSONContract, contractError(errors.New("source proof enum receiver is nil")))
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(core.ErrJSONContract, contractError(err))
	}
	for candidate := bounds[0]; candidate < bounds[1]; candidate++ {
		if candidate.String() == value {
			*destination = candidate
			return nil
		}
	}
	return errors.Join(core.ErrJSONContract, contractError(errors.New("source proof enum token is unsupported")))
}

type resultWire Result
type summaryWire Summary

func (r Result) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONDocument(resultWire(r))
}

func (r *Result) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.Join(core.ErrJSONContract, contractError(errors.New("source proof receiver is nil")))
	}
	wire, err := core.DecodeStrictJSONStructure[resultWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrJSONContract, core.ErrSourceProofContract, err)
	}
	candidate := Result(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*r = candidate
	return nil
}

// MarshalJSON emits one validated canonical proof-stream summary.
func (s Summary) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONDocument(summaryWire(s))
}

// UnmarshalJSON admits one strict proof-stream summary without mutating the
// receiver on rejection.
func (s *Summary) UnmarshalJSON(data []byte) error {
	if s == nil {
		return errors.Join(core.ErrJSONContract, contractError(errors.New("source proof summary receiver is nil")))
	}
	wire, err := core.DecodeStrictJSONStructure[summaryWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrJSONContract, core.ErrSourceProofContract, err)
	}
	candidate := Summary(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*s = candidate
	return nil
}

func (r Result) Digest() (core.SHA256Digest, error) {
	encoded, err := r.MarshalJSON()
	if err != nil {
		return core.SHA256Digest{}, err
	}
	return core.SHA256Of(encoded), nil
}
