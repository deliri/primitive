package taskmanager

import (
	"context"
	"errors"
	"net/http"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	jsonMaximumBytes       = 256 << 10
	operationTimeoutSecond = 15
)

// ClientConfiguration binds the product-neutral task-manager client to one
// caller-owned Exchange client and one HTTP origin.
type ClientConfiguration struct {
	HTTP      exchange.Client
	Headers   exchange.Headers
	Authority core.HTTPEndpoint
}

func (c ClientConfiguration) Validate() error {
	if err := errors.Join(c.HTTP.Validate(), c.Authority.Validate(), c.Headers.Validate()); err != nil {
		return contractError(err)
	}
	url := c.Authority.HTTPURL()
	if url.Scheme != core.SchemeHTTPS || url.Path != "" || url.RawPath != "" ||
		url.RawQuery != "" || url.ForceQuery {
		return contractError()
	}
	return nil
}

// Client is the paired caller side of the task-manager socket.
type Client struct {
	http    exchange.Client
	headers exchange.Headers
	targets [routeLimit]core.HTTPEndpoint
	policy  exchange.JSONPolicy
}

// NewClient constructs every route target and validates the complete client
// before any network operation can begin.
func NewClient(configuration ClientConfiguration) (Client, error) {
	if err := configuration.Validate(); err != nil {
		return Client{}, err
	}
	policy, err := socketJSONPolicy()
	if err != nil {
		return Client{}, err
	}
	client := Client{http: configuration.HTTP, policy: policy, headers: configuration.Headers}
	for route := RouteListProjects; route < routeLimit; route++ {
		target, targetErr := routeTarget(configuration.Authority, route)
		if targetErr != nil {
			return Client{}, targetErr
		}
		client.targets[route] = target
	}
	if err := client.Validate(); err != nil {
		return Client{}, err
	}
	return client, nil
}

func (c Client) Validate() error {
	if err := errors.Join(c.http.Validate(), c.policy.Validate(), c.headers.Validate()); err != nil {
		return contractError(err)
	}
	for route := RouteListProjects; route < routeLimit; route++ {
		if err := c.targets[route].Validate(); err != nil {
			return contractError(err)
		}
	}
	return nil
}

// ListProjects crosses one real HTTP socket with a bounded typed page request.
func (c Client) ListProjects(ctx context.Context, request ListProjectsRequest) (ProjectPage, error) {
	var zero ProjectPage
	semantics, err := clientSemantics(RouteListProjects, exchange.IdempotencyKey{})
	if err != nil {
		return zero, err
	}
	response, err := exchange.SendJSON[ListProjectsRequest, ProjectPage](exchange.JSONCall[ListProjectsRequest]{
		Context: ctx,
		Client:  c.http,
		Request: exchange.JSONRequest[ListProjectsRequest]{
			Target: c.targets[RouteListProjects], Body: request, Semantics: semantics,
			Headers: c.headers, ExpectedStatus: core.HTTPStatusOK(),
		},
		Policy: c.policy,
	})
	if err != nil {
		return zero, contractError(err)
	}
	if response.Body.Lifecycle != request.Lifecycle || response.Body.Order != request.Order {
		return zero, contractError()
	}
	if len(response.Body.Items) > int(request.Limit) {
		return zero, contractError()
	}
	return response.Body, nil
}

// UpdateTask binds the mutation identity in the typed document to the real
// idempotency header before crossing the socket.
func (c Client) UpdateTask(ctx context.Context, request UpdateTaskRequest) (TaskDetail, error) {
	var zero TaskDetail
	key, err := request.IdempotencyKey()
	if err != nil {
		return zero, err
	}
	semantics, err := clientSemantics(RouteUpdateTask, key)
	if err != nil {
		return zero, err
	}
	response, err := exchange.SendReplayBoundJSON[UpdateTaskRequest, TaskDetail](exchange.JSONCall[UpdateTaskRequest]{
		Context: ctx,
		Client:  c.http,
		Request: exchange.JSONRequest[UpdateTaskRequest]{
			Target: c.targets[RouteUpdateTask], Body: request, Semantics: semantics,
			Headers: c.headers, ExpectedStatus: core.HTTPStatusOK(),
		},
		Policy: c.policy,
	})
	if err != nil {
		return zero, contractError(err)
	}
	if response.Body.Summary.ID != request.TaskID || response.Body.Summary.ProjectID != request.ProjectID ||
		!revisionAdvances(request.ExpectedRevision, response.Body.Summary.Revision) ||
		!changeMatchesDetail(request.Change, response.Body) {
		return zero, contractError()
	}
	return response.Body, nil
}

func changeMatchesDetail(change TaskChange, detail TaskDetail) bool {
	summary := detail.Summary
	if change.Title != nil && *change.Title != summary.Title {
		return false
	}
	if change.Kind != nil && *change.Kind != summary.Kind {
		return false
	}
	if change.State != nil && *change.State != summary.State {
		return false
	}
	if change.Description != nil && *change.Description != detail.Description {
		return false
	}
	return true
}

// ReceiveListProjects validates the mounted socket before admitting the body.
func ReceiveListProjects(request *http.Request) (exchange.Received[*ListProjectsRequest], error) {
	var zero exchange.Received[*ListProjectsRequest]
	semantics, err := serverRoute(request, RouteListProjects)
	if err != nil {
		return zero, err
	}
	policy, err := socketServerPolicy()
	if err != nil {
		return zero, err
	}
	received, err := exchange.ReceiveJSON[ListProjectsRequest, *ListProjectsRequest](exchange.JSONReceiveCall{
		Request: request, Route: semantics, Policy: policy,
	})
	if err != nil {
		return zero, contractError(err)
	}
	return received, nil
}

// ReceiveUpdateTask validates the mounted socket and binds the real replay
// header to the typed mutation identity.
func ReceiveUpdateTask(request *http.Request) (exchange.Received[*UpdateTaskRequest], error) {
	var zero exchange.Received[*UpdateTaskRequest]
	semantics, err := serverRoute(request, RouteUpdateTask)
	if err != nil {
		return zero, err
	}
	policy, err := socketServerPolicy()
	if err != nil {
		return zero, err
	}
	received, err := exchange.ReceiveReplayBoundJSON[UpdateTaskRequest, *UpdateTaskRequest](exchange.JSONReceiveCall{
		Request: request, Route: semantics, Policy: policy,
	})
	if err != nil {
		return zero, contractError(err)
	}
	return received, nil
}

// WriteProjectPage validates and writes one bounded project page.
func WriteProjectPage(writer http.ResponseWriter, page ProjectPage) error {
	return writeJSON(writer, page)
}

// WriteTaskSummary validates and writes one updated task summary.
func WriteTaskSummary(writer http.ResponseWriter, summary TaskSummary) error {
	return writeJSON(writer, summary)
}

func writeJSON[Body core.ValidatedJSONMarshaler](writer http.ResponseWriter, body Body) error {
	limit, err := socketJSONLimit()
	if err != nil {
		return err
	}
	err = exchange.WriteJSON(exchange.JSONWriteCall[Body]{
		Writer: writer,
		Response: exchange.ServerJSONResponse[Body]{
			Body: body, Status: core.HTTPStatusOK(),
		},
		Policy: exchange.JSONWritePolicy{ResponseBodyLimit: limit},
	})
	if err != nil {
		return contractError(err)
	}
	return nil
}

func serverRoute(request *http.Request, route Route) (exchange.RouteSemantics, error) {
	semantics, err := route.Semantics()
	if err != nil || request == nil || request.URL == nil {
		return exchange.RouteSemantics{}, contractError(err)
	}
	path, err := route.Path()
	if err != nil || request.URL.Path != path || request.URL.RawPath != "" ||
		request.URL.RawQuery != "" || request.URL.ForceQuery {
		return exchange.RouteSemantics{}, contractError(err)
	}
	return semantics, nil
}

func clientSemantics(route Route, key exchange.IdempotencyKey) (exchange.RequestSemantics, error) {
	server, err := route.Semantics()
	if err != nil {
		return exchange.RequestSemantics{}, err
	}
	semantics := exchange.RequestSemantics{Method: server.Method, Replay: server.Replay, IdempotencyKey: key}
	if err := semantics.Validate(); err != nil {
		return exchange.RequestSemantics{}, contractError(err)
	}
	return semantics, nil
}

func routeTarget(authority core.HTTPEndpoint, route Route) (core.HTTPEndpoint, error) {
	path, err := route.Path()
	if err != nil {
		return core.HTTPEndpoint{}, err
	}
	target, err := core.ParseHTTPEndpoint(authority.String() + path)
	if err != nil {
		return core.HTTPEndpoint{}, contractError(err)
	}
	return target, nil
}

func socketJSONPolicy() (exchange.JSONPolicy, error) {
	timeout, err := temporal.DurationFromSeconds(operationTimeoutSecond)
	if err != nil {
		return exchange.JSONPolicy{}, contractError(err)
	}
	limit, err := socketJSONLimit()
	if err != nil {
		return exchange.JSONPolicy{}, err
	}
	policy := exchange.JSONPolicy{
		Operation: exchange.OperationPolicy{
			OperationTimeout: timeout,
			AttemptTimeout:   timeout,
			Retry:            exchange.RetryPolicy{MaximumAttempts: 1},
			Redirect:         exchange.RedirectPolicy{Mode: exchange.RedirectReject},
		},
		RequestBodyLimit: limit, ResponseBodyLimit: limit,
	}
	if err := policy.Validate(); err != nil {
		return exchange.JSONPolicy{}, contractError(err)
	}
	return policy, nil
}

func socketServerPolicy() (exchange.ServerPolicy, error) {
	limit, err := socketJSONLimit()
	if err != nil {
		return exchange.ServerPolicy{}, err
	}
	return exchange.ServerPolicy{RequestBodyLimit: limit}, nil
}

func socketJSONLimit() (core.ByteCount, error) {
	limit, err := core.NewByteCount(jsonMaximumBytes)
	if err != nil {
		return core.ByteCount{}, contractError(err)
	}
	return limit, nil
}

var (
	_ core.Validatable = ClientConfiguration{}
	_ core.Validatable = Client{}
)
