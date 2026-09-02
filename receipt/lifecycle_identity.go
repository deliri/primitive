package receipt

import (
	"encoding/hex"
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// LifecycleIdentityBytes is the fixed width of principal, submission, and
	// object identities.
	LifecycleIdentityBytes = 16
	// LifecycleIdentityHexBytes is the canonical hexadecimal text width.
	LifecycleIdentityHexBytes = LifecycleIdentityBytes * 2
)

type lifecycleIdentity struct {
	value [LifecycleIdentityBytes]byte
}

type principalIdentityDomain uint8
type submissionIdentityDomain uint8
type objectIdentityDomain uint8

// PrincipalIdentity identifies one opaque consumer-owned namespace principal.
type PrincipalIdentity struct {
	value lifecycleIdentity
	_     principalIdentityDomain
}

// SubmissionIdentity identifies one submitted input.
type SubmissionIdentity struct {
	value lifecycleIdentity
	_     submissionIdentityDomain
}

// ObjectIdentity identifies one immutable object.
type ObjectIdentity struct {
	value lifecycleIdentity
	_     objectIdentityDomain
}

func newLifecycleIdentity(value [LifecycleIdentityBytes]byte) (lifecycleIdentity, error) {
	if value == ([LifecycleIdentityBytes]byte{}) {
		return lifecycleIdentity{}, lifecycleIdentityError("lifecycle identity is all zero")
	}
	return lifecycleIdentity{value: value}, nil
}

func parseLifecycleIdentity(value string) (lifecycleIdentity, error) {
	var raw [LifecycleIdentityBytes]byte
	if err := decodeCanonicalIdentity(value, raw[:]); err != nil {
		return lifecycleIdentity{}, contractError(core.ErrLifecycleIdentityContract, err)
	}
	return newLifecycleIdentity(raw)
}

func decodeCanonicalIdentity(value string, destination []byte) error {
	if len(value) != hex.EncodedLen(len(destination)) {
		return errors.New("identity has invalid text length")
	}
	if err := core.DecodeCanonicalHex(destination, value); err != nil {
		return errors.Join(errors.New("identity text is not canonical"), err)
	}
	return nil
}

func (i lifecycleIdentity) validate() error {
	if i.value == ([LifecycleIdentityBytes]byte{}) {
		return lifecycleIdentityError("lifecycle identity is unset")
	}
	return nil
}

func (i lifecycleIdentity) string() string {
	if i.validate() != nil {
		return ""
	}
	return hex.EncodeToString(i.value[:])
}

func marshalLifecycleIdentity(i lifecycleIdentity) ([]byte, error) {
	if err := i.validate(); err != nil {
		return nil, jsonError(err)
	}
	return json.Marshal(i.string())
}

func unmarshalLifecycleIdentity(data []byte) (lifecycleIdentity, error) {
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return lifecycleIdentity{}, jsonError(core.ErrLifecycleIdentityContract, err)
	}
	decoded, err := parseLifecycleIdentity(value)
	if err != nil {
		return lifecycleIdentity{}, jsonError(err)
	}
	return decoded, nil
}

// NewPrincipalIdentity constructs a principal identity from its exact bytes.
func NewPrincipalIdentity(value [LifecycleIdentityBytes]byte) (PrincipalIdentity, error) {
	identity, err := newLifecycleIdentity(value)
	if err != nil {
		return PrincipalIdentity{}, err
	}
	return PrincipalIdentity{value: identity}, nil
}

// ParsePrincipalIdentity accepts canonical lowercase hexadecimal.
func ParsePrincipalIdentity(value string) (PrincipalIdentity, error) {
	identity, err := parseLifecycleIdentity(value)
	if err != nil {
		return PrincipalIdentity{}, err
	}
	return PrincipalIdentity{value: identity}, nil
}

// Validate rejects the unset identity.
func (i PrincipalIdentity) Validate() error { return i.value.validate() }

// String returns canonical lowercase hexadecimal, or empty text when unset.
func (i PrincipalIdentity) String() string { return i.value.string() }

// MarshalJSON emits canonical lowercase hexadecimal.
func (i PrincipalIdentity) MarshalJSON() ([]byte, error) {
	return marshalLifecycleIdentity(i.value)
}

// UnmarshalJSON accepts one canonical principal identity without mutating on failure.
func (i *PrincipalIdentity) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError(lifecycleIdentityError("nil principal identity receiver"))
	}
	value, err := unmarshalLifecycleIdentity(data)
	if err != nil {
		return err
	}
	*i = PrincipalIdentity{value: value}
	return nil
}

// ScopeFor derives the exact receipt namespace for one opaque principal and
// offering. The consumer decides what the principal represents.
func ScopeFor(principal PrincipalIdentity, offering core.Offering) (Scope, error) {
	scope := Scope{Principal: principal, Offering: offering}
	if err := scope.Validate(); err != nil {
		return Scope{}, err
	}
	return scope, nil
}

// NewSubmissionIdentity constructs a submission identity from its exact bytes.
func NewSubmissionIdentity(value [LifecycleIdentityBytes]byte) (SubmissionIdentity, error) {
	identity, err := newLifecycleIdentity(value)
	if err != nil {
		return SubmissionIdentity{}, err
	}
	return SubmissionIdentity{value: identity}, nil
}

// ParseSubmissionIdentity accepts canonical lowercase hexadecimal.
func ParseSubmissionIdentity(value string) (SubmissionIdentity, error) {
	identity, err := parseLifecycleIdentity(value)
	if err != nil {
		return SubmissionIdentity{}, err
	}
	return SubmissionIdentity{value: identity}, nil
}

// Validate rejects the unset identity.
func (i SubmissionIdentity) Validate() error { return i.value.validate() }

// String returns canonical lowercase hexadecimal, or empty text when unset.
func (i SubmissionIdentity) String() string { return i.value.string() }

// MarshalJSON emits canonical lowercase hexadecimal.
func (i SubmissionIdentity) MarshalJSON() ([]byte, error) {
	return marshalLifecycleIdentity(i.value)
}

// UnmarshalJSON accepts one canonical submission identity without mutating on failure.
func (i *SubmissionIdentity) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError(lifecycleIdentityError("nil submission identity receiver"))
	}
	value, err := unmarshalLifecycleIdentity(data)
	if err != nil {
		return err
	}
	*i = SubmissionIdentity{value: value}
	return nil
}

// NewObjectIdentity constructs an object identity from its exact bytes.
func NewObjectIdentity(value [LifecycleIdentityBytes]byte) (ObjectIdentity, error) {
	identity, err := newLifecycleIdentity(value)
	if err != nil {
		return ObjectIdentity{}, err
	}
	return ObjectIdentity{value: identity}, nil
}

// ParseObjectIdentity accepts canonical lowercase hexadecimal.
func ParseObjectIdentity(value string) (ObjectIdentity, error) {
	identity, err := parseLifecycleIdentity(value)
	if err != nil {
		return ObjectIdentity{}, err
	}
	return ObjectIdentity{value: identity}, nil
}

// Validate rejects the unset identity.
func (i ObjectIdentity) Validate() error { return i.value.validate() }

// String returns canonical lowercase hexadecimal, or empty text when unset.
func (i ObjectIdentity) String() string { return i.value.string() }

// MarshalJSON emits canonical lowercase hexadecimal.
func (i ObjectIdentity) MarshalJSON() ([]byte, error) {
	return marshalLifecycleIdentity(i.value)
}

// UnmarshalJSON accepts one canonical object identity without mutating on failure.
func (i *ObjectIdentity) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError(lifecycleIdentityError("nil object identity receiver"))
	}
	value, err := unmarshalLifecycleIdentity(data)
	if err != nil {
		return err
	}
	*i = ObjectIdentity{value: value}
	return nil
}

func lifecycleIdentityError(message string) error {
	return contractError(core.ErrLifecycleIdentityContract, errors.New(message))
}
