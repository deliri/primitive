package sourceobservation

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

type GeneratedState uint8

const (
	GeneratedUnknown GeneratedState = iota
	GeneratedAuthored
	GeneratedProduced
	generatedStateLimit
)

func (s GeneratedState) Validate() error {
	if !s.IsValid() {
		return contractError(errors.New("source observation generated state is invalid"))
	}
	return nil
}

func (s GeneratedState) IsValid() bool {
	return s > GeneratedUnknown && s < generatedStateLimit && generatedStateTokens()[s] != ""
}

func generatedStateTokens() [generatedStateLimit]string {
	return [...]string{"", "authored", "generated"}
}

func (s GeneratedState) String() string {
	if s >= generatedStateLimit {
		return ""
	}
	return generatedStateTokens()[s]
}

type SelectionState uint8

const (
	SelectionUnknown SelectionState = iota
	SelectionIncluded
	SelectionExcluded
	selectionStateLimit
)

func (s SelectionState) Validate() error {
	if !s.IsValid() {
		return contractError(errors.New("source observation build selection is invalid"))
	}
	return nil
}

func (s SelectionState) IsValid() bool {
	return s > SelectionUnknown && s < selectionStateLimit && selectionStateTokens()[s] != ""
}

func selectionStateTokens() [selectionStateLimit]string {
	return [...]string{"", "included", "excluded"}
}

func (s SelectionState) String() string {
	if s >= selectionStateLimit {
		return ""
	}
	return selectionStateTokens()[s]
}

type DeclarationKind uint8

const (
	DeclarationUnknown DeclarationKind = iota
	DeclarationConstant
	DeclarationVariable
	DeclarationType
	DeclarationFunction
	DeclarationMethod
	DeclarationTest
	DeclarationBenchmark
	DeclarationFuzzTarget
	declarationKindLimit
)

func (k DeclarationKind) Validate() error {
	if !k.IsValid() {
		return contractError(errors.New("source observation declaration kind is invalid"))
	}
	return nil
}

func (k DeclarationKind) IsValid() bool {
	return k > DeclarationUnknown && k < declarationKindLimit && declarationKindTokens()[k] != ""
}

func declarationKindTokens() [declarationKindLimit]string {
	return [...]string{"", "constant", "variable", "type", "function", "method", "test", "benchmark", "fuzz_target"}
}

func (k DeclarationKind) String() string {
	if k >= declarationKindLimit {
		return ""
	}
	return declarationKindTokens()[k]
}

type ReferenceKind uint8

const (
	ReferenceUnknown ReferenceKind = iota
	ReferencePackage
	ReferenceExternal
	ReferenceDynamic
	referenceKindLimit
)

func (k ReferenceKind) Validate() error {
	if !k.IsValid() {
		return contractError(errors.New("source observation reference kind is invalid"))
	}
	return nil
}

func (k ReferenceKind) IsValid() bool {
	return k > ReferenceUnknown && k < referenceKindLimit && referenceKindTokens()[k] != ""
}

func referenceKindTokens() [referenceKindLimit]string {
	return [...]string{"", "package", "external", "dynamic"}
}

func (k ReferenceKind) String() string {
	if k >= referenceKindLimit {
		return ""
	}
	return referenceKindTokens()[k]
}

type sourceEnum interface {
	GeneratedState | SelectionState | DeclarationKind | ReferenceKind
	Validate() error
	String() string
}

func marshalSourceEnum[T sourceEnum](value T) ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(value.String())
}

func (s GeneratedState) MarshalJSON() ([]byte, error)  { return marshalSourceEnum(s) }
func (s SelectionState) MarshalJSON() ([]byte, error)  { return marshalSourceEnum(s) }
func (k DeclarationKind) MarshalJSON() ([]byte, error) { return marshalSourceEnum(k) }
func (k ReferenceKind) MarshalJSON() ([]byte, error)   { return marshalSourceEnum(k) }

func (s *GeneratedState) UnmarshalJSON(data []byte) error {
	return unmarshalSourceEnum(data, s, [...]GeneratedState{GeneratedAuthored, generatedStateLimit})
}
func (s *SelectionState) UnmarshalJSON(data []byte) error {
	return unmarshalSourceEnum(data, s, [...]SelectionState{SelectionIncluded, selectionStateLimit})
}
func (k *DeclarationKind) UnmarshalJSON(data []byte) error {
	return unmarshalSourceEnum(data, k, [...]DeclarationKind{DeclarationConstant, declarationKindLimit})
}
func (k *ReferenceKind) UnmarshalJSON(data []byte) error {
	return unmarshalSourceEnum(data, k, [...]ReferenceKind{ReferencePackage, referenceKindLimit})
}

func unmarshalSourceEnum[T sourceEnum](data []byte, destination *T, bounds [2]T) error {
	if destination == nil {
		return errors.Join(core.ErrJSONContract, contractError(errors.New("source observation enum receiver is nil")))
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
	return errors.Join(core.ErrJSONContract, contractError(errors.New("source observation enum token is unsupported")))
}
