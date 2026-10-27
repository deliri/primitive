package googlecompute

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
)

func (v InstanceState) IsValid() bool { return v.Validate() == nil }
func (v InstanceState) String() string {
	if !v.IsValid() {
		return ""
	}
	return [...]string{"", "PENDING", "PROVISIONING", "STAGING", "RUNNING", "STOPPING", "SUSPENDING", "SUSPENDED", "REPAIRING", "TERMINATED"}[v]
}
func (v InstanceState) MarshalJSON() ([]byte, error) {
	if err := v.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(v.String())
}
func (v *InstanceState) UnmarshalJSON(data []byte) error {
	if v == nil {
		return errors.Join(core.ErrJSONContract, core.ErrGoogleComputeResponse)
	}
	text, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(core.ErrGoogleComputeResponse, err)
	}
	parsed, err := instanceState(text)
	if err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*v = parsed
	return nil
}

func (v OperationState) IsValid() bool { return v.Validate() == nil }
func (v OperationState) String() string {
	if !v.IsValid() {
		return ""
	}
	return [...]string{"", "PENDING", "RUNNING", "DONE"}[v]
}
func (v OperationState) MarshalJSON() ([]byte, error) {
	if err := v.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(v.String())
}
func (v *OperationState) UnmarshalJSON(data []byte) error {
	if v == nil {
		return errors.Join(core.ErrJSONContract, core.ErrGoogleComputeResponse)
	}
	text, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(core.ErrGoogleComputeResponse, err)
	}
	parsed, err := operationState(text)
	if err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*v = parsed
	return nil
}

func (v OperationType) IsValid() bool { return v.Validate() == nil }
func (v OperationType) String() string {
	if !v.IsValid() {
		return ""
	}
	return [...]string{"", "start", "stop"}[v]
}
func (v OperationType) MarshalJSON() ([]byte, error) {
	if err := v.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(v.String())
}
func (v *OperationType) UnmarshalJSON(data []byte) error {
	if v == nil {
		return errors.Join(core.ErrJSONContract, core.ErrGoogleComputeResponse)
	}
	text, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(core.ErrGoogleComputeResponse, err)
	}
	parsed, err := operationType(text)
	if err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*v = parsed
	return nil
}
