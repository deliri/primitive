package timeproof

import (
	"github.com/deliri/primitive/v2026/core"
	"reflect"
	"slices"
	"testing"
)

func TestTimestampZeroProofRejectsEveryPartialFact(t *testing.T) {
	t.Parallel()
	fixture := loadAuthenticFixture(t)
	verified, err := Verify(VerifyRequest{Response: fixture.response, Request: fixture.request, ExpectedDigest: fixture.digest})
	if err != nil {
		t.Fatalf("Verify(fixture) error = %v, want nil", err)
	}
	cases := []struct {
		name     string
		value    AuthoritativeTimestamp
		wantZero bool
	}{
		{name: "absent proof", wantZero: true},
		{name: "retained signer", value: AuthoritativeTimestamp{signer: verified.signer}},
		{name: "retained serial", value: AuthoritativeTimestamp{serial: verified.serial}},
		{name: "retained generation", value: AuthoritativeTimestamp{time: verified.time}},
		{name: "retained policy", value: AuthoritativeTimestamp{policy: verified.policy}},
		{name: "retained response", value: AuthoritativeTimestamp{evidence: AuthorityEvidence{response: fixture.response}}},
		{name: "retained request body", value: AuthoritativeTimestamp{evidence: AuthorityEvidence{request: Request{body: fixture.request.body}}}},
		{name: "retained request digest", value: AuthoritativeTimestamp{evidence: AuthorityEvidence{request: Request{digest: fixture.digest}}}},
		{name: "retained request nonce", value: AuthoritativeTimestamp{evidence: AuthorityEvidence{request: Request{nonce: fixture.request.nonce}}}},
		{name: "retained request authority", value: AuthoritativeTimestamp{evidence: AuthorityEvidence{request: Request{authority: fixture.request.authority}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := timestampHasNoProof(tc.value); got != tc.wantZero {
				t.Fatalf("partial timestamp zero = %t for %+v, want %t", got, tc.value, tc.wantZero)
			}
		})
	}
}

// Every component participates in the refusal proof; populated private fields
// cannot disappear behind an incomplete production-side zero predicate.
func timestampHasNoProof(value AuthoritativeTimestamp) bool {
	return value.evidence.response == nil && value.evidence.request.body == nil &&
		value.evidence.request.digest == (core.SHA256Digest{}) &&
		value.evidence.request.nonce == (Nonce{}) && value.evidence.request.authority == AuthorityUnknown &&
		value.time == (AuthoritativeTime{}) && value.signer == (core.SHA256Digest{}) &&
		value.serial == (SerialNumber{}) && value.policy == TimestampPolicyUnknown
}

func TestZeroProofPredicateCoversCompleteCarrierShapes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		shape reflect.Type
		want  []string
	}{
		{name: "timestamp proof fields", shape: reflect.TypeFor[AuthoritativeTimestamp](), want: []string{"evidence", "time", "signer", "serial", "policy"}},
		{name: "evidence custody fields", shape: reflect.TypeFor[AuthorityEvidence](), want: []string{"response", "request"}},
		{name: "request binding fields", shape: reflect.TypeFor[Request](), want: []string{"body", "digest", "nonce", "authority"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got []string
			for field := range tc.shape.Fields() {
				got = append(got, field.Name)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("zero-proof carrier fields = %v, want %v; update the complete field predicate", got, tc.want)
			}
		})
	}
}
