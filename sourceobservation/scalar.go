package sourceobservation

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

type ContextID struct{ value string }
type Language struct{ value string }
type Symbol struct{ value string }
type ImportPath struct{ value string }
type EffectName struct{ value string }
type Toolchain struct{ value string }

func NewContextID(value string) (ContextID, error) {
	return constructScalar[ContextID](value, func(value string) ContextID { return ContextID{value: value} })
}
func NewLanguage(value string) (Language, error) {
	return constructScalar[Language](value, func(value string) Language { return Language{value: value} })
}
func NewSymbol(value string) (Symbol, error) {
	return constructScalar[Symbol](value, func(value string) Symbol { return Symbol{value: value} })
}
func NewImportPath(value string) (ImportPath, error) {
	return constructScalar[ImportPath](value, func(value string) ImportPath { return ImportPath{value: value} })
}
func NewEffectName(value string) (EffectName, error) {
	return constructScalar[EffectName](value, func(value string) EffectName { return EffectName{value: value} })
}
func NewToolchain(value string) (Toolchain, error) {
	return constructScalar[Toolchain](value, func(value string) Toolchain { return Toolchain{value: value} })
}

type scalarContract interface {
	ContextID | Language | Symbol | ImportPath | EffectName | Toolchain
	Validate() error
}

func constructScalar[T scalarContract](value string, construct func(string) T) (T, error) {
	candidate := construct(value)
	if err := candidate.Validate(); err != nil {
		var zero T
		return zero, err
	}
	return candidate, nil
}

func validateScalar(value, name string) error {
	if value == "" || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return contractError(errors.New("source observation " + name + " is invalid"))
	}
	for _, current := range value {
		if unicode.IsControl(current) || unicode.IsSpace(current) {
			return contractError(errors.New("source observation " + name + " contains whitespace or control data"))
		}
	}
	return nil
}

func (v ContextID) Validate() error  { return validateScalar(v.value, "build context identity") }
func (v Language) Validate() error   { return validateScalar(v.value, "language") }
func (v Symbol) Validate() error     { return validateScalar(v.value, "symbol") }
func (v ImportPath) Validate() error { return validateScalar(v.value, "import path") }
func (v EffectName) Validate() error { return validateScalar(v.value, "effect name") }
func (v Toolchain) Validate() error  { return validateScalar(v.value, "toolchain") }

func (v ContextID) String() string  { return v.value }
func (v Language) String() string   { return v.value }
func (v Symbol) String() string     { return v.value }
func (v ImportPath) String() string { return v.value }
func (v EffectName) String() string { return v.value }
func (v Toolchain) String() string  { return v.value }

func marshalScalar(value string, validate func() error) ([]byte, error) {
	if err := validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(value)
}

func (v ContextID) MarshalJSON() ([]byte, error)  { return marshalScalar(v.value, v.Validate) }
func (v Language) MarshalJSON() ([]byte, error)   { return marshalScalar(v.value, v.Validate) }
func (v Symbol) MarshalJSON() ([]byte, error)     { return marshalScalar(v.value, v.Validate) }
func (v ImportPath) MarshalJSON() ([]byte, error) { return marshalScalar(v.value, v.Validate) }
func (v EffectName) MarshalJSON() ([]byte, error) { return marshalScalar(v.value, v.Validate) }
func (v Toolchain) MarshalJSON() ([]byte, error)  { return marshalScalar(v.value, v.Validate) }

func decodeScalar[T scalarContract](data []byte, construct func(string) (T, error)) (T, error) {
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		var zero T
		return zero, errors.Join(core.ErrJSONContract, contractError(err))
	}
	candidate, err := construct(value)
	if err != nil {
		var zero T
		return zero, errors.Join(core.ErrJSONContract, err)
	}
	return candidate, nil
}

func decodeInto[T scalarContract](destination *T, data []byte, construct func(string) (T, error)) error {
	if destination == nil {
		return errors.Join(core.ErrJSONContract, contractError(errors.New("source observation scalar receiver is nil")))
	}
	candidate, err := decodeScalar(data, construct)
	if err == nil {
		*destination = candidate
	}
	return err
}

func (v *ContextID) UnmarshalJSON(data []byte) error {
	return decodeInto(v, data, NewContextID)
}
func (v *Language) UnmarshalJSON(data []byte) error {
	return decodeInto(v, data, NewLanguage)
}
func (v *Symbol) UnmarshalJSON(data []byte) error {
	return decodeInto(v, data, NewSymbol)
}
func (v *ImportPath) UnmarshalJSON(data []byte) error {
	return decodeInto(v, data, NewImportPath)
}
func (v *EffectName) UnmarshalJSON(data []byte) error {
	return decodeInto(v, data, NewEffectName)
}
func (v *Toolchain) UnmarshalJSON(data []byte) error {
	return decodeInto(v, data, NewToolchain)
}
