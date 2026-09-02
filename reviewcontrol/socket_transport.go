package reviewcontrol

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/proofledger"
)

func SocketContract(operation Operation, path exchange.SocketRoutePath) (exchange.JSONSocketContract, error) {
	if err := errors.Join(operation.Validate(), path.Validate()); err != nil {
		return exchange.JSONSocketContract{}, contractError(err)
	}
	requestLimit, err := core.NewByteCount(SocketDocumentMaximumBytes)
	if err != nil {
		return exchange.JSONSocketContract{}, contractError(err)
	}
	responseLimit, err := core.NewByteCount(SocketDocumentMaximumBytes)
	if err != nil {
		return exchange.JSONSocketContract{}, contractError(err)
	}
	replay, err := operationReplay(operation)
	if err != nil {
		return exchange.JSONSocketContract{}, err
	}
	contract := exchange.JSONSocketContract{
		Path: path, Route: exchange.RouteSemantics{Method: exchange.MethodPost, Replay: replay},
		RequestBodyLimit: requestLimit, ResponseBodyLimit: responseLimit, SuccessStatus: core.HTTPStatusOK(),
	}
	if err := contract.Validate(); err != nil {
		return exchange.JSONSocketContract{}, contractError(err)
	}
	return contract, nil
}

func operationReplay(operation Operation) (exchange.ReplayMode, error) {
	switch operation {
	case OperationIssueReview, OperationRecordObservation, OperationRecordDecision:
		return exchange.ReplayIdempotencyKey, nil
	case OperationReadReview, OperationReadEvents, OperationReadProjection:
		return exchange.ReplaySingleAttempt, nil
	default:
		return exchange.ReplayUnknown, contractError(errors.New("review control operation has no socket replay contract"))
	}
}

type SocketResult[Response core.Validatable] struct {
	Response Response
	Metadata exchange.ResponseMetadata
}

func (r SocketResult[Response]) Validate() error {
	if err := errors.Join(r.Response.Validate(), r.Metadata.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

type IssuerSocketClient struct{ socket exchange.ClientSocket }
type ObservationSocketClient struct{ socket exchange.ClientSocket }
type DecisionSocketClient struct{ socket exchange.ClientSocket }

type ReadSocketClientConfiguration struct {
	Review     exchange.ClientSocketConfiguration
	Events     exchange.ClientSocketConfiguration
	Projection exchange.ClientSocketConfiguration
}

type ReadSocketClient struct {
	review     exchange.ClientSocket
	events     exchange.ClientSocket
	projection exchange.ClientSocket
}

func NewIssuerSocketClient(configuration exchange.ClientSocketConfiguration) (IssuerSocketClient, error) {
	socket, err := newOperationClient(OperationIssueReview, configuration)
	return IssuerSocketClient{socket: socket}, err
}

func NewObservationSocketClient(configuration exchange.ClientSocketConfiguration) (ObservationSocketClient, error) {
	socket, err := newOperationClient(OperationRecordObservation, configuration)
	return ObservationSocketClient{socket: socket}, err
}

func NewDecisionSocketClient(configuration exchange.ClientSocketConfiguration) (DecisionSocketClient, error) {
	socket, err := newOperationClient(OperationRecordDecision, configuration)
	return DecisionSocketClient{socket: socket}, err
}

func NewReadSocketClient(configuration ReadSocketClientConfiguration) (ReadSocketClient, error) {
	review, err := newOperationClient(OperationReadReview, configuration.Review)
	if err != nil {
		return ReadSocketClient{}, err
	}
	events, err := newOperationClient(OperationReadEvents, configuration.Events)
	if err != nil {
		return ReadSocketClient{}, err
	}
	projection, err := newOperationClient(OperationReadProjection, configuration.Projection)
	if err != nil {
		return ReadSocketClient{}, err
	}
	return ReadSocketClient{review: review, events: events, projection: projection}, nil
}

func newOperationClient(operation Operation, configuration exchange.ClientSocketConfiguration) (exchange.ClientSocket, error) {
	want, err := SocketContract(operation, configuration.Contract.Path)
	if err != nil {
		return exchange.ClientSocket{}, err
	}
	if !sameSocketContract(configuration.Contract, want) {
		return exchange.ClientSocket{}, contractError(errors.New("review control client socket contract differs from operation"))
	}
	socket, err := exchange.NewClientSocket(configuration)
	if err != nil {
		return exchange.ClientSocket{}, contractError(err)
	}
	return socket, nil
}

func (c IssuerSocketClient) IssueReview(ctx context.Context, request IssueReviewRequest) (SocketResult[IssueReviewResponse], error) {
	response, err := exchange.SendReplayBoundSocketJSON[IssueReviewRequest, IssueReviewResponse](ctx, c.socket, request)
	if err != nil {
		return SocketResult[IssueReviewResponse]{}, contractError(err)
	}
	if err := bindReceiptRequest(request.Request, response.Body.Receipt); err != nil {
		return SocketResult[IssueReviewResponse]{}, err
	}
	return newSocketResult(response)
}

func (c ObservationSocketClient) RecordObservation(ctx context.Context, request RecordObservationRequest) (SocketResult[RecordObservationResponse], error) {
	response, err := exchange.SendReplayBoundSocketJSON[RecordObservationRequest, RecordObservationResponse](ctx, c.socket, request)
	if err != nil {
		return SocketResult[RecordObservationResponse]{}, contractError(err)
	}
	if err := bindReceiptRequest(request.Request, response.Body.Receipt); err != nil {
		return SocketResult[RecordObservationResponse]{}, err
	}
	return newSocketResult(response)
}

func (c DecisionSocketClient) RecordDecision(ctx context.Context, request RecordDecisionRequest) (SocketResult[RecordDecisionResponse], error) {
	response, err := exchange.SendReplayBoundSocketJSON[RecordDecisionRequest, RecordDecisionResponse](ctx, c.socket, request)
	if err != nil {
		return SocketResult[RecordDecisionResponse]{}, contractError(err)
	}
	if err := bindReceiptRequest(request.Intent.Request, response.Body.Receipt); err != nil {
		return SocketResult[RecordDecisionResponse]{}, err
	}
	return newSocketResult(response)
}

func (c ReadSocketClient) ReadReview(ctx context.Context, request ReadReviewRequest) (SocketResult[ReadReviewResponse], error) {
	return sendSocketRead[ReadReviewRequest, ReadReviewResponse](ctx, c.review, request)
}

func (c ReadSocketClient) ReadEvents(ctx context.Context, request ReadEventsRequest) (SocketResult[ReadEventsResponse], error) {
	return sendSocketRead[ReadEventsRequest, ReadEventsResponse](ctx, c.events, request)
}

func (c ReadSocketClient) ReadProjection(ctx context.Context, request ReadProjectionRequest) (SocketResult[ReadProjectionResponse], error) {
	return sendSocketRead[ReadProjectionRequest, ReadProjectionResponse](ctx, c.projection, request)
}

func sendSocketRead[Request core.ValidatedJSONMarshaler, Response core.Validatable](ctx context.Context, socket exchange.ClientSocket, request Request) (SocketResult[Response], error) {
	response, err := exchange.SendSocketJSON[Request, Response](ctx, socket, request)
	if err != nil {
		return SocketResult[Response]{}, contractError(err)
	}
	return newSocketResult(response)
}

func newSocketResult[Response core.Validatable](response exchange.JSONResponse[Response]) (SocketResult[Response], error) {
	result := SocketResult[Response]{Response: response.Body, Metadata: response.Metadata}
	return result, result.Validate()
}

func bindReceiptRequest(request controlwire.RequestNonce, document proofledger.ReceiptDocument) error {
	if err := errors.Join(request.Validate(), document.Validate()); err != nil {
		return contractError(err)
	}
	if document.Receipt.Request != request {
		return errors.Join(core.ErrProofLedgerReceiptMismatch, contractError())
	}
	return nil
}

type IssueSocketServer struct {
	socket  exchange.ServerSocket
	service ReviewIssuer
}

type ReviewReadSocketServer struct {
	socket  exchange.ServerSocket
	service ReviewReader
}

type ObservationSocketServer struct {
	socket  exchange.ServerSocket
	service ObservationRecorder
}

type DecisionSocketServer struct {
	socket  exchange.ServerSocket
	service HumanDecisionRecorder
}

type EventReadSocketServer struct {
	socket  exchange.ServerSocket
	service EventReader
}

type ProjectionReadSocketServer struct {
	socket  exchange.ServerSocket
	service ProjectionReader
}

func NewIssueSocketServer(path exchange.SocketRoutePath, service ReviewIssuer) (IssueSocketServer, error) {
	socket, err := newOperationServer(OperationIssueReview, path, service != nil)
	return IssueSocketServer{socket: socket, service: service}, err
}

func NewReviewReadSocketServer(path exchange.SocketRoutePath, service ReviewReader) (ReviewReadSocketServer, error) {
	socket, err := newOperationServer(OperationReadReview, path, service != nil)
	return ReviewReadSocketServer{socket: socket, service: service}, err
}

func NewObservationSocketServer(path exchange.SocketRoutePath, service ObservationRecorder) (ObservationSocketServer, error) {
	socket, err := newOperationServer(OperationRecordObservation, path, service != nil)
	return ObservationSocketServer{socket: socket, service: service}, err
}

func NewDecisionSocketServer(path exchange.SocketRoutePath, service HumanDecisionRecorder) (DecisionSocketServer, error) {
	socket, err := newOperationServer(OperationRecordDecision, path, service != nil)
	return DecisionSocketServer{socket: socket, service: service}, err
}

func NewEventReadSocketServer(path exchange.SocketRoutePath, service EventReader) (EventReadSocketServer, error) {
	socket, err := newOperationServer(OperationReadEvents, path, service != nil)
	return EventReadSocketServer{socket: socket, service: service}, err
}

func NewProjectionReadSocketServer(path exchange.SocketRoutePath, service ProjectionReader) (ProjectionReadSocketServer, error) {
	socket, err := newOperationServer(OperationReadProjection, path, service != nil)
	return ProjectionReadSocketServer{socket: socket, service: service}, err
}

func newOperationServer(operation Operation, path exchange.SocketRoutePath, servicePresent bool) (exchange.ServerSocket, error) {
	if !servicePresent {
		return exchange.ServerSocket{}, contractError(errors.New("review control server service is absent"))
	}
	contract, err := SocketContract(operation, path)
	if err != nil {
		return exchange.ServerSocket{}, err
	}
	socket, err := exchange.NewServerSocket(contract)
	if err != nil {
		return exchange.ServerSocket{}, contractError(err)
	}
	return socket, nil
}

func (s IssueSocketServer) Serve(call exchange.SocketServerCall) error {
	return serveMutation[IssueReviewRequest, *IssueReviewRequest, IssueReviewResponse](s.socket, call, s.service.IssueReview)
}

func (s ReviewReadSocketServer) Serve(call exchange.SocketServerCall) error {
	return serveRead[ReadReviewRequest, *ReadReviewRequest, ReadReviewResponse](s.socket, call, s.service.ReadReview)
}

func (s ObservationSocketServer) Serve(call exchange.SocketServerCall) error {
	return serveMutation[RecordObservationRequest, *RecordObservationRequest, RecordObservationResponse](s.socket, call, s.service.RecordObservation)
}

func (s DecisionSocketServer) Serve(call exchange.SocketServerCall, authority VerifiedHumanAuthority) error {
	if err := authority.Validate(); err != nil {
		return err
	}
	return serveMutation[RecordDecisionRequest, *RecordDecisionRequest, RecordDecisionResponse](s.socket, call, func(ctx context.Context, body RecordDecisionRequest) (RecordDecisionResponse, error) {
		return s.service.RecordDecision(ctx, authority, body)
	})
}

func (s EventReadSocketServer) Serve(call exchange.SocketServerCall) error {
	return serveRead[ReadEventsRequest, *ReadEventsRequest, ReadEventsResponse](s.socket, call, s.service.ReadEvents)
}

func (s ProjectionReadSocketServer) Serve(call exchange.SocketServerCall) error {
	return serveRead[ReadProjectionRequest, *ReadProjectionRequest, ReadProjectionResponse](s.socket, call, s.service.ReadProjection)
}

func serveRead[Request any, RequestPtr interface {
	*Request
	core.Validatable
}, Response core.ValidatedJSONMarshaler](socket exchange.ServerSocket, call exchange.SocketServerCall, service func(context.Context, Request) (Response, error)) error {
	received, err := exchange.ReceiveSocketJSON[Request, RequestPtr](socket, call)
	if err != nil {
		return contractError(err)
	}
	ctx, err := call.Context()
	if err != nil {
		return contractError(err)
	}
	response, err := service(ctx, *received.Body)
	if err != nil {
		return err
	}
	if err := exchange.WriteSocketJSON(socket, call, response); err != nil {
		return contractError(err)
	}
	return nil
}

func serveMutation[Request any, RequestPtr interface {
	*Request
	exchange.IdempotencyBound
}, Response core.ValidatedJSONMarshaler](socket exchange.ServerSocket, call exchange.SocketServerCall, service func(context.Context, Request) (Response, error)) error {
	received, err := exchange.ReceiveReplayBoundSocketJSON[Request, RequestPtr](socket, call)
	if err != nil {
		return contractError(err)
	}
	ctx, err := call.Context()
	if err != nil {
		return contractError(err)
	}
	response, err := service(ctx, *received.Body)
	if err != nil {
		return err
	}
	if err := exchange.WriteSocketJSON(socket, call, response); err != nil {
		return contractError(err)
	}
	return nil
}

func sameSocketContract(left, right exchange.JSONSocketContract) bool {
	return left.Path.String() == right.Path.String() && left.Route == right.Route && left.RequestBodyLimit == right.RequestBodyLimit && left.ResponseBodyLimit == right.ResponseBodyLimit && left.SuccessStatus == right.SuccessStatus
}
