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

func projectOperation(reference InstanceReference, expectedName OperationName, expectedType OperationType, wire *compute.Operation) (OperationObservation, error) {
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
	switch text {
	case "PENDING":
		return InstanceStatePending, nil
	case "PROVISIONING":
		return InstanceStateProvisioning, nil
	case "STAGING":
		return InstanceStateStaging, nil
	case "RUNNING":
		return InstanceStateRunning, nil
	case "STOPPING":
		return InstanceStateStopping, nil
	case "SUSPENDING":
		return InstanceStateSuspending, nil
	case "SUSPENDED":
		return InstanceStateSuspended, nil
	case "REPAIRING":
		return InstanceStateRepairing, nil
	case "TERMINATED":
		return InstanceStateTerminated, nil
	default:
		return InstanceStateUnknown, core.ErrGoogleComputeResponse
	}
}
func operationState(text string) (OperationState, error) {
	switch text {
	case "PENDING":
		return OperationStatePending, nil
	case "RUNNING":
		return OperationStateRunning, nil
	case "DONE":
		return OperationStateDone, nil
	default:
		return OperationStateUnknown, core.ErrGoogleComputeResponse
	}
}
func operationType(text string) (OperationType, error) {
	switch text {
	case "start":
		return OperationTypeStart, nil
	case "stop":
		return OperationTypeStop, nil
	default:
		return OperationTypeUnknown, core.ErrGoogleComputeResponse
	}
}
