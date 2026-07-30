package lease_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
)

func FuzzDecisionJSON(f *testing.F) {
	subject := fixtureSubject(f, 161)
	decision := fixtureGrantDecision(f, subject, 1, 1_000, fixtureGrant())
	seed, err := json.Marshal(decision)
	if err != nil {
		f.Fatalf("json.Marshal() error = %v, want nil", err)
	}
	f.Add(seed)
	f.Add([]byte("null"))
	f.Add([]byte(`{"outcome":"grant"}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var got lease.Decision
		err := got.UnmarshalJSON(data)
		if err != nil {
			if !errors.Is(err, core.ErrLeaseContract) {
				t.Fatalf("json.Unmarshal() error = %v, want %v", err, core.ErrLeaseContract)
			}
			if got != (lease.Decision{}) {
				t.Fatalf("rejected Decision mutated zero receiver")
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("Decision.Validate() error = %v, want nil", err)
		}
		canonical, err := json.Marshal(got)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v, want nil", err)
		}
		if len(canonical) > lease.DecisionCanonicalJSONMaximumBytes {
			t.Fatalf("canonical Decision = %d bytes, want at most %d",
				len(canonical), lease.DecisionCanonicalJSONMaximumBytes)
		}
		var roundTrip lease.Decision
		if err := json.Unmarshal(canonical, &roundTrip); err != nil {
			t.Fatalf("canonical json.Unmarshal() error = %v, want nil", err)
		}
		if roundTrip != got {
			t.Fatalf("canonical round trip changed Decision")
		}
		// The canonical projection is what Attest signs, so a second encoding of
		// the decoded value must be byte-identical: any accepted wire spelling
		// that produced two distinct canonical forms would produce two distinct
		// signatures over one decision.
		second, err := json.Marshal(roundTrip)
		if err != nil {
			t.Fatalf("second json.Marshal() error = %v, want nil", err)
		}
		if !bytes.Equal(second, canonical) {
			t.Fatalf("canonical encoding is not idempotent: %s then %s", canonical, second)
		}
		header, err := roundTrip.Header()
		if err != nil {
			t.Fatalf("Decision.Header() error = %v, want nil", err)
		}
		if err := header.Validate(); err != nil {
			t.Fatalf("accepted Decision carries an invalid header: %v", err)
		}
		if got.Outcome() != roundTrip.Outcome() || !got.Outcome().IsValid() {
			t.Fatalf("accepted Decision outcome = %v, want a stable closed member", got.Outcome())
		}
	})
}

func FuzzDocumentJSON(f *testing.F) {
	authority := fixtureAuthority(f, 171)
	subject := fixtureSubject(f, 172)
	decision := fixtureGrantDecision(f, subject, 1, 1_000, fixtureGrant())
	document, _ := fixtureVerified(f, authority, decision, subject)
	seed, err := json.Marshal(document)
	if err != nil {
		f.Fatalf("json.Marshal() error = %v, want nil", err)
	}
	f.Add(seed)
	f.Add([]byte("null"))
	f.Add([]byte(`{"decision":{}}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var got lease.Document
		err := got.UnmarshalJSON(data)
		if err != nil {
			if !errors.Is(err, core.ErrLeaseContract) {
				t.Fatalf("json.Unmarshal() error = %v, want %v", err, core.ErrLeaseContract)
			}
			if got != (lease.Document{}) {
				t.Fatalf("rejected Document mutated zero receiver")
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("Document.Validate() error = %v, want nil", err)
		}
		canonical, err := json.Marshal(got)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v, want nil", err)
		}
		if len(canonical) > lease.DocumentCanonicalJSONMaximumBytes {
			t.Fatalf("canonical Document = %d bytes, want at most %d",
				len(canonical), lease.DocumentCanonicalJSONMaximumBytes)
		}
		var roundTrip lease.Document
		if err := json.Unmarshal(canonical, &roundTrip); err != nil {
			t.Fatalf("canonical json.Unmarshal() error = %v, want nil", err)
		}
		if roundTrip != got {
			t.Fatalf("canonical round trip changed Document")
		}
		second, err := json.Marshal(roundTrip)
		if err != nil {
			t.Fatalf("second json.Marshal() error = %v, want nil", err)
		}
		if !bytes.Equal(second, canonical) {
			t.Fatalf("canonical encoding is not idempotent: %s then %s", canonical, second)
		}
		// A structurally accepted document still carries no authority. Verifying
		// it against the fixture's real trusted key must either authenticate the
		// real fixture decision or fail with a typed verification identity; it
		// must never authenticate a mutated body.
		header, err := roundTrip.Decision.Header()
		if err != nil {
			t.Fatalf("Decision.Header() error = %v, want nil", err)
		}
		verified, verifyErr := lease.Verify(lease.VerifyRequest{
			Document: roundTrip, TrustedKeys: authority.trusted,
			ExpectedSubject: header.Subject,
		})
		if verifyErr != nil {
			if !errors.Is(verifyErr, core.ErrLeaseContract) {
				t.Fatalf("lease.Verify() error = %v, want %v", verifyErr, core.ErrLeaseContract)
			}
			if verified != (lease.Verified{}) {
				t.Fatalf("rejected lease.Verify() returned a non-zero proof carrier")
			}
			return
		}
		authentic, decisionErr := verified.Decision()
		if decisionErr != nil {
			t.Fatalf("Verified.Decision() error = %v, want nil", decisionErr)
		}
		if authentic != decision {
			t.Fatalf("lease.Verify() authenticated a Document that is not the signed fixture decision")
		}
	})
}
