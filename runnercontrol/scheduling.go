package runnercontrol

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"slices"

	"github.com/deliri/primitive/v2026/core"
	primitiveid "github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/runprotocol"
)

const SchedulingMemberMaximum = 256

type SchedulingUnitKind uint8

const (
	SchedulingUnitUnknown SchedulingUnitKind = iota
	SchedulingUnitRunPlan
	SchedulingUnitRunBatch
	schedulingUnitKindLimit
)

func (k SchedulingUnitKind) Validate() error {
	if k <= SchedulingUnitUnknown || k >= schedulingUnitKindLimit {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (k SchedulingUnitKind) IsValid() bool { return k.Validate() == nil }

func (k SchedulingUnitKind) String() string {
	if !k.IsValid() {
		return invalidEnumString()
	}
	return []string{"", scheduleRunPlanText, scheduleRunBatchText}[k]
}

func (k SchedulingUnitKind) MarshalJSON() ([]byte, error) {
	if err := k.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(k.String())
}

func (k *SchedulingUnitKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	switch value {
	case scheduleRunPlanText:
		*k = SchedulingUnitRunPlan
	case scheduleRunBatchText:
		*k = SchedulingUnitRunBatch
	default:
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	return nil
}

type SchedulingUnitIdentity struct {
	Kind     SchedulingUnitKind `json:"kind"`
	Identity primitiveid.UUIDv7 `json:"identity"`
}

func (i SchedulingUnitIdentity) Validate() error {
	return errors.Join(i.Kind.Validate(), i.Identity.Validate())
}

type MemberSet struct {
	Entries []runprotocol.RunID `json:"entries"`
}

func (s MemberSet) Validate() error {
	if len(s.Entries) == 0 || len(s.Entries) > SchedulingMemberMaximum {
		return core.ErrPrimitiveContract
	}
	var previous []byte
	for index := range s.Entries {
		if err := s.Entries[index].Validate(); err != nil {
			return err
		}
		encoded, err := s.Entries[index].MarshalJSON()
		if err != nil {
			return err
		}
		if index > 0 && bytes.Compare(previous, encoded) >= 0 {
			return core.ErrPrimitiveContract
		}
		previous = encoded
	}
	return nil
}

func (s MemberSet) Contains(run runprotocol.RunID) bool {
	if s.Validate() != nil || run.Validate() != nil {
		return false
	}
	return slices.Contains(s.Entries, run)
}

func (s MemberSet) Digest() (core.SHA256Digest, error) {
	if err := s.Validate(); err != nil {
		return core.SHA256Digest{}, err
	}
	encoded, err := core.MarshalCanonicalJSONDocument(s)
	if err != nil {
		return core.SHA256Digest{}, errors.Join(core.ErrJSONContract, err)
	}
	return core.SHA256Of(encoded), nil
}

type SchedulingFence struct {
	Machine         MachineFence           `json:"machine_fence"`
	Unit            SchedulingUnitIdentity `json:"scheduling_unit"`
	MemberSetDigest core.SHA256Digest      `json:"member_set_digest"`
}

func (f SchedulingFence) Validate() error {
	return errors.Join(f.Machine.Validate(), f.Unit.Validate(), f.MemberSetDigest.Validate())
}

var (
	_ core.Validatable = SchedulingUnitUnknown
	_ json.Unmarshaler = (*SchedulingUnitKind)(nil)
	_ core.Validatable = SchedulingUnitIdentity{}
	_ core.Validatable = MemberSet{}
	_ core.Validatable = SchedulingFence{}
)
