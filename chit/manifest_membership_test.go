package chit

import (
	"bytes"
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/receipt"
)

func TestManifestMembershipCompleteStreamBoundaryTable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		extents  []uint64
		selected int
	}{
		{name: "empty object preserves zero extent", extents: []uint64{0}},
		{name: "minimum nonempty object preserves one byte", extents: []uint64{1}},
		{name: "first member remains bound to a nonempty tail", extents: []uint64{1, 2}},
		{name: "last member remains bound to a nonempty prefix", extents: []uint64{2, 1}, selected: 1},
		{name: "empty interior member keeps its exact position", extents: []uint64{1, 0, 2}, selected: 1},
		{name: "one below signed extent ceiling is authentic", extents: []uint64{math.MaxInt64 - 1}},
		{name: "exact signed extent ceiling is authentic", extents: []uint64{math.MaxInt64}},
		{name: "last member closes the exact aggregate extent", extents: []uint64{math.MaxInt64 - 1, 1}, selected: 1},
		{name: "empty prefix stays distinct beside saturated tail", extents: []uint64{0, math.MaxInt64}},
		{name: "member beyond catalog page retains full stream closure", extents: make([]uint64, core.CatalogPageMaximumEntries+1), selected: core.CatalogPageMaximumEntries},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fixture := newChitFixture(t, 0x21, 1)
			additions := manifestAdmissionExtentAdditions(t, tc.extents)
			accumulator := mustManifestAdmissionAccumulator(t)
			var selected ManifestAdmission
			for index, addition := range additions {
				admission, err := accumulator.Add(addition)
				if err != nil {
					t.Fatal(err)
				}
				if index == tc.selected {
					selected = admission
				}
			}
			summary := manifestSummaryFixture(t, additions...)
			sealed, err := accumulator.Seal(summary)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := sealed.Destroy(); err != nil {
					t.Fatal(err)
				}
			})
			payload := fixture.document.Payload
			payload.Manifest = summary
			chitDocument, err := Issue(Issuance{Signer: fixture.private, TrustedKeys: fixture.trusted, Payload: payload})
			if err != nil {
				t.Fatal(err)
			}
			custody, err := Verify(Verification{Document: chitDocument, TrustedKeys: fixture.trusted, Expected: Expectation{Identity: payload.Identity, Scope: payload.Scope}})
			if err != nil {
				t.Fatal(err)
			}
			addition := additions[tc.selected]
			entry, err := sealed.Verify(selected, addition.Evidence)
			if err != nil {
				t.Fatal(err)
			}
			document, err := IssueManifestMembership(ManifestMembershipIssuance{Signer: fixture.private, Custody: custody, Entry: entry})
			if err != nil {
				t.Fatal(err)
			}
			if err := sealed.Destroy(); err != nil {
				t.Fatal(err)
			}
			// The originating in-process authority is gone. Only the genuine signed
			// receipt may restore exactly this member of the complete stream.
			data, err := document.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			var retained ManifestMembershipDocument
			if err := retained.UnmarshalJSON(data); err != nil {
				t.Fatal(err)
			}
			proof, err := VerifyManifestMembership(ManifestMembershipVerification{Document: retained, Custody: custody, Evidence: addition.Evidence, Sequence: addition.Entry.Sequence, TrustedKeys: fixture.trusted})
			if err != nil {
				t.Fatal(err)
			}
			gotAddition, additionErr := proof.Addition()
			gotSummary, summaryErr := proof.Summary()
			if errors.Join(additionErr, summaryErr) != nil || gotAddition.Entry != addition.Entry || gotSummary != summary {
				t.Fatalf("retained membership = %+v/%+v/%v/%v, want exact entry and complete closure", gotAddition, gotSummary, additionErr, summaryErr)
			}
		})
	}
}

func membershipFixture(t testing.TB, fixture chitFixture) (ManifestMembershipDocument, Verified) {
	t.Helper()
	custody, err := Verify(Verification{Document: fixture.document, TrustedKeys: fixture.trusted, Expected: Expectation{Scope: fixture.scope, Identity: fixture.identity}})
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := NewManifestEntryVerifier(fixture.addition.Entry.Sequence)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifier.Add(fixture.addition); err != nil {
		t.Fatal(err)
	}
	entry, err := verifier.Seal(fixture.summary)
	if err != nil {
		t.Fatal(err)
	}
	document, err := IssueManifestMembership(ManifestMembershipIssuance{Signer: fixture.private, Custody: custody, Entry: entry})
	if err != nil {
		t.Fatal(err)
	}
	return document, custody
}

func TestManifestMembershipLayerTriad(t *testing.T) {
	t.Parallel()
	fixture := newChitFixture(t, 0x91, 1)
	foreign := newChitFixture(t, 0xa1, 1)
	document, custody := membershipFixture(t, fixture)
	foreignDocument, foreignCustody := membershipFixture(t, foreign)
	base := ManifestMembershipVerification{Document: document, Custody: custody, Evidence: fixture.addition.Evidence, Sequence: fixture.addition.Entry.Sequence, TrustedKeys: fixture.trusted}
	changedName, err := ParseEntryName(chitFixtureNameB)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func(*ManifestMembershipVerification)
		want   error
	}{
		{name: "positive exact verified closure restores exact entry"},
		{name: "neutral repeated verification yields same membership"},
		{name: "negative unset certificate", mutate: func(r *ManifestMembershipVerification) { r.Document = ManifestMembershipDocument{} }, want: core.ErrChitContract},
		{name: "negative unset custody", mutate: func(r *ManifestMembershipVerification) { r.Custody = Verified{} }, want: core.ErrChitContract},
		{name: "negative unset receipt", mutate: func(r *ManifestMembershipVerification) { r.Evidence = receipt.VerifiedEvidence{} }, want: core.ErrReceiptContract},
		{name: "negative untrusted issuer", mutate: func(r *ManifestMembershipVerification) { r.TrustedKeys = foreign.trusted }, want: core.ErrChitVerification},
		{name: "negative unsigned name substitution", mutate: func(r *ManifestMembershipVerification) { r.Document.Payload.Entry.Name = changedName }, want: core.ErrChitVerification},
		{name: "negative foreign authenticated custody", mutate: func(r *ManifestMembershipVerification) { r.Custody = foreignCustody }, want: core.ErrChitConflict},
		{name: "negative foreign authenticated receipt", mutate: func(r *ManifestMembershipVerification) { r.Evidence = foreign.addition.Evidence }, want: core.ErrChitConflict},
		{name: "negative wrong selected sequence", mutate: func(r *ManifestMembershipVerification) { r.Sequence = mustEntrySequence(t, 2) }, want: core.ErrChitConflict},
		{name: "negative unsigned collection substitution", mutate: func(r *ManifestMembershipVerification) {
			r.Document.Payload.Custody.Collection = foreignDocument.Payload.Custody.Collection
		}, want: core.ErrChitVerification},
		{name: "negative foreign attestation substitution", mutate: func(r *ManifestMembershipVerification) { r.Document.Attestation = foreignDocument.Attestation }, want: core.ErrChitVerification},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := base
			if tc.mutate != nil {
				tc.mutate(&request)
			}
			got, err := VerifyManifestMembership(request)
			if !errors.Is(err, tc.want) {
				t.Fatalf("membership error = %v, want %v", err, tc.want)
			}
			if err != nil {
				if got != (VerifiedManifestEntry{}) {
					t.Fatalf("refused membership = %+v, want zero", got)
				}
				return
			}
			addition, err := got.Addition()
			summary, summaryErr := got.Summary()
			if errors.Join(err, summaryErr) != nil || addition.Entry != fixture.addition.Entry || summary != fixture.summary {
				t.Fatalf("membership = %+v/%+v/%v/%v, want exact source entry and closure", addition, summary, err, summaryErr)
			}
		})
	}
	if got, err := IssueManifestMembership(ManifestMembershipIssuance{Signer: fixture.private, Custody: custody}); !errors.Is(err, core.ErrChitContract) || got != (ManifestMembershipDocument{}) {
		t.Fatalf("unverified issuance = %+v/%v, want zero and refusal", got, err)
	}
}

func FuzzManifestMembershipDocumentSemanticClosure(f *testing.F) {
	fixture := newChitFixture(f, 0x81, 1)
	document, custody := membershipFixture(f, fixture)
	seed, err := document.MarshalJSON()
	if err != nil {
		f.Fatal(err)
	}
	for _, data := range [][]byte{seed, nil, []byte("{}"), bytes.Repeat([]byte(" "), ChitDocumentJSONMaximumBytes+1)} {
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		got := document
		err := got.UnmarshalJSON(data)
		if err != nil {
			if !errors.Is(err, core.ErrJSONContract) || got != document {
				t.Fatalf("decoder refusal = %+v/%v, want unchanged and typed JSON refusal", got, err)
			}
			return
		}
		proof, err := VerifyManifestMembership(ManifestMembershipVerification{Document: got, Custody: custody, Evidence: fixture.addition.Evidence, Sequence: fixture.addition.Entry.Sequence, TrustedKeys: fixture.trusted})
		if err != nil {
			if proof != (VerifiedManifestEntry{}) || (!errors.Is(err, core.ErrChitVerification) && !errors.Is(err, core.ErrChitConflict)) {
				t.Fatalf("authentication refusal = %+v/%v, want zero typed proof", proof, err)
			}
			return
		}
		if got != document || got.Validate() != nil {
			t.Fatalf("authenticated mutation = %+v, want exact signed seed", got)
		}
		encoded, err := got.MarshalJSON()
		var roundTrip ManifestMembershipDocument
		decodeErr := roundTrip.UnmarshalJSON(encoded)
		second, secondErr := roundTrip.MarshalJSON()
		if errors.Join(err, decodeErr, secondErr) != nil || roundTrip != got || !bytes.Equal(second, encoded) {
			t.Fatalf("canonical closure = %v/%v/%v, want exact fixed point", err, decodeErr, secondErr)
		}
	})
}
