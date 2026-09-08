package controlwire

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type controlwireJSONBoundary interface {
	controlwireJSONValue
	UnmarshalJSON([]byte) error
}

// Typed dispatch is shared by the table and fuzzer; it contains no verdicts.
func (f controlwireFuzzFixtures) jsonReceiver(door controlwireJSONDoor) (controlwireJSONBoundary, error) {
	switch door {
	case controlwireJSONDoorRequestNonce:
		v := f.requestNonce
		return &v, core.ErrControlWireNonce
	case controlwireJSONDoorAuthorityNonce:
		v := f.authorityNonce
		return &v, core.ErrControlWireNonce
	case controlwireJSONDoorRevision:
		v := f.revision
		return &v, core.ErrControlWireRevision
	case controlwireJSONDoorPolicyRevisionID:
		v := f.policyID
		return &v, core.ErrControlWirePolicyCursor
	case controlwireJSONDoorPolicyCursor:
		v := f.policyCursor
		return &v, core.ErrControlWirePolicyCursor
	case controlwireJSONDoorRegistrationToken:
		v := f.token
		return &v, core.ErrControlWireToken
	case controlwireJSONDoorRegistrationTokenVerifier:
		v := f.verifier
		return &v, core.ErrControlWireToken
	case controlwireJSONDoorRouteFamily:
		v := f.routeFamily
		return &v, core.ErrControlWireRoute
	case controlwireJSONDoorRequestCommitment:
		v := f.commitment
		return &v, core.ErrControlWireContract
	case controlwireJSONDoorReplayIdentity:
		v := f.replayIdentity
		return &v, core.ErrControlWireContract
	default:
		return nil, core.ErrControlWireContract
	}
}

func TestJSONIngressExactDocumentCeilings(t *testing.T) {
	t.Parallel()
	for door := controlwireJSONDoorUnknown + 1; door < controlwireJSONDoorLimit; door++ {
		t.Run(door.receiverName(), func(t *testing.T) {
			t.Parallel()
			maximum := core.JSONDocumentMaximumBytes
			if door == controlwireJSONDoorReplayIdentity {
				maximum = ReplayIdentityJSONMaximumBytes
			}
			cases := []struct {
				name        string
				delta       int
				wantRefusal bool
			}{
				{name: "positive last byte below ceiling", delta: -1},
				{name: "positive exact ceiling", delta: 0},
				{name: "negative first byte beyond ceiling preserves populated receiver", delta: 1, wantRefusal: true},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					fixtures := controlwireFixturesForFuzz(t)
					defer func() { _ = fixtures.token.Destroy() }()
					got, wantIdentity := fixtures.jsonReceiver(door)
					before, err := got.MarshalJSON()
					if err != nil {
						t.Fatalf("seed encoding error=%v, want nil", err)
					}
					document := append(bytes.Repeat([]byte{' '}, maximum+tc.delta-len(before)), before...)
					gotErr := got.UnmarshalJSON(document)
					if (gotErr != nil) != tc.wantRefusal {
						t.Fatalf("decode %d bytes error=%v, want refusal=%v", len(document), gotErr, tc.wantRefusal)
					}
					if gotErr != nil && (!errors.Is(gotErr, wantIdentity) || !errors.Is(gotErr, core.ErrJSONContract)) {
						t.Fatalf("decode error=%v, want %v and %v", gotErr, wantIdentity, core.ErrJSONContract)
					}
					if token, ok := got.(*RegistrationToken); ok && gotErr == nil {
						defer func() { _ = token.Destroy() }()
					}
					after, err := got.MarshalJSON()
					if err != nil || !bytes.Equal(after, before) {
						t.Fatalf("receiver=%q/%v, want %q/nil", after, err, before)
					}
				})
			}
		})
	}
}

func TestProtocolSupportCorruptCountRefusesBeforeIndexing(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		count int
	}{
		{name: "negative count cannot index private padding", count: -1},
		{name: "neutral zero count cannot represent support"},
		{name: "negative count beyond fixed storage", count: ProtocolCapabilityMaximum + 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ProtocolSupport{count: tc.count}.Validate()
			if !errors.Is(got, core.ErrControlWireProtocolSupport) {
				t.Fatalf("support error=%v, want %v", got, core.ErrControlWireProtocolSupport)
			}
		})
	}
}

func TestNilJSONReceiversRefuseEveryExternalDoor(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		receiver controlwireJSONBoundary
		want     error
	}{
		{name: "request nonce", receiver: (*RequestNonce)(nil), want: core.ErrControlWireNonce},
		{name: "authority nonce", receiver: (*AuthorityNonce)(nil), want: core.ErrControlWireNonce},
		{name: "revision", receiver: (*Revision)(nil), want: core.ErrControlWireRevision},
		{name: "policy ID", receiver: (*PolicyRevisionID)(nil), want: core.ErrControlWirePolicyCursor},
		{name: "cursor", receiver: (*PolicyCursor)(nil), want: core.ErrControlWirePolicyCursor},
		{name: "token", receiver: (*RegistrationToken)(nil), want: core.ErrControlWireToken},
		{name: "verifier", receiver: (*RegistrationTokenVerifier)(nil), want: core.ErrControlWireToken},
		{name: "family", receiver: (*RouteFamily)(nil), want: core.ErrControlWireRoute},
		{name: "commitment", receiver: (*RequestCommitment)(nil), want: core.ErrControlWireContract},
		{name: "replay identity", receiver: (*ReplayIdentity)(nil), want: core.ErrControlWireContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.receiver.UnmarshalJSON(nil)
			if !errors.Is(err, tc.want) || !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("nil receiver error=%v, want %v and %v", err, tc.want, core.ErrJSONContract)
			}
		})
	}
}

func TestProtocolSupportPrivateStorageCannotSmuggleCapabilities(t *testing.T) {
	t.Parallel()
	first := ProtocolCapability{Revision: Revision2026V1, Family: RouteFamilyRegistrations}
	last := ProtocolCapability{Revision: Revision2026V1, Family: RouteFamilyUpgrades}
	for _, tc := range []struct {
		name    string
		count   int
		values  [ProtocolCapabilityMaximum]ProtocolCapability
		wantErr error
	}{
		{name: "positive exact sorted two member prefix", count: 2, values: [ProtocolCapabilityMaximum]ProtocolCapability{first, last}},
		{name: "negative undeclared capability after prefix", count: 1, values: [ProtocolCapabilityMaximum]ProtocolCapability{first, last}, wantErr: core.ErrControlWireProtocolSupport},
		{name: "negative invalid declared element", count: 2, values: [ProtocolCapabilityMaximum]ProtocolCapability{first}, wantErr: core.ErrControlWireProtocolSupport},
		{name: "negative inverted order", count: 2, values: [ProtocolCapabilityMaximum]ProtocolCapability{last, first}, wantErr: core.ErrControlWireProtocolSupport},
		{name: "negative duplicated pair", count: 2, values: [ProtocolCapabilityMaximum]ProtocolCapability{first, first}, wantErr: core.ErrControlWireProtocolSupport},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ProtocolSupport{count: tc.count, capabilities: tc.values}.Validate()
			if !errors.Is(got, tc.wantErr) {
				t.Fatalf("support error=%v, want %v", got, tc.wantErr)
			}
		})
	}
}

func TestClientNilInterfaceBodyRefusesWithoutExecution(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		call ClientJSONCall[RoutedJSONRequest]
	}{
		{name: "absent request cannot invoke a capability"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, policy, err := clientExchange(tc.call)
			if !errors.Is(err, core.ErrControlWireContract) || got.Body != nil || policy != (exchange.JSONPolicy{}) {
				t.Fatalf("nil request=%v/%v/%v, want zero outputs and %v", got, policy, err, core.ErrControlWireContract)
			}
		})
	}
}
