package controlwire_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// The embedded real request supplies validated typed facts. Only the emitting
// capability or its declared byte budget is broken by each row.
type replayEmissionProbe struct {
	controlplane.RegistrationRequest
	wire       []byte
	maximum    core.ByteCount
	limitErr   error
	marshalErr error
}

func (p replayEmissionProbe) MarshalJSON() ([]byte, error) { return p.wire, p.marshalErr }
func (p replayEmissionProbe) ControlRequestBodyLimit() (core.ByteCount, error) {
	return p.maximum, p.limitErr
}

func TestReplayCommitmentEmissionLayerTriad(t *testing.T) {
	t.Parallel()
	fixture := productionSocketFixture(t)
	canonical, err := fixture.request.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := controlwire.CommitReplayIdentity(fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name                 string
		wire                 []byte
		maximum              uint64
		limitErr, errorCause error
		wantErr              error
	}{
		{name: "positive exact body limit retains every request fact", wire: canonical, maximum: uint64(len(canonical))},
		{name: "positive one byte of unused budget cannot change commitment", wire: canonical, maximum: uint64(len(canonical) + 1)},
		{name: "negative one byte beyond request limit cannot earn a commitment", wire: canonical, maximum: uint64(len(canonical) - 1), wantErr: core.ErrControlWireContract},
		{name: "neutral absent marshal output cannot create a replay identity", maximum: 1, wantErr: core.ErrControlWireContract},
		{name: "negative JSON null is not a request document", wire: []byte("null"), maximum: 4, wantErr: core.ErrControlWireContract},
		{name: "negative scalar is not a struct document", wire: []byte("1"), maximum: 1, wantErr: core.ErrControlWireContract},
		{name: "negative truncated object cannot earn a commitment", wire: canonical[:len(canonical)-1], maximum: uint64(len(canonical)), wantErr: core.ErrControlWireContract},
		{name: "negative trailing second document is not one request", wire: append(bytes.Clone(canonical), canonical...), maximum: uint64(2 * len(canonical)), wantErr: core.ErrControlWireContract},
		{name: "negative zero byte budget cannot be ignored", wire: canonical, wantErr: core.ErrControlWireContract},
		{name: "negative budget cannot exceed the shared wire ceiling", wire: canonical, maximum: core.JSONDocumentMaximumBytes + 1, wantErr: core.ErrControlWireContract},
		{name: "negative budget provider failure keeps its cause", wire: canonical, maximum: uint64(len(canonical)), limitErr: core.ErrContextObservation, wantErr: core.ErrContextObservation},
		{name: "negative marshaler failure keeps its cause and yields no proof", wire: canonical, maximum: uint64(len(canonical)), errorCause: core.ErrSecretMaterialAllZero, wantErr: core.ErrSecretMaterialAllZero},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var maximum core.ByteCount
			if tc.maximum != 0 {
				var countErr error
				maximum, countErr = core.NewByteCount(tc.maximum)
				if countErr != nil {
					t.Fatal(countErr)
				}
			}
			input := replayEmissionProbe{RegistrationRequest: fixture.request, wire: tc.wire, maximum: maximum, limitErr: tc.limitErr, marshalErr: tc.errorCause}
			before := bytes.Clone(tc.wire)
			got, gotErr := controlwire.CommitReplayIdentity(input)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("commit error=%v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (controlwire.ReplayIdentity{}) || !errors.Is(gotErr, core.ErrControlWireContract) {
					t.Fatalf("refused commit=%v/%v, want zero and control-wire identity", got, gotErr)
				}
			} else if got != baseline {
				t.Fatalf("commit=%v, want exact baseline %v", got, baseline)
			}
			if !bytes.Equal(tc.wire, before) {
				t.Fatalf("marshaler-owned bytes=%q, want %q", tc.wire, before)
			}
		})
	}
}

func TestRouteCapabilityRefusalDoesNotLeakPartialFacts(t *testing.T) {
	t.Parallel()
	route, err := controlwire.NewRouteContract(controlwireExternalOfferingFixture(t, 1), controlwire.RouteFamilyRegistrations)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name     string
		revision controlwire.Revision
		wantErr  error
	}{
		{name: "published revision binds exact pair", revision: controlwire.Revision2026V1},
		{name: "unset revision yields no partial capability", revision: controlwire.RevisionUnknown, wantErr: core.ErrControlWireProtocolSupport},
		{name: "future revision yields no partial capability", revision: controlwire.Revision2026V1 + 1, wantErr: core.ErrControlWireProtocolSupport},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := route.ProtocolCapability(tc.revision)
			want := controlwire.ProtocolCapability{}
			if tc.wantErr == nil {
				want = controlwire.ProtocolCapability{Revision: tc.revision, Family: route.Family()}
			}
			if got != want || !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("capability=%v/%v, want %v/%v", got, gotErr, want, tc.wantErr)
			}
		})
	}
}

// Client fixture retains the real request's paired marshal/unmarshal contract.
type clientBudgetProbe struct {
	controlplane.RegistrationRequest
	maximum  core.ByteCount
	limitErr error
}

func (p clientBudgetProbe) ControlRequestBodyLimit() (core.ByteCount, error) {
	return p.maximum, p.limitErr
}

func TestClientRequestOwnedByteBudgetBeforeHTTP(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		delta     int
		zero      bool
		limitErr  error
		wantErr   error
		wantCalls int32
		wantCause error
	}{
		{name: "positive exact owned budget reaches authority", wantCalls: 1},
		{name: "negative first byte beyond owned budget never reaches authority", delta: -1, wantErr: core.ErrExchangeRequest, wantCause: core.ErrJSONContract},
		{name: "negative absent budget never reaches authority", zero: true, wantErr: core.ErrControlWireContract},
		{name: "negative budget provider failure remains typed", limitErr: core.ErrContextObservation, wantErr: core.ErrContextObservation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fixture := productionSocketFixture(t)
			canonical, err := fixture.request.MarshalJSON()
			if err != nil {
				t.Fatalf("fixture marshal error=%v, want nil", err)
			}
			maximum, err := core.NewByteCount(uint64(len(canonical) + tc.delta))
			if err != nil {
				t.Fatalf("fixture budget error=%v, want nil", err)
			}
			if tc.zero {
				maximum = core.ByteCount{}
			}
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set(core.HTTPHeaderContentType().String(), standardMediaType(t, exchange.StandardMediaTypeJSON).String())
				_, _ = w.Write(fixture.responseCanonical)
			}))
			defer server.Close()
			endpoint, err := core.ParseHTTPEndpoint(server.URL)
			if err != nil {
				t.Fatalf("endpoint error=%v, want nil", err)
			}
			transport, err := exchange.NewClient(server.Client())
			if err != nil {
				t.Fatalf("transport error=%v, want nil", err)
			}
			body := clientBudgetProbe{RegistrationRequest: fixture.request, maximum: maximum, limitErr: tc.limitErr}
			got, gotErr := controlwire.SendRoutedJSON[clientBudgetProbe, controlplane.ResponseDocument[controlplane.RegistrationDocument, *controlplane.RegistrationDocument], *controlplane.ResponseDocument[controlplane.RegistrationDocument, *controlplane.RegistrationDocument]](controlwire.ClientJSONCall[clientBudgetProbe]{Context: context.Background(), Body: body, Client: socketClient(t, transport, endpoint)})
			if !errors.Is(gotErr, tc.wantErr) || calls.Load() != tc.wantCalls || (tc.wantCause != nil && !errors.Is(gotErr, tc.wantCause)) {
				t.Fatalf("send error=%v calls=%d, want %v/%v and %d calls", gotErr, calls.Load(), tc.wantErr, tc.wantCause, tc.wantCalls)
			}
			if tc.wantErr != nil && got.Body != nil {
				t.Fatalf("refused response=%v, want nil", got.Body)
			}
		})
	}
}

// This broken producer changes a contract fact between ingress and the final
// output validation. The socket must return no usable partial result.
type changingRevisionRequest struct {
	controlplane.RegistrationRequest
	projections int
}

func (r *changingRevisionRequest) ControlRevision() controlwire.Revision {
	r.projections++
	if r.projections >= 3 {
		return controlwire.RevisionUnknown
	}
	return r.RegistrationRequest.ControlRevision()
}

func TestReceiveFinalValidationDoesNotLeakPartialOutput(t *testing.T) {
	t.Parallel()
	fixture := productionSocketFixture(t)
	route, err := fixture.request.ControlRoute()
	if err != nil {
		t.Fatalf("route error=%v, want nil", err)
	}
	path, err := route.Path()
	if err != nil {
		t.Fatalf("path error=%v, want nil", err)
	}
	wire, err := fixture.request.MarshalJSON()
	if err != nil {
		t.Fatalf("fixture encoding error=%v, want nil", err)
	}
	for _, tc := range []struct {
		name     string
		unstable bool
		wantErr  error
	}{
		{name: "positive stable producer survives final binding"},
		{name: "negative changed revision cannot leak received facts", unstable: true, wantErr: core.ErrControlWireProtocolSupport},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(wire))
			request.Header.Set(core.HTTPHeaderContentType().String(), standardMediaType(t, exchange.StandardMediaTypeJSON).String())
			request.Header.Set(core.HTTPHeaderIdempotencyKey().String(), fixture.request.RequestNonce.String())
			call := controlwire.AuthorityJSONReceiveCall{Call: fixtureSocketServerCall(t, request), Route: route, Authority: socketServer(t, fixture.support)}
			if tc.unstable {
				got, err := controlwire.ReceiveRoutedJSON[changingRevisionRequest, *changingRevisionRequest](call)
				if got.Body != nil {
					defer func() { _ = got.Body.Token.Destroy() }()
				}
				if !errors.Is(err, tc.wantErr) || got != (controlwire.RoutedJSONReceive[*changingRevisionRequest]{}) {
					t.Fatalf("refused receive=%v/%v, want zero/%v", got, err, tc.wantErr)
				}
				return
			}
			got, err := controlwire.ReceiveRoutedJSON[controlplane.RegistrationRequest, *controlplane.RegistrationRequest](call)
			if err != nil || got.Body == nil || got.Validate() != nil {
				t.Fatalf("receive=%v/%v, want valid complete result", got, err)
			}
			defer func() { _ = got.Body.Token.Destroy() }()
		})
	}
}
