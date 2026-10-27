package googlecompute

import (
	"errors"
	"net/url"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"google.golang.org/api/compute/v1"
)

func projectInstance(reference InstanceReference, wire *compute.Instance) (InstanceObservation, error) {
	if wire == nil {
		return InstanceObservation{}, core.ErrGoogleComputeResponse
	}
	if wire.Name != reference.Instance.String() {
		return InstanceObservation{}, core.ErrGoogleComputeBinding
	}
	if _, err := boundLink(wire.Zone, zonePath(reference)); err != nil {
		return InstanceObservation{}, err
	}
	if _, err := boundLink(wire.SelfLink, instancePath(reference)); err != nil {
		return InstanceObservation{}, err
	}
	state, err := instanceState(wire.Status)
	if err != nil {
		return InstanceObservation{}, err
	}
	instant, err := observedInstant()
	if err != nil {
		return InstanceObservation{}, errors.Join(core.ErrGoogleComputeResponse, err)
	}
	return InstanceObservation{State: state, ObservedAt: instant}, nil
}

type operationProjection struct {
	instance      InstanceReference
	name          OperationName
	operationType OperationType
	wire          *compute.Operation
}

func projectOperation(projection operationProjection) (OperationObservation, error) {
	reference, expectedName, expectedType, wire := projection.instance, projection.name, projection.operationType, projection.wire
	if wire == nil {
		return OperationObservation{}, core.ErrGoogleComputeResponse
	}
	if err := responseStatus(wire.HTTPStatusCode, wire.Header); err != nil {
		return OperationObservation{}, err
	}
	name, err := NewOperationName(wire.Name)
	if err != nil {
		return OperationObservation{}, errors.Join(core.ErrGoogleComputeResponse, err)
	}
	if expectedName.value != "" && name != expectedName {
		return OperationObservation{}, core.ErrGoogleComputeBinding
	}
	target, err := boundLink(wire.TargetLink, instancePath(reference))
	if err != nil {
		return OperationObservation{}, err
	}
	zone, err := boundLink(wire.Zone, zonePath(reference))
	if err != nil {
		return OperationObservation{}, err
	}
	operationType, err := operationType(wire.OperationType)
	if err != nil {
		return OperationObservation{}, err
	}
	if operationType != expectedType {
		return OperationObservation{}, core.ErrGoogleComputeBinding
	}
	state, err := operationState(wire.Status)
	if err != nil {
		return OperationObservation{}, err
	}
	failed := false
	if wire.Error != nil {
		for _, entry := range wire.Error.Errors {
			if entry == nil || entry.Code == "" {
				return OperationObservation{}, core.ErrGoogleComputeResponse
			}
			failed = true
		}
	}
	instant, err := observedInstant()
	if err != nil {
		return OperationObservation{}, errors.Join(core.ErrGoogleComputeResponse, err)
	}
	return OperationObservation{Name: name, TargetLink: target, Zone: zone, Type: operationType, State: state, Failed: failed, ObservedAt: instant}, nil
}

func zonePath(r InstanceReference) string {
	return "/compute/v1/projects/" + r.Project.String() + "/zones/" + r.Zone.String()
}
func instancePath(r InstanceReference) string {
	return zonePath(r) + "/instances/" + r.Instance.String()
}
func boundLink(text, path string) (core.HTTPEndpoint, error) {
	u, err := url.Parse(text)
	if err != nil {
		return core.HTTPEndpoint{}, errors.Join(core.ErrGoogleComputeBinding, err)
	}
	if u.Scheme != "https" || (u.Host != "compute.googleapis.com" && u.Host != "www.googleapis.com") || u.User != nil || u.Path != path || u.RawPath != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return core.HTTPEndpoint{}, core.ErrGoogleComputeBinding
	}
	endpoint, err := core.ParseHTTPEndpoint(text)
	if err != nil {
		return core.HTTPEndpoint{}, errors.Join(core.ErrGoogleComputeBinding, err)
	}
	return endpoint, nil
}
func observedInstant() (temporal.Instant, error) {
	o, err := temporal.Observe()
	if err != nil {
		return temporal.Instant{}, err
	}
	return o.Instant()
}

func instanceState(text string) (InstanceState, error) {
	for value := InstanceStatePending; value <= InstanceStateTerminated; value++ {
		if value.String() == text {
			return value, nil
		}
	}
	return InstanceStateUnknown, core.ErrGoogleComputeResponse
}
func operationState(text string) (OperationState, error) {
	for value := OperationStatePending; value <= OperationStateDone; value++ {
		if value.String() == text {
			return value, nil
		}
	}
	return OperationStateUnknown, core.ErrGoogleComputeResponse
}
func operationType(text string) (OperationType, error) {
	for value := OperationTypeStart; value <= OperationTypeStop; value++ {
		if value.String() == text {
			return value, nil
		}
	}
	return OperationTypeUnknown, core.ErrGoogleComputeResponse
}
