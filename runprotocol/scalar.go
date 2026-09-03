package runprotocol

import (
	json "encoding/json/v2"
	"errors"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

const (
	IdentifierMaximumBytes         = 128
	NameMaximumBytes               = 160
	TextMaximumBytes               = 4096
	SourcePathMaximumBytes         = 1024
	RepositoryIdentityMaximumBytes = 512
)

type Identifier struct{ value string }
type Name struct{ value string }
type Text struct{ value string }
type SourcePath struct{ value string }
type RepositoryIdentity struct{ value string }

func NewIdentifier(value string) (Identifier, error) {
	candidate := Identifier{value: value}
	if err := candidate.Validate(); err != nil {
		return Identifier{}, err
	}
	return candidate, nil
}

func NewName(value string) (Name, error) {
	candidate := Name{value: value}
	if err := candidate.Validate(); err != nil {
		return Name{}, err
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

func ParseSourcePath(value string) (SourcePath, error) {
	candidate := SourcePath{value: value}
	if err := candidate.Validate(); err != nil {
		return SourcePath{}, err
	}
	return candidate, nil
}

func NewRepositoryIdentity(value string) (RepositoryIdentity, error) {
	candidate := RepositoryIdentity{value: value}
	if err := candidate.Validate(); err != nil {
		return RepositoryIdentity{}, err
	}
	return candidate, nil
}

func (v Identifier) Validate() error {
	if !validIdentifier(v.value, IdentifierMaximumBytes) {
		return contractError(errors.New("run protocol identifier is invalid"))
	}
	return nil
}

func (v Name) Validate() error {
	return validateHumanText(v.value, NameMaximumBytes, "run protocol name is invalid")
}

func (v Text) Validate() error {
	return validateHumanText(v.value, TextMaximumBytes, "run protocol text is invalid")
}

func (v SourcePath) Validate() error {
	if !validSourcePath(v.value) {
		return contractError(errors.New("run protocol source path is invalid"))
	}
	return nil
}

func (v RepositoryIdentity) Validate() error {
	if !validOpaqueToken(v.value, RepositoryIdentityMaximumBytes) {
		return contractError(errors.New("run protocol repository identity is invalid"))
	}
	return nil
}

func validIdentifier(value string, maximum int) bool {
	if !validOpaqueToken(value, maximum) || !identifierEdge(value[0]) {
		return false
	}
	for index := 1; index < len(value); index++ {
		if !identifierContinuation(value[index]) {
			return false
		}
	}
	return identifierEdge(value[len(value)-1])
}

func identifierEdge(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}

func identifierContinuation(value byte) bool {
	return identifierEdge(value) || value == '-' || value == '_' || value == '.' || value == ':'
}

func validOpaqueToken(value string, maximum int) bool {
	if len(value) == 0 || len(value) > maximum || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, current := range value {
		if unicode.IsControl(current) || unicode.IsSpace(current) {
			return false
		}
	}
	return true
}

func validateHumanText(value string, maximum int, diagnostic string) error {
	if len(value) == 0 || len(value) > maximum || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return contractError(errors.New(diagnostic))
	}
	for _, current := range value {
		if unicode.IsControl(current) {
			return contractError(errors.New(diagnostic))
		}
	}
	return nil
}

func validSourcePath(value string) bool {
	if !validSourcePathText(value) {
		return false
	}
	if !validSourcePathEdges(value) {
		return false
	}
	if !validSourcePathCanonical(value) {
		return false
	}
	return !strings.HasPrefix(value, "../") && !strings.Contains(value, "/../")
}

func validSourcePathText(value string) bool {
	return len(value) > 0 && len(value) <= SourcePathMaximumBytes && utf8.ValidString(value) && strings.TrimSpace(value) == value
}

func validSourcePathEdges(value string) bool {
	return value != "." && value != ".." && !strings.HasPrefix(value, "/") && !strings.HasSuffix(value, "/")
}

func validSourcePathCanonical(value string) bool {
	return path.Clean(value) == value && !strings.ContainsAny(value, "\\\x00\r\n")
}

func (v Identifier) String() string         { return v.value }
func (v Name) String() string               { return v.value }
func (v Text) String() string               { return v.value }
func (v SourcePath) String() string         { return v.value }
func (v RepositoryIdentity) String() string { return v.value }

func marshalScalar(value string, validate func() error) ([]byte, error) {
	if err := validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONString(value)
	if err != nil {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func decodeScalar(data []byte, construct func(string) error) error {
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(err)
	}
	if err := construct(value); err != nil {
		return jsonError(err)
	}
	return nil
}

func (v Identifier) MarshalJSON() ([]byte, error) { return marshalScalar(v.value, v.Validate) }
func (v Name) MarshalJSON() ([]byte, error)       { return marshalScalar(v.value, v.Validate) }
func (v Text) MarshalJSON() ([]byte, error)       { return marshalScalar(v.value, v.Validate) }
func (v SourcePath) MarshalJSON() ([]byte, error) { return marshalScalar(v.value, v.Validate) }
func (v RepositoryIdentity) MarshalJSON() ([]byte, error) {
	return marshalScalar(v.value, v.Validate)
}

func (v *Identifier) UnmarshalJSON(data []byte) error {
	if v == nil {
		return jsonError(errors.New("nil run protocol identifier receiver"))
	}
	return decodeScalar(data, func(raw string) error {
		candidate, err := NewIdentifier(raw)
		if err == nil {
			*v = candidate
		}
		return err
	})
}

func (v *Name) UnmarshalJSON(data []byte) error {
	if v == nil {
		return jsonError(errors.New("nil run protocol name receiver"))
	}
	return decodeScalar(data, func(raw string) error {
		candidate, err := NewName(raw)
		if err == nil {
			*v = candidate
		}
		return err
	})
}

func (v *Text) UnmarshalJSON(data []byte) error {
	if v == nil {
		return jsonError(errors.New("nil run protocol text receiver"))
	}
	return decodeScalar(data, func(raw string) error {
		candidate, err := NewText(raw)
		if err == nil {
			*v = candidate
		}
		return err
	})
}

func (v *SourcePath) UnmarshalJSON(data []byte) error {
	if v == nil {
		return jsonError(errors.New("nil run protocol source path receiver"))
	}
	return decodeScalar(data, func(raw string) error {
		candidate, err := ParseSourcePath(raw)
		if err == nil {
			*v = candidate
		}
		return err
	})
}

func (v *RepositoryIdentity) UnmarshalJSON(data []byte) error {
	if v == nil {
		return jsonError(errors.New("nil run protocol repository identity receiver"))
	}
	return decodeScalar(data, func(raw string) error {
		candidate, err := NewRepositoryIdentity(raw)
		if err == nil {
			*v = candidate
		}
		return err
	})
}

var (
	_ json.Marshaler   = Identifier{}
	_ json.Unmarshaler = (*Identifier)(nil)
	_ json.Marshaler   = Name{}
	_ json.Unmarshaler = (*Name)(nil)
	_ json.Marshaler   = Text{}
	_ json.Unmarshaler = (*Text)(nil)
)
