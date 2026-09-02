package reviewcontrol

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/proofledger"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	reviewIssueTestPath       = "/review/issue"
	reviewReadTestPath        = "/review/read"
	reviewObservationTestPath = "/review/observation"
	reviewDecisionTestPath    = "/review/decision"
	reviewEventsTestPath      = "/review/events"
	reviewProjectionTestPath  = "/review/projection"
)

type socketServiceFixture struct {
	issue       IssueReviewResponse
	review      ReadReviewResponse
	observation RecordObservationResponse
	decision    RecordDecisionResponse
	events      ReadEventsResponse
	projection  ReadProjectionResponse
	calls       atomic.Uint64
}

func (s *socketServiceFixture) IssueReview(_ context.Context, request IssueReviewRequest) (IssueReviewResponse, error) {
	s.calls.Add(1)
	if err := request.Validate(); err != nil {
		return IssueReviewResponse{}, err
	}
	return s.issue, nil
}

func (s *socketServiceFixture) ReadReview(_ context.Context, request ReadReviewRequest) (ReadReviewResponse, error) {
	s.calls.Add(1)
	if err := request.Validate(); err != nil {
		return ReadReviewResponse{}, err
	}
	return s.review, nil
}

func (s *socketServiceFixture) RecordObservation(_ context.Context, request RecordObservationRequest) (RecordObservationResponse, error) {
	s.calls.Add(1)
	if err := request.Validate(); err != nil {
		return RecordObservationResponse{}, err
	}
	return s.observation, nil
}

func (s *socketServiceFixture) RecordDecision(_ context.Context, authority VerifiedHumanAuthority, request RecordDecisionRequest) (RecordDecisionResponse, error) {
	s.calls.Add(1)
	if err := errors.Join(authority.Validate(), request.Validate()); err != nil {
		return RecordDecisionResponse{}, err
	}
	return s.decision, nil
}

func (s *socketServiceFixture) ReadEvents(_ context.Context, request ReadEventsRequest) (ReadEventsResponse, error) {
	s.calls.Add(1)
	if err := request.Validate(); err != nil {
		return ReadEventsResponse{}, err
	}
	return s.events, nil
}

func (s *socketServiceFixture) ReadProjection(_ context.Context, request ReadProjectionRequest) (ReadProjectionResponse, error) {
	s.calls.Add(1)
	if err := request.Validate(); err != nil {
		return ReadProjectionResponse{}, err
	}
	return s.projection, nil
}

func TestReviewControlClientServerLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive all six operations cross the real paired HTTP sockets", func(t *testing.T) {
		t.Parallel()
		fixtures := newSocketFixtures(t)
		service := &socketServiceFixture{
			issue: fixtures.issueResponse, review: fixtures.readResponse,
			observation: fixtures.observationResponse, decision: fixtures.decisionResponse,
			events: fixtures.eventsResponse, projection: fixtures.projectionResponse,
		}
		authority := verifiedHuman(t, AuthorityHuman)
		httpServer := newReviewSocketHTTPServer(t, service, authority)
		defer httpServer.Close()
		issuer, observations, decisions, reads := newReviewSocketClients(t, httpServer)

		issueResult, issueErr := issuer.IssueReview(context.Background(), fixtures.issueRequest)
		observationResult, observationErr := observations.RecordObservation(context.Background(), fixtures.observationRequest)
		decisionResult, decisionErr := decisions.RecordDecision(context.Background(), fixtures.decisionRequest)
		reviewResult, reviewErr := reads.ReadReview(context.Background(), fixtures.readRequest)
		eventsResult, eventsErr := reads.ReadEvents(context.Background(), fixtures.eventsRequest)
		projectionResult, projectionErr := reads.ReadProjection(context.Background(), fixtures.projectionRequest)
		gotErr := errors.Join(issueErr, observationErr, decisionErr, reviewErr, eventsErr, projectionErr)
		if gotErr != nil {
			t.Fatalf("six socket operations error = %v, want nil", gotErr)
		}
		if issueResult.Response != fixtures.issueResponse || observationResult.Response != fixtures.observationResponse ||
			decisionResult.Response != fixtures.decisionResponse || reviewResult.Response.Packet.Identity != fixtures.readResponse.Packet.Identity ||
			len(eventsResult.Response.Page.Events) != 1 || eventsResult.Response.Page.Events[0].Hash != fixtures.eventsResponse.Page.Events[0].Hash ||
			projectionResult.Response.Projection.Review != fixtures.projectionResponse.Projection.Review {
			t.Fatalf("six socket response facts differ from shared documents")
		}
		if issueResult.Metadata.Attempts != 1 || observationResult.Metadata.Attempts != 1 || decisionResult.Metadata.Attempts != 1 ||
			reviewResult.Metadata.Attempts != 1 || eventsResult.Metadata.Attempts != 1 || projectionResult.Metadata.Attempts != 1 || service.calls.Load() != 6 {
			t.Fatalf("six socket execution accounting = (attempts %d/%d/%d/%d/%d/%d, calls=%d), want all 1 and calls=6",
				issueResult.Metadata.Attempts, observationResult.Metadata.Attempts, decisionResult.Metadata.Attempts,
				reviewResult.Metadata.Attempts, eventsResult.Metadata.Attempts, projectionResult.Metadata.Attempts, service.calls.Load())
		}
	})

	t.Run("negative response receipt for another request is refused by the client", func(t *testing.T) {
		t.Parallel()
		fixtures := newSocketFixtures(t)
		service := &socketServiceFixture{decision: fixtures.decisionResponse}
		service.decision.Receipt = reviewReceiptForRequest(t, fixtures.readResponse.Packet, reviewNonce(t, 99))
		httpServer := newReviewDecisionHTTPServer(t, service, verifiedHuman(t, AuthorityHuman))
		defer httpServer.Close()
		client := newDecisionSocketClient(t, httpServer)
		got, gotErr := client.RecordDecision(context.Background(), fixtures.decisionRequest)
		gotIsZero := got.Response.Receipt.Receipt.Sequence == 0 && got.Metadata.Attempts == 0 && got.Metadata.Bytes.Uint64() == 0
		if !errors.Is(gotErr, core.ErrProofLedgerReceiptMismatch) || !gotIsZero || service.calls.Load() != 1 {
			t.Fatalf("DecisionSocketClient.RecordDecision(mismatched receipt) = (%+v, calls=%d, %v), want (zero, 1, %v)", got, service.calls.Load(), gotErr, core.ErrProofLedgerReceiptMismatch)
		}
	})

	t.Run("neutral cancelled decision reaches no server service", func(t *testing.T) {
		t.Parallel()
		fixtures := newSocketFixtures(t)
		service := &socketServiceFixture{decision: fixtures.decisionResponse}
		httpServer := newReviewDecisionHTTPServer(t, service, verifiedHuman(t, AuthorityHuman))
		defer httpServer.Close()
		client := newDecisionSocketClient(t, httpServer)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		got, gotErr := client.RecordDecision(ctx, fixtures.decisionRequest)
		gotIsZero := got.Response.Receipt.Receipt.Sequence == 0 && got.Metadata.Attempts == 0 && got.Metadata.Bytes.Uint64() == 0
		if !errors.Is(gotErr, context.Canceled) || !gotIsZero || service.calls.Load() != 0 {
			t.Fatalf("DecisionSocketClient.RecordDecision(cancelled) = (%+v, calls=%d, %v), want (zero, 0, %v)", got, service.calls.Load(), gotErr, context.Canceled)
		}
	})
}

func reviewReceiptForRequest(t testing.TB, packet Packet, request controlwire.RequestNonce) proofledger.ReceiptDocument {
	t.Helper()
	genesis, err := proofledger.NewGenesisHead(proofLedgerIdentity(t))
	if err != nil {
		t.Fatalf("NewGenesisHead() error = %v, want nil", err)
	}
	_, actor := reviewKey(t, 2)
	event, err := proofledger.NewEnvelope(proofledger.Issue[EventPayload]{
		Intent: proofledger.AppendIntent[EventPayload]{Request: request, Ledger: genesis.Ledger, ExpectedHead: genesis, Actor: actor, Payload: EventPayload{Kind: EventReviewIssued, Review: &packet}},
		Event:  proofEventIdentity(t, 0), RecordedAt: reviewInstant(t, 0),
	})
	if err != nil {
		t.Fatalf("NewEnvelope(receipt event) error = %v, want nil", err)
	}
	signer, producer := reviewKey(t, 3)
	document, err := proofledger.IssueReceipt(proofledger.ReceiptIssuance[EventPayload]{Event: event, Producer: producer, Signer: signer})
	if err != nil {
		t.Fatalf("IssueReceipt() error = %v, want nil", err)
	}
	return document
}

func newReviewSocketHTTPServer(t testing.TB, service *socketServiceFixture, authority VerifiedHumanAuthority) *httptest.Server {
	t.Helper()
	issue := newIssueServerForTest(t, service)
	review := newReviewReadServerForTest(t, service)
	observation := newObservationServerForTest(t, service)
	decision := newDecisionServerForTest(t, service)
	events := newEventReadServerForTest(t, service)
	projection := newProjectionReadServerForTest(t, service)
	mux := http.NewServeMux()
	mux.HandleFunc(reviewIssueTestPath, func(writer http.ResponseWriter, request *http.Request) {
		serveReviewSocketCall(writer, request, issue.Serve)
	})
	mux.HandleFunc(reviewReadTestPath, func(writer http.ResponseWriter, request *http.Request) {
		serveReviewSocketCall(writer, request, review.Serve)
	})
	mux.HandleFunc(reviewObservationTestPath, func(writer http.ResponseWriter, request *http.Request) {
		serveReviewSocketCall(writer, request, observation.Serve)
	})
	mux.HandleFunc(reviewDecisionTestPath, func(writer http.ResponseWriter, request *http.Request) {
		serveReviewSocketCall(writer, request, func(call exchange.SocketServerCall) error { return decision.Serve(call, authority) })
	})
	mux.HandleFunc(reviewEventsTestPath, func(writer http.ResponseWriter, request *http.Request) {
		serveReviewSocketCall(writer, request, events.Serve)
	})
	mux.HandleFunc(reviewProjectionTestPath, func(writer http.ResponseWriter, request *http.Request) {
		serveReviewSocketCall(writer, request, projection.Serve)
	})
	return httptest.NewServer(mux)
}

func newReviewDecisionHTTPServer(t testing.TB, service *socketServiceFixture, authority VerifiedHumanAuthority) *httptest.Server {
	t.Helper()
	server := newDecisionServerForTest(t, service)
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		serveReviewSocketCall(writer, request, func(call exchange.SocketServerCall) error { return server.Serve(call, authority) })
	}))
}

func serveReviewSocketCall(writer http.ResponseWriter, request *http.Request, serve func(exchange.SocketServerCall) error) {
	call, err := exchange.NewSocketServerCall(writer, request)
	if err == nil {
		err = serve(call)
	}
	serveReviewTestResponse(writer, err)
}

func serveReviewTestResponse(writer http.ResponseWriter, err error) {
	if err != nil {
		http.Error(writer, "review request refused", http.StatusInternalServerError)
	}
}

func reviewSocketPath(t testing.TB, value string) exchange.SocketRoutePath {
	t.Helper()
	got, err := exchange.ParseSocketRoutePath(value)
	if err != nil {
		t.Fatalf("ParseSocketRoutePath(%q) error = %v, want nil", value, err)
	}
	return got
}

func newIssueServerForTest(t testing.TB, service ReviewIssuer) IssueSocketServer {
	t.Helper()
	got, err := NewIssueSocketServer(reviewSocketPath(t, reviewIssueTestPath), service)
	if err != nil {
		t.Fatalf("NewIssueSocketServer() error = %v, want nil", err)
	}
	return got
}

func newReviewReadServerForTest(t testing.TB, service ReviewReader) ReviewReadSocketServer {
	t.Helper()
	got, err := NewReviewReadSocketServer(reviewSocketPath(t, reviewReadTestPath), service)
	if err != nil {
		t.Fatalf("NewReviewReadSocketServer() error = %v, want nil", err)
	}
	return got
}

func newObservationServerForTest(t testing.TB, service ObservationRecorder) ObservationSocketServer {
	t.Helper()
	got, err := NewObservationSocketServer(reviewSocketPath(t, reviewObservationTestPath), service)
	if err != nil {
		t.Fatalf("NewObservationSocketServer() error = %v, want nil", err)
	}
	return got
}

func newDecisionServerForTest(t testing.TB, service HumanDecisionRecorder) DecisionSocketServer {
	t.Helper()
	got, err := NewDecisionSocketServer(reviewSocketPath(t, reviewDecisionTestPath), service)
	if err != nil {
		t.Fatalf("NewDecisionSocketServer() error = %v, want nil", err)
	}
	return got
}

func newEventReadServerForTest(t testing.TB, service EventReader) EventReadSocketServer {
	t.Helper()
	got, err := NewEventReadSocketServer(reviewSocketPath(t, reviewEventsTestPath), service)
	if err != nil {
		t.Fatalf("NewEventReadSocketServer() error = %v, want nil", err)
	}
	return got
}

func newProjectionReadServerForTest(t testing.TB, service ProjectionReader) ProjectionReadSocketServer {
	t.Helper()
	got, err := NewProjectionReadSocketServer(reviewSocketPath(t, reviewProjectionTestPath), service)
	if err != nil {
		t.Fatalf("NewProjectionReadSocketServer() error = %v, want nil", err)
	}
	return got
}

func newReviewSocketClients(t testing.TB, server *httptest.Server) (IssuerSocketClient, ObservationSocketClient, DecisionSocketClient, ReadSocketClient) {
	t.Helper()
	issueConfiguration := reviewClientConfiguration(t, server, OperationIssueReview, reviewIssueTestPath)
	observationConfiguration := reviewClientConfiguration(t, server, OperationRecordObservation, reviewObservationTestPath)
	decisionConfiguration := reviewClientConfiguration(t, server, OperationRecordDecision, reviewDecisionTestPath)
	issuer, issueErr := NewIssuerSocketClient(issueConfiguration)
	observations, observationErr := NewObservationSocketClient(observationConfiguration)
	decisions, decisionErr := NewDecisionSocketClient(decisionConfiguration)
	reads, readErr := NewReadSocketClient(ReadSocketClientConfiguration{
		Review:     reviewClientConfiguration(t, server, OperationReadReview, reviewReadTestPath),
		Events:     reviewClientConfiguration(t, server, OperationReadEvents, reviewEventsTestPath),
		Projection: reviewClientConfiguration(t, server, OperationReadProjection, reviewProjectionTestPath),
	})
	if err := errors.Join(issueErr, observationErr, decisionErr, readErr); err != nil {
		t.Fatalf("review socket client constructors error = %v, want nil", err)
	}
	return issuer, observations, decisions, reads
}

func newDecisionSocketClient(t testing.TB, server *httptest.Server) DecisionSocketClient {
	t.Helper()
	got, err := NewDecisionSocketClient(reviewClientConfiguration(t, server, OperationRecordDecision, reviewDecisionTestPath))
	if err != nil {
		t.Fatalf("NewDecisionSocketClient() error = %v, want nil", err)
	}
	return got
}

func reviewClientConfiguration(t testing.TB, server *httptest.Server, operation Operation, pathValue string) exchange.ClientSocketConfiguration {
	t.Helper()
	client, err := exchange.NewClient(server.Client())
	if err != nil {
		t.Fatalf("exchange.NewClient() error = %v, want nil", err)
	}
	path := reviewSocketPath(t, pathValue)
	target, err := core.ParseHTTPEndpoint(server.URL + path.String())
	if err != nil {
		t.Fatalf("ParseHTTPEndpoint() error = %v, want nil", err)
	}
	contract, err := SocketContract(operation, path)
	if err != nil {
		t.Fatalf("SocketContract() error = %v, want nil", err)
	}
	operationTimeout, err := temporal.DurationFromMilliseconds(60_000)
	if err != nil {
		t.Fatalf("DurationFromMilliseconds(operation) error = %v, want nil", err)
	}
	attemptTimeout, err := temporal.DurationFromMilliseconds(30_000)
	if err != nil {
		t.Fatalf("DurationFromMilliseconds(attempt) error = %v, want nil", err)
	}
	return exchange.ClientSocketConfiguration{
		Client: client, Target: target, Contract: contract,
		Operation: exchange.OperationPolicy{OperationTimeout: operationTimeout, AttemptTimeout: attemptTimeout, Retry: exchange.RetryPolicy{MaximumAttempts: 1}, Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}},
	}
}
