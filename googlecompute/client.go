package googlecompute

import (
	"context"
	"errors"
	"net/http"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

// Client borrows the caller's transport. It owns no idle pool or credentials.
type Client struct{ service *compute.Service }

func NewClient(network exchange.Client) (Client, error) {
	if err := network.Validate(); err != nil {
		return Client{}, errors.Join(core.ErrGoogleComputeContract, err)
	}
	get, err := exchange.NewOfficialSDKMethodResponseBoundary(exchange.OfficialSDKMethodResponseBoundaryRequest{Method: exchange.MethodGet, Representation: exchange.OfficialSDKResponseRepresentationJSON})
	if err != nil {
		return Client{}, errors.Join(core.ErrGoogleComputeContract, err)
	}
	transport, err := network.OfficialSDKResponseTransport(get)
	if err != nil {
		return Client{}, errors.Join(core.ErrGoogleComputeContract, err)
	}
	post, err := exchange.NewOfficialSDKMethodResponseBoundary(exchange.OfficialSDKMethodResponseBoundaryRequest{Method: exchange.MethodPost, Representation: exchange.OfficialSDKResponseRepresentationJSON})
	if err != nil {
		return Client{}, errors.Join(core.ErrGoogleComputeContract, err)
	}
	transport, err = exchange.NewOfficialSDKResponseTransport(exchange.OfficialSDKResponseTransportRequest{Base: transport, Boundary: post})
	if err != nil {
		return Client{}, errors.Join(core.ErrGoogleComputeContract, err)
	}
	httpClient, err := exchange.NewOfficialSDKHTTPClient(transport)
	if err != nil {
		return Client{}, errors.Join(core.ErrGoogleComputeContract, err)
	}
	service, err := compute.NewService(context.Background(), option.WithHTTPClient(httpClient))
	if err != nil {
		return Client{}, errors.Join(core.ErrGoogleComputeContract, err)
	}
	return Client{service: service}, nil
}
func (c Client) Validate() error {
	if c.service == nil {
		return core.ErrGoogleComputeContract
	}
	return nil
}

func (c Client) ReadInstance(ctx context.Context, request InstanceReadRequest) (InstanceObservation, error) {
	if err := errors.Join(c.Validate(), request.Validate(), contextError(ctx)); err != nil {
		return InstanceObservation{}, errors.Join(core.ErrGoogleComputeContract, err)
	}
	callContext, cancel, err := attemptContext(ctx, request.Policy)
	if err != nil {
		return InstanceObservation{}, err
	}
	defer cancel()
	r := request.Instance
	call := c.service.Instances.Get(r.Project.String(), r.Zone.String(), r.Instance.String()).Context(callContext).Fields("name", "zone", "selfLink", "status")
	bearer, err := request.Token.BearerValue()
	if err != nil {
		return InstanceObservation{}, errors.Join(core.ErrGoogleComputeContract, err)
	}
	call.Header().Set("Authorization", bearer)
	result, err := call.Do()
	if err != nil {
		return InstanceObservation{}, responseError(callContext, err)
	}
	if result == nil {
		return InstanceObservation{}, core.ErrGoogleComputeResponse
	}
	if err := responseStatus(result.HTTPStatusCode); err != nil {
		return InstanceObservation{}, err
	}
	return projectInstance(r, result)
}

func (c Client) StartInstance(ctx context.Context, request InstanceStartRequest) (OperationObservation, error) {
	if err := errors.Join(c.Validate(), request.Validate(), contextError(ctx)); err != nil {
		return OperationObservation{}, errors.Join(core.ErrGoogleComputeContract, err)
	}
	callContext, cancel, err := attemptContext(ctx, request.Policy)
	if err != nil {
		return OperationObservation{}, err
	}
	defer cancel()
	r := request.Instance
	call := c.service.Instances.Start(r.Project.String(), r.Zone.String(), r.Instance.String()).Context(callContext).Fields(operationFields)
	bearer, err := request.Token.BearerValue()
	if err != nil {
		return OperationObservation{}, errors.Join(core.ErrGoogleComputeContract, err)
	}
	call.Header().Set("Authorization", bearer)
	result, err := call.Do()
	if err != nil {
		return OperationObservation{}, responseError(callContext, err)
	}
	return projectOperation(r, OperationName{}, OperationTypeStart, result)
}

func (c Client) StopInstance(ctx context.Context, request InstanceStopRequest) (OperationObservation, error) {
	if err := errors.Join(c.Validate(), request.Validate(), contextError(ctx)); err != nil {
		return OperationObservation{}, errors.Join(core.ErrGoogleComputeContract, err)
	}
	callContext, cancel, err := attemptContext(ctx, request.Policy)
	if err != nil {
		return OperationObservation{}, err
	}
	defer cancel()
	r := request.Instance
	call := c.service.Instances.Stop(r.Project.String(), r.Zone.String(), r.Instance.String()).Context(callContext).Fields(operationFields)
	bearer, err := request.Token.BearerValue()
	if err != nil {
		return OperationObservation{}, errors.Join(core.ErrGoogleComputeContract, err)
	}
	call.Header().Set("Authorization", bearer)
	result, err := call.Do()
	if err != nil {
		return OperationObservation{}, responseError(callContext, err)
	}
	return projectOperation(r, OperationName{}, OperationTypeStop, result)
}

func (c Client) ReadOperation(ctx context.Context, request OperationReadRequest) (OperationObservation, error) {
	if err := errors.Join(c.Validate(), request.Validate(), contextError(ctx)); err != nil {
		return OperationObservation{}, errors.Join(core.ErrGoogleComputeContract, err)
	}
	callContext, cancel, err := attemptContext(ctx, request.Policy)
	if err != nil {
		return OperationObservation{}, err
	}
	defer cancel()
	r := request.Instance
	call := c.service.ZoneOperations.Get(r.Project.String(), r.Zone.String(), request.Operation.String()).Context(callContext).Fields(operationFields)
	bearer, err := request.Token.BearerValue()
	if err != nil {
		return OperationObservation{}, errors.Join(core.ErrGoogleComputeContract, err)
	}
	call.Header().Set("Authorization", bearer)
	result, err := call.Do()
	if err != nil {
		return OperationObservation{}, responseError(callContext, err)
	}
	return projectOperation(r, request.Operation, request.Type, result)
}

// The four generated Do methods use gensupport.SendRequest, not its retrying
// variant. With one attempt, the admitted attempt deadline also bounds total work.
func attemptContext(ctx context.Context, policy exchange.OperationPolicy) (context.Context, context.CancelFunc, error) {
	return temporal.WithTimeout(temporal.TimeoutRequest{Parent: ctx, Duration: policy.AttemptTimeout})
}

func responseError(ctx context.Context, err error) error {
	return errors.Join(core.ErrGoogleComputeResponse, err, contextError(ctx))
}

// context.Cause may call a foreign context's Value method. Keep that refusal
// typed while preserving ordinary native and caller-specific terminal causes.
func contextError(ctx context.Context) (err error) {
	if err = contextstate.Validate(ctx); err == nil {
		return nil
	}
	defer func() {
		if value := recover(); value != nil {
			err = errors.Join(err, core.ErrContextObservation)
		}
	}()
	return errors.Join(err, context.Cause(ctx))
}

func responseStatus(status int) error {
	if status != http.StatusOK {
		return errors.Join(core.ErrGoogleComputeResponse, &googleapi.Error{Code: status})
	}
	return nil
}

const operationFields googleapi.Field = "name,targetLink,zone,operationType,status,error"
