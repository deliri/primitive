package sourceclaim

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

// ID is one stable compiler-visible claim, boundary, or requirement identity.
type ID struct{ value string }

// Text is one human-authored statement. Primitive validates its canonical
// shape but does not invent a semantic size quota for a problem or reason.
type Text struct{ value string }

// Reference is one bounded compiler-facing declaration, import, effect, or
// build-context coordinate. Its interpretation is closed by a predicate.
type Reference struct{ value string }

func NewID(value string) (ID, error) {
	candidate := ID{value: value}
	if err := candidate.Validate(); err != nil {
		return ID{}, err
	}
	return candidate, nil
}

func NewText(value string) (Text, error) {
	candidate := Text{value: value}
	if err := candidate.Validate(); err != nil {
		return Text{}, err
	}
	return candidate, nil
}

func NewReference(value string) (Reference, error) {
	candidate := Reference{value: value}
	if err := candidate.Validate(); err != nil {
		return Reference{}, err
	}
	return candidate, nil
}

func (i ID) Validate() error {
	value := i.value
	if len(value) == 0 || !identifierEdge(value[0]) || !identifierEdge(value[len(value)-1]) {
		return contractError(errors.New("source claim identity is invalid"))
	}
	for index := 1; index < len(value)-1; index++ {
		if !identifierContinuation(value[index]) {
			return contractError(errors.New("source claim identity is not canonical"))
		}
	}
	return nil
}

func (t Text) Validate() error {
	value := t.value
	if len(value) == 0 || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return contractError(errors.New("source claim text is invalid"))
	}
	for _, current := range value {
		if unicode.IsControl(current) {
			return contractError(errors.New("source claim text contains control data"))
		}
	}
	return nil
}

func (r Reference) Validate() error {
	value := r.value
	if len(value) == 0 || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return contractError(errors.New("source claim reference is invalid"))
	}
	for _, current := range value {
		if unicode.IsControl(current) || unicode.IsSpace(current) {
			return contractError(errors.New("source claim reference contains whitespace or control data"))
		}
	}
	return nil
}

func identifierEdge(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}

func identifierContinuation(value byte) bool {
	return identifierEdge(value) || value == '-' || value == '_' || value == '.' || value == ':'
}

func (i ID) String() string        { return i.value }
func (t Text) String() string      { return t.value }
func (r Reference) String() string { return r.value }

func marshalScalar(value string, validate func() error) ([]byte, error) {
	if err := validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(value)
}

func (i ID) MarshalJSON() ([]byte, error)        { return marshalScalar(i.value, i.Validate) }
func (t Text) MarshalJSON() ([]byte, error)      { return marshalScalar(t.value, t.Validate) }
func (r Reference) MarshalJSON() ([]byte, error) { return marshalScalar(r.value, r.Validate) }

func (i *ID) UnmarshalJSON(data []byte) error {
	if i == nil {
		return errors.Join(core.ErrJSONContract, contractError(errors.New("source claim identity receiver is nil")))
	}
	return decodeScalar(data, func(value string) error {
		candidate, err := NewID(value)
		if err == nil {
			*i = candidate
		}
		return err
	})
}

func (t *Text) UnmarshalJSON(data []byte) error {
	if t == nil {
		return errors.Join(core.ErrJSONContract, contractError(errors.New("source claim text receiver is nil")))
	}
	return decodeScalar(data, func(value string) error {
		candidate, err := NewText(value)
		if err == nil {
			*t = candidate
		}
		return err
	})
}

func (r *Reference) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.Join(core.ErrJSONContract, contractError(errors.New("source claim reference receiver is nil")))
	}
	return decodeScalar(data, func(value string) error {
		candidate, err := NewReference(value)
		if err == nil {
			*r = candidate
		}
		return err
	})
}

func decodeScalar(data []byte, construct func(string) error) error {
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(core.ErrJSONContract, contractError(err))
	}
	if err := construct(value); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	return nil
}
