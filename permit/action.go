// Package permit authenticates bounded, offering-bound sets of opaque action
// identifiers. Callers own the identifiers' meaning and all issuance policy.
package permit

import (
	"bytes"
	"errors"
	"slices"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
)

const (
	ActionMaximumBytes  = 64
	ActionsMaximumCount = 32
)

// Action is an opaque canonical identifier. It is not an execution enum:
// Primitive never dispatches it or interprets its spelling.
type Action struct {
	name [ActionMaximumBytes]byte
	size uint8
}

// ActionForRoute names the mechanical operation selected by a control route.
// Both sides derive this identity from the same closed route agreement.
func ActionForRoute(family controlwire.RouteFamily) (Action, error) {
	if err := family.Validate(); err != nil {
		return Action{}, errors.Join(core.ErrPermitContract, err)
	}
	return ParseAction("route." + family.Token())
}

func ParseAction(text string) (Action, error) {
	if len(text) == 0 || len(text) > ActionMaximumBytes {
		return Action{}, core.ErrPermitContract
	}
	var a Action
	copy(a.name[:], text)
	size, err := core.CheckedUint8FromInt(len(text))
	if err != nil {
		return Action{}, errors.Join(core.ErrPermitContract, err)
	}
	a.size = size
	if err := a.Validate(); err != nil {
		return Action{}, err
	}
	return a, nil
}

func (a Action) Validate() error {
	if a.size == 0 || int(a.size) > len(a.name) {
		return core.ErrPermitContract
	}
	for i, c := range a.name[:a.size] {
		if !actionCharacter(c, i > 0 && i < int(a.size)-1) {
			return core.ErrPermitContract
		}
	}
	for _, c := range a.name[a.size:] {
		if c != 0 {
			return core.ErrPermitContract
		}
	}
	return nil
}

func actionCharacter(c byte, interior bool) bool {
	if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
		return true
	}
	return interior && (c == '-' || c == '_' || c == '.')
}

func (a Action) String() string {
	if a.Validate() != nil {
		return ""
	}
	return string(a.name[:a.size])
}

func (a Action) MarshalJSON() ([]byte, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	return core.MarshalCanonicalJSONString(a.String())
}

func (a *Action) UnmarshalJSON(data []byte) error {
	if a == nil || len(data) > ActionMaximumBytes*6+2 {
		return errors.Join(core.ErrJSONContract, core.ErrPermitContract)
	}
	text, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(core.ErrJSONContract, core.ErrPermitContract, err)
	}
	candidate, err := ParseAction(text)
	if err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*a = candidate
	return nil
}

// Actions owns a fixed-size, sorted set. Empty is an explicit absence of grants.
// The caller cannot mutate storage retained by a signed or verified document.
type Actions struct {
	values [ActionsMaximumCount]Action
	count  uint8
}

func NewActions(values ...Action) (Actions, error) {
	if len(values) > ActionsMaximumCount {
		return Actions{}, core.ErrPermitContract
	}
	var set Actions
	copy(set.values[:], values)
	count, err := core.CheckedUint8FromInt(len(values))
	if err != nil {
		return Actions{}, errors.Join(core.ErrPermitContract, err)
	}
	set.count = count
	slices.SortFunc(set.values[:set.count], compareAction)
	if err := set.Validate(); err != nil {
		return Actions{}, err
	}
	return set, nil
}

func compareAction(a, b Action) int { return bytes.Compare(a.name[:], b.name[:]) }

func (s Actions) Validate() error {
	if int(s.count) > len(s.values) {
		return core.ErrPermitContract
	}
	for i, value := range s.values {
		if i >= int(s.count) {
			if value != (Action{}) {
				return core.ErrPermitContract
			}
			continue
		}
		if err := value.Validate(); err != nil {
			return err
		}
		if i > 0 && compareAction(s.values[i-1], value) >= 0 {
			return core.ErrPermitContract
		}
	}
	return nil
}

func (s Actions) Contains(action Action) (bool, error) {
	if err := errors.Join(s.Validate(), action.Validate()); err != nil {
		return false, err
	}
	_, found := slices.BinarySearchFunc(s.values[:s.count], action, compareAction)
	return found, nil
}

func (s Actions) Count() (int, error) {
	if err := s.Validate(); err != nil {
		return 0, err
	}
	return int(s.count), nil
}

// With returns an owned union with one action. Repeating an existing member is
// an exact no-op; capacity refusal returns zero and never mutates the receiver.
func (s Actions) With(action Action) (Actions, error) {
	present, err := s.Contains(action)
	if err != nil {
		return Actions{}, err
	}
	if present {
		return s, nil
	}
	if int(s.count) == len(s.values) {
		return Actions{}, core.ErrPermitContract
	}
	s.values[s.count] = action
	s.count++
	slices.SortFunc(s.values[:s.count], compareAction)
	return s, nil
}

func (s Actions) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return core.MarshalCanonicalJSONDocument(s.values[:s.count])
}

func (s *Actions) UnmarshalJSON(data []byte) error {
	if s == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPermitContract)
	}
	limits := core.DefaultStrictJSONLimits()
	maximum, err := core.NewByteCount(ActionsMaximumCount*(ActionMaximumBytes*6+3) + 2)
	if err != nil {
		return errors.Join(core.ErrJSONContract, core.ErrPermitContract, err)
	}
	limits.DocumentMaximumBytes = maximum
	limits.ArrayItemMaximum = ActionsMaximumCount
	values, err := core.DecodeStrictJSONStructure[[]Action](data, limits)
	if err != nil {
		return errors.Join(core.ErrJSONContract, core.ErrPermitContract, err)
	}
	// Null cannot stand in for an explicitly empty set.
	if values == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPermitContract)
	}
	candidate, err := NewActions(values...)
	if err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*s = candidate
	return nil
}
