package lease_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/lease"
)

// maximalIdentifierBytes is the widest opaque identifier. Identifier text is
// fixed-width lowercase hexadecimal, so every nonzero value encodes to the same
// extent; all-maximum bytes simply make that explicit.
func maximalIdentifierBytes() [lease.IdentifierBytes]byte {
	var value [lease.IdentifierBytes]byte
	for index := range value {
		value[index] = math.MaxUint8
	}
	return value
}

// maximalSubject is the widest subject: three fixed-width identifiers.
func maximalSubject(tb testing.TB) lease.Subject {
	tb.Helper()

	value := maximalIdentifierBytes()
	product, err := lease.NewProduct(value)
	if err != nil {
		tb.Fatalf("lease.NewProduct() error = %v, want nil", err)
	}
	entitlement, err := lease.NewEntitlementID(value)
	if err != nil {
		tb.Fatalf("lease.NewEntitlementID() error = %v, want nil", err)
	}
	device, err := lease.NewDeviceID(value)
	if err != nil {
		tb.Fatalf("lease.NewDeviceID() error = %v, want nil", err)
	}
	return lease.Subject{Product: product, EntitlementID: entitlement, DeviceID: device}
}

// maximalHeader is the widest header: the widest subject, the widest generation
// decimal, and the widest instant decimal.
func maximalHeader(tb testing.TB) lease.Header {
	tb.Helper()

	generation, err := lease.NewGeneration(math.MaxUint64)
	if err != nil {
		tb.Fatalf("lease.NewGeneration() error = %v, want nil", err)
	}
	return lease.Header{
		Revision:   lease.RevisionV1,
		Subject:    maximalSubject(tb),
		Generation: generation,
		IssuedAt:   fixtureInstant(math.MinInt64),
	}
}

// maximalGrant is the widest grant: four most-negative instants, which carry
// the widest signed decimal, arranged to satisfy the timeline order.
func maximalGrant() lease.Grant {
	return lease.Grant{
		NotBefore:    fixtureInstant(math.MinInt64),
		ContactAfter: fixtureInstant(math.MinInt64 + 1),
		NotAfter:     fixtureInstant(math.MinInt64 + 2),
		GoodUntil:    fixtureInstant(math.MinInt64 + 3),
	}
}

// maximalRefusal is the widest refusal: the widest instant decimal.
func maximalRefusal() lease.Refusal {
	return lease.Refusal{
		ContactAfter: fixtureInstant(math.MinInt64),
	}
}

// maximalRevocation is the widest revocation: the longest reason token.
func maximalRevocation() lease.Revocation {
	return lease.Revocation{Reason: lease.RevocationReasonSecurityOrPlatformRisk}
}

// TestCanonicalMaximumsAreExactlyAttained proves each declared canonical
// maximum is reached by a real maximal value, not merely never exceeded. These
// constants are the package's fixed-size claim: a maximum no value can reach
// silently over-reserves the wire, and one a value can exceed turns a valid
// signed decision into an encoding failure at the worst possible moment.
func TestCanonicalMaximumsAreExactlyAttained(t *testing.T) {
	t.Parallel()

	generation, err := lease.NewGeneration(math.MaxUint64)
	if err != nil {
		t.Fatalf("lease.NewGeneration() error = %v, want nil", err)
	}
	product, err := lease.NewProduct(maximalIdentifierBytes())
	if err != nil {
		t.Fatalf("lease.NewProduct() error = %v, want nil", err)
	}

	cases := []struct {
		value json.Marshaler
		name  string
		want  int
	}{
		{name: "identifier", value: product, want: lease.IdentifierCanonicalJSONMaximumBytes},
		{name: "generation", value: generation, want: lease.GenerationCanonicalJSONMaximumBytes},
		{name: "revision", value: lease.RevisionV1, want: lease.RevisionCanonicalJSONMaximumBytes},
		{name: "outcome", value: lease.OutcomeRevocation, want: lease.OutcomeCanonicalJSONMaximumBytes},
		{
			name:  "revocation reason",
			value: lease.RevocationReasonSecurityOrPlatformRisk,
			want:  lease.RevocationReasonCanonicalJSONMaximumBytes,
		},
		{name: "subject", value: maximalSubject(t), want: lease.SubjectCanonicalJSONMaximumBytes},
		{name: "grant", value: maximalGrant(), want: lease.GrantCanonicalJSONMaximumBytes},
		{name: "refusal", value: maximalRefusal(), want: lease.RefusalCanonicalJSONMaximumBytes},
		{name: "revocation", value: maximalRevocation(), want: lease.RevocationCanonicalJSONMaximumBytes},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			encoded, marshalErr := tc.value.MarshalJSON()
			if marshalErr != nil {
				t.Fatalf("%s MarshalJSON() error = %v, want nil", tc.name, marshalErr)
			}
			if len(encoded) != tc.want {
				t.Fatalf("len(%s canonical JSON) = %d, want %d: %s", tc.name, len(encoded), tc.want, encoded)
			}
		})
	}
}

// TestSignedMaximumsAreExactlyAttained proves the widest grant decision and the
// widest signed document reach their declared maxima through the real Attest
// signing path, so the exported document bound covers a real envelope rather
// than an estimate of one.
func TestSignedMaximumsAreExactlyAttained(t *testing.T) {
	t.Parallel()

	decision, err := lease.NewGrantDecision(lease.GrantDecisionRequest{
		Header: maximalHeader(t), Grant: maximalGrant(),
	})
	if err != nil {
		t.Fatalf("lease.NewGrantDecision() error = %v, want nil", err)
	}
	decisionJSON, err := json.Marshal(decision)
	if err != nil {
		t.Fatalf("json.Marshal(Decision) error = %v, want nil", err)
	}
	if len(decisionJSON) != lease.DecisionCanonicalJSONMaximumBytes {
		t.Fatalf("len(Decision canonical JSON) = %d, want %d",
			len(decisionJSON), lease.DecisionCanonicalJSONMaximumBytes)
	}

	authority := fixtureAuthority(t, 181)
	envelope, err := attest.Sign(attest.SignRequest[lease.Domain]{
		Body: decision, Key: authority.private,
	})
	if err != nil {
		t.Fatalf("attest.Sign() error = %v, want nil", err)
	}
	document := lease.Document{Decision: decision, Attestation: envelope}
	documentJSON, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("json.Marshal(Document) error = %v, want nil", err)
	}
	if len(documentJSON) != lease.DocumentCanonicalJSONMaximumBytes {
		t.Fatalf("len(Document canonical JSON) = %d, want %d",
			len(documentJSON), lease.DocumentCanonicalJSONMaximumBytes)
	}

	verified, err := lease.Verify(lease.VerifyRequest{
		Document: document, TrustedKeys: authority.trusted,
		ExpectedSubject: maximalSubject(t),
	})
	if err != nil {
		t.Fatalf("lease.Verify() error = %v, want nil", err)
	}
	if err := verified.Validate(); err != nil {
		t.Fatalf("Verified.Validate() error = %v, want nil", err)
	}
}

// TestGrantIsTheBindingDecisionVariant proves the exported decision maximum is
// set by the grant body and that both denial variants encode strictly smaller.
// The exported constant is the one number a caller sizes a buffer against, so
// which variant binds it must be a tested fact rather than an assumption.
func TestGrantIsTheBindingDecisionVariant(t *testing.T) {
	t.Parallel()

	header := maximalHeader(t)
	grant, err := lease.NewGrantDecision(lease.GrantDecisionRequest{
		Header: header, Grant: maximalGrant(),
	})
	if err != nil {
		t.Fatalf("lease.NewGrantDecision() error = %v, want nil", err)
	}
	grantJSON, err := json.Marshal(grant)
	if err != nil {
		t.Fatalf("json.Marshal(grant) error = %v, want nil", err)
	}
	refusal, err := lease.NewRefusalDecision(lease.RefusalDecisionRequest{
		Header: header, Refusal: maximalRefusal(),
	})
	if err != nil {
		t.Fatalf("lease.NewRefusalDecision() error = %v, want nil", err)
	}
	revocation, err := lease.NewRevocationDecision(lease.RevocationDecisionRequest{
		Header: header, Revocation: maximalRevocation(),
	})
	if err != nil {
		t.Fatalf("lease.NewRevocationDecision() error = %v, want nil", err)
	}

	cases := []struct {
		name     string
		decision lease.Decision
	}{
		{name: "maximal refusal", decision: refusal},
		{name: "maximal revocation", decision: revocation},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name+" encodes strictly smaller than the maximal grant", func(t *testing.T) {
			t.Parallel()

			encoded, marshalErr := json.Marshal(tc.decision)
			if marshalErr != nil {
				t.Fatalf("json.Marshal() error = %v, want nil", marshalErr)
			}
			if len(encoded) >= len(grantJSON) {
				t.Fatalf("len(%s JSON) = %d, want less than the maximal grant %d",
					tc.name, len(encoded), len(grantJSON))
			}
		})
	}
}

// TestAcceptedJSONBoundsExceedTheirCanonicalMaximums proves every accepted
// bound leaves real whitespace headroom over its canonical form. A bound equal
// to the canonical extent would reject its own canonical output the moment a
// peer added one insignificant byte.
func TestAcceptedJSONBoundsExceedTheirCanonicalMaximums(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		canonical int
		accepted  int
	}{
		{
			name:      "identifier",
			canonical: lease.IdentifierCanonicalJSONMaximumBytes,
			accepted:  lease.IdentifierJSONMaximumBytes,
		},
		{
			name:      "generation",
			canonical: lease.GenerationCanonicalJSONMaximumBytes,
			accepted:  lease.GenerationJSONMaximumBytes,
		},
		{
			name:      "revision",
			canonical: lease.RevisionCanonicalJSONMaximumBytes,
			accepted:  lease.RevisionJSONMaximumBytes,
		},
		{
			name:      "outcome",
			canonical: lease.OutcomeCanonicalJSONMaximumBytes,
			accepted:  lease.OutcomeJSONMaximumBytes,
		},
		{
			name:      "revocation reason",
			canonical: lease.RevocationReasonCanonicalJSONMaximumBytes,
			accepted:  lease.RevocationReasonJSONMaximumBytes,
		},
		{
			name:      "subject",
			canonical: lease.SubjectCanonicalJSONMaximumBytes,
			accepted:  lease.SubjectJSONMaximumBytes,
		},
		{
			name:      "grant",
			canonical: lease.GrantCanonicalJSONMaximumBytes,
			accepted:  lease.GrantJSONMaximumBytes,
		},
		{
			name:      "refusal",
			canonical: lease.RefusalCanonicalJSONMaximumBytes,
			accepted:  lease.RefusalJSONMaximumBytes,
		},
		{
			name:      "revocation",
			canonical: lease.RevocationCanonicalJSONMaximumBytes,
			accepted:  lease.RevocationJSONMaximumBytes,
		},
		{
			name:      "decision",
			canonical: lease.DecisionCanonicalJSONMaximumBytes,
			accepted:  lease.DecisionJSONMaximumBytes,
		},
		{
			name:      "document",
			canonical: lease.DocumentCanonicalJSONMaximumBytes,
			accepted:  lease.DocumentJSONMaximumBytes,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.canonical <= 0 {
				t.Fatalf("%s canonical maximum = %d, want a positive extent", tc.name, tc.canonical)
			}
			if tc.accepted <= tc.canonical {
				t.Fatalf("%s accepted bound = %d, want greater than canonical %d",
					tc.name, tc.accepted, tc.canonical)
			}
		})
	}
}
