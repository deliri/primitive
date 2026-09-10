package controlwire_test

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// The embedded real request supplies validated typed facts. Only the emitting
// capability is broken by each row.
type replayEmissionProbe struct {
	controlplane.RegistrationRequest
	wire       []byte
	marshalErr error
}

func (p replayEmissionProbe) MarshalJSON() ([]byte, error) { return p.wire, p.marshalErr }

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
		name       string
		wire       []byte
		errorCause error
		wantErr    error
	}{
		{name: "canonical producer retains every request fact", wire: canonical},
		{name: "neutral absent marshal output cannot create a replay identity", wantErr: core.ErrControlWireContract},
		{name: "negative JSON null is not a request document", wire: []byte("null"), wantErr: core.ErrControlWireContract},
		{name: "negative scalar is not a struct document", wire: []byte("1"), wantErr: core.ErrControlWireContract},
		{name: "negative truncated object cannot earn a commitment", wire: canonical[:len(canonical)-1], wantErr: core.ErrControlWireContract},
		{name: "negative trailing second document is not one request", wire: append(bytes.Clone(canonical), canonical...), wantErr: core.ErrControlWireContract},
		{name: "negative marshaler failure keeps its cause and yields no proof", wire: canonical, errorCause: core.ErrSecretMaterialAllZero, wantErr: core.ErrSecretMaterialAllZero},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			input := replayEmissionProbe{RegistrationRequest: fixture.request, wire: tc.wire, marshalErr: tc.errorCause}
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
