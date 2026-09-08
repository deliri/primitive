package controlwire_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func BenchmarkParseRequestNonce(b *testing.B) {
	nonce, err := controlwire.NewRequestNonce([core.SHA256DigestBytes]byte{1})
	if err != nil {
		b.Fatal(err)
	}
	text := nonce.String()
	if len(text) != hex.EncodedLen(core.SHA256DigestBytes) {
		b.Fatalf("nonce text bytes=%d, want exact digest width", len(text))
	}
	var got controlwire.RequestNonce
	b.ReportAllocs()
	for b.Loop() {
		got, err = controlwire.ParseRequestNonce(text)
	}
	if err != nil || got != nonce {
		b.Fatalf("ParseRequestNonce=%v/%v, want exact fixture", got, err)
	}
}

func BenchmarkRequestNonceJSON(b *testing.B) {
	b.ReportAllocs()
	nonce, err := controlwire.NewRequestNonce([core.SHA256DigestBytes]byte{1})
	if err != nil {
		b.Fatal(err)
	}
	canonical, err := nonce.MarshalJSON()
	if err != nil || len(canonical) == 0 {
		b.Fatalf("nonce JSON=%d bytes/%v, want nonempty fixture", len(canonical), err)
	}
	oversized, err := core.MarshalCanonicalJSONString(string(bytes.Repeat([]byte{'a'}, core.JSONDocumentMaximumBytes+1)))
	if err != nil {
		b.Fatal(err)
	}
	cases := []struct {
		name    string
		data    []byte
		wantErr error
	}{
		{name: "Canonical", data: canonical},
		{name: "Oversized", data: oversized, wantErr: core.ErrControlWireNonce},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			got := nonce
			b.ReportAllocs()
			for b.Loop() {
				err = got.UnmarshalJSON(tc.data)
			}
			if !errors.Is(err, tc.wantErr) || got != nonce {
				b.Fatalf("nonce decode=%v/%v, want retained fixture and %v", got, err, tc.wantErr)
			}
		})
	}
}

// Parsing, deriving the persisted verifier, and destroying the owned secret
// is one complete token-consumption workload. No secret survives an iteration.
func BenchmarkRegistrationTokenLifecycle(b *testing.B) {
	raw := [controlwire.RegistrationTokenBytes]byte{1}
	input := []byte(hex.EncodeToString(raw[:]))
	want := core.SHA256Of(raw[:])
	wantText, err := want.Hex()
	if err != nil {
		b.Fatal(err)
	}
	var got controlwire.RegistrationTokenVerifier
	b.ReportAllocs()
	for b.Loop() {
		token, parseErr := controlwire.ParseRegistrationToken(input)
		if parseErr != nil {
			b.Fatal(parseErr)
		}
		got, err = token.Verifier()
		destroyErr := token.Destroy()
		if err != nil || destroyErr != nil {
			b.Fatalf("token lifecycle=%v/%v, want nil", err, destroyErr)
		}
	}
	if got.String() != wantText {
		b.Fatalf("verifier=%v, want exact Go SHA256 digest", got)
	}
}

func BenchmarkPolicyCursorRoundTrip(b *testing.B) {
	activation, err := controlwire.NewPolicyActivation(1)
	if err != nil {
		b.Fatal(err)
	}
	want := controlwire.PolicyCursor{Revision: controlwire.PolicyRevisionID{15: 1}, Activation: activation}
	canonical, err := want.MarshalJSON()
	if err != nil || len(canonical) == 0 {
		b.Fatalf("cursor seed=%d/%v, want nonempty JSON", len(canonical), err)
	}
	var got controlwire.PolicyCursor
	var encoded []byte
	b.ReportAllocs()
	for b.Loop() {
		if err = got.UnmarshalJSON(canonical); err != nil {
			b.Fatal(err)
		}
		encoded, err = got.MarshalJSON()
	}
	if err != nil || got != want || !bytes.Equal(encoded, canonical) {
		b.Fatalf("cursor round trip=%v/%v, want exact fixture and canonical bytes", got, err)
	}
}

func BenchmarkProtocolSupport(b *testing.B) {
	b.ReportAllocs()
	families := protocolFamilyInventory()
	capabilities := protocolCapabilities(families[:])
	slices.Reverse(capabilities)
	support, err := controlwire.NewProtocolSupport(controlwire.ProtocolSupportRequest{Capabilities: capabilities})
	if err != nil {
		b.Fatal(err)
	}
	capability := capabilities[0]
	b.Run("ConstructFullReversed", func(b *testing.B) {
		var got controlwire.ProtocolSupport
		b.ReportAllocs()
		for b.Loop() {
			got, err = controlwire.NewProtocolSupport(controlwire.ProtocolSupportRequest{Capabilities: capabilities})
		}
		if err != nil || got != support {
			b.Fatalf("support=%v/%v, want canonical full fixture", got, err)
		}
	})
	b.Run("AssessFull", func(b *testing.B) {
		request := controlwire.ProtocolAssessmentRequest{Support: support, Capability: capability}
		if err := request.Validate(); err != nil {
			b.Fatal(err)
		}
		var got controlwire.ProtocolAssessment
		b.ReportAllocs()
		for b.Loop() {
			got, err = controlwire.AssessProtocol(request)
		}
		if err != nil || got.Capability != capability || got.Outcome != controlwire.ProtocolSupportOutcomeAccepted {
			b.Fatalf("assessment=%v/%v, want exact accepted pair", got, err)
		}
	})
}

func BenchmarkCommitReplayRegistration(b *testing.B) {
	fixture := productionSocketFixture(b)
	defer func() { _ = fixture.request.Token.Destroy() }()
	want, err := controlwire.CommitReplayIdentity(fixture.request)
	if err != nil || want.Validate() != nil {
		b.Fatalf("replay seed=%v/%v, want valid fixture", want, err)
	}
	var got controlwire.ReplayIdentity
	b.ReportAllocs()
	for b.Loop() {
		got, err = controlwire.CommitReplayIdentity(fixture.request)
	}
	if err != nil || !got.Equal(want) {
		b.Fatalf("replay=%v/%v, want exact fixture", got, err)
	}
}

// This workload includes fresh net/http test-request and recorder construction
// alongside real Exchange ingress. It performs no network round trip.
func BenchmarkReceiveRegistrationWithHTTPFixture(b *testing.B) {
	fixture := productionSocketFixture(b)
	defer func() { _ = fixture.request.Token.Destroy() }()
	route, err := fixture.request.ControlRoute()
	if err != nil {
		b.Fatal(err)
	}
	path, err := route.Path()
	if err != nil {
		b.Fatal(err)
	}
	canonical, err := fixture.request.MarshalJSON()
	if err != nil || len(canonical) == 0 {
		b.Fatalf("request seed=%d/%v, want nonempty", len(canonical), err)
	}
	authority := socketServer(b, fixture.support)
	media := standardMediaType(b, exchange.StandardMediaTypeJSON)
	key := fixture.request.ControlNonce().String()
	wantReplay, err := controlwire.CommitReplayIdentity(fixture.request)
	if err != nil {
		b.Fatal(err)
	}
	var got controlwire.RoutedJSONReceive[*controlplane.RegistrationRequest]
	b.ReportAllocs()
	for b.Loop() {
		request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(canonical))
		request.Header.Set(core.HTTPHeaderContentType().String(), media.String())
		request.Header.Set(core.HTTPHeaderIdempotencyKey().String(), key)
		call, callErr := exchange.NewSocketServerCall(httptest.NewRecorder(), request)
		if callErr != nil {
			b.Fatal(callErr)
		}
		got, err = controlwire.ReceiveRoutedJSON[controlplane.RegistrationRequest, *controlplane.RegistrationRequest](controlwire.AuthorityJSONReceiveCall{Call: call, Route: route, Authority: authority})
		if err != nil || got.Body == nil {
			b.Fatalf("receive=%v, want body", err)
		}
		if destroyErr := got.Body.Token.Destroy(); destroyErr != nil {
			b.Fatal(destroyErr)
		}
	}
	if got.Replay != wantReplay || got.Assessment.Outcome != controlwire.ProtocolSupportOutcomeAccepted {
		b.Fatalf("receive replay/assessment=%v/%v, want exact supported fixture", got.Replay, got.Assessment)
	}
}

// Includes fresh HTTP output fixture construction and writes the real signed
// response through Controlwire and Exchange into Go's recorder.
func BenchmarkWriteResponseWithHTTPFixture(b *testing.B) {
	fixture := productionSocketFixture(b)
	defer func() { _ = fixture.request.Token.Destroy() }()
	authority := socketServer(b, fixture.support)
	if len(fixture.responseCanonical) == 0 {
		b.Fatalf("response fixture bytes=%d, want nonempty", len(fixture.responseCanonical))
	}
	var got *httptest.ResponseRecorder
	var err error
	b.ReportAllocs()
	for b.Loop() {
		got = httptest.NewRecorder()
		call, callErr := exchange.NewSocketServerCall(got, httptest.NewRequest(http.MethodPost, "/", nil))
		if callErr != nil {
			b.Fatal(callErr)
		}
		err = controlwire.WriteControlJSON(controlwire.ControlJSONWriteCall[controlplane.ResponseProjection[controlplane.RegistrationDocument]]{Call: call, Body: fixture.response, Authority: authority})
	}
	if got == nil {
		b.Fatalf("response recorder=%v, want completed write", got)
	}
	if err != nil || got.Code != http.StatusOK || !bytes.Equal(got.Body.Bytes(), fixture.responseCanonical) {
		b.Fatalf("response=%d/%d bytes/%v, want exact signed fixture", got.Code, got.Body.Len(), err)
	}
}
