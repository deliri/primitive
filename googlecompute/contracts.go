package googlecompute

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/googleidentity"
	"github.com/deliri/primitive/v2026/temporal"
)

// ResourceName is one unambiguous Google resource path segment.
type ResourceName struct{ value string }

func NewResourceName(value string) (ResourceName, error) {
	r := ResourceName{value: value}
	if err := r.Validate(); err != nil {
		return ResourceName{}, err
	}
	return r, nil
}
func (r ResourceName) Validate() error { return validateSegment(r.value) }
func (r ResourceName) String() string  { return r.value }

// OperationName is the provider-issued zonal operation name.
type OperationName struct{ value string }

func NewOperationName(value string) (OperationName, error) {
	r := OperationName{value: value}
	if err := r.Validate(); err != nil {
		return OperationName{}, err
	}
	return r, nil
}
func (r OperationName) Validate() error { return validateSegment(r.value) }
func (r OperationName) String() string  { return r.value }

func validateSegment(value string) error {
	if value == "" {
		return core.ErrGoogleComputeContract
	}
	for _, c := range []byte(value) {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return core.ErrGoogleComputeContract
		}
	}
	return nil
}

type InstanceReference struct{ Project, Zone, Instance ResourceName }

func (r InstanceReference) Validate() error {
	return errors.Join(r.Project.Validate(), r.Zone.Validate(), r.Instance.Validate())
}

type InstanceReadRequest struct {
	Instance InstanceReference
	Token    googleidentity.AccessToken
	Policy   exchange.OperationPolicy
}
type InstanceStartRequest struct {
	Instance InstanceReference
	Token    googleidentity.AccessToken
	Policy   exchange.OperationPolicy
}
type InstanceStopRequest struct {
	Instance InstanceReference
	Token    googleidentity.AccessToken
	Policy   exchange.OperationPolicy
}
type OperationReadRequest struct {
	Instance  InstanceReference
	Operation OperationName
	Type      OperationType
	Token     googleidentity.AccessToken
	Policy    exchange.OperationPolicy
}

func (r InstanceReadRequest) Validate() error  { return validateRequest(r.Instance, r.Token, r.Policy) }
func (r InstanceStartRequest) Validate() error { return validateRequest(r.Instance, r.Token, r.Policy) }
func (r InstanceStopRequest) Validate() error  { return validateRequest(r.Instance, r.Token, r.Policy) }
func (r OperationReadRequest) Validate() error {
	return errors.Join(validateRequest(r.Instance, r.Token, r.Policy), r.Operation.Validate(), r.Type.Validate())
}
func validateRequest(instance InstanceReference, token googleidentity.AccessToken, policy exchange.OperationPolicy) error {
	err := errors.Join(instance.Validate(), token.Validate(), policy.Validate())
	if policy.Retry.MaximumAttempts != 1 || policy.Redirect.Mode != exchange.RedirectReject {
		err = errors.Join(err, core.ErrGoogleComputeContract)
	}
	if err != nil {
		return errors.Join(core.ErrGoogleComputeContract, err)
	}
	return nil
}

type InstanceState uint8

const (
	InstanceStateUnknown InstanceState = iota
	InstanceStatePending
	InstanceStateProvisioning
	InstanceStateStaging
	InstanceStateRunning
	InstanceStateStopping
	InstanceStateSuspending
	InstanceStateSuspended
	InstanceStateRepairing
	InstanceStateTerminated
)

func (s InstanceState) Validate() error {
	if s < InstanceStatePending || s > InstanceStateTerminated {
		return core.ErrGoogleComputeResponse
	}
	return nil
}

type OperationState uint8

const (
	OperationStateUnknown OperationState = iota
	OperationStatePending
	OperationStateRunning
	OperationStateDone
)

func (s OperationState) Validate() error {
	if s < OperationStatePending || s > OperationStateDone {
		return core.ErrGoogleComputeResponse
	}
	return nil
}

type OperationType uint8

const (
	OperationTypeUnknown OperationType = iota
	OperationTypeStart
	OperationTypeStop
)

func (v OperationType) Validate() error {
	if v != OperationTypeStart && v != OperationTypeStop {
		return core.ErrGoogleComputeContract
	}
	return nil
}

type InstanceObservation struct {
	State      InstanceState
	ObservedAt temporal.Instant
}

func (o InstanceObservation) Validate() error {
	return errors.Join(o.State.Validate(), o.ObservedAt.Validate())
}

// OperationObservation retains only bound operation facts. Failed reports an
// actual provider error entry; application recovery policy remains the caller's.
type OperationObservation struct {
	Name       OperationName
	TargetLink core.HTTPEndpoint
	Zone       core.HTTPEndpoint
	Type       OperationType
	State      OperationState
	Failed     bool
	ObservedAt temporal.Instant
}

func (o OperationObservation) Validate() error {
	return errors.Join(o.Name.Validate(), o.TargetLink.Validate(), o.Zone.Validate(), o.Type.Validate(), o.State.Validate(), o.ObservedAt.Validate())
}
