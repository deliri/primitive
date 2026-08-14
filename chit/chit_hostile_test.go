package chit

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"hash/crc32"
	"math"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/receipt"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	chitFixtureNameA = "evidence-a.json"
	chitFixtureNameB = "evidence-b.json"
)

type chitFixture struct {
	private  ed25519.PrivateKey
	addition ManifestAddition
	trusted  attest.TrustedKeys
	document Document
	summary  ManifestSummary
	scope    receipt.Scope
	identity ChitID
}

type chitEntryFixtureRequest struct {
	Extent   *core.ByteLength
	Name     string
	Private  ed25519.PrivateKey
	Trusted  attest.TrustedKeys
	Sequence uint64
	Scope    receipt.Scope
	Marker   byte
}

type chitDerivedEntryFixtureRequest struct {
	Name     string
	Fixture  chitFixture
	Sequence uint64
	Marker   byte
}

func TestManifestAccumulatorLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive contiguous authenticated streams seal exact summaries", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			objects uint64
		}{
			{name: "one object minimum stream", objects: 1},
			{name: "two object stream", objects: 2},
			{name: "three object stream", objects: 3},
			{name: "four object stream", objects: 4},
			{name: "five object stream", objects: 5},
			{name: "seven object stream", objects: 7},
			{name: "eight object stream", objects: 8},
			{name: "ten object stream", objects: 10},
			{name: "sixteen object stream", objects: 16},
			{name: "thirty-two object stream", objects: 32},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				fixture := newChitFixture(t, byte(tc.objects)+0x20, 1)
				accumulator := NewManifestAccumulator()
				if gotErr := accumulator.Add(fixture.addition); gotErr != nil {
					t.Fatalf("ManifestAccumulator.Add(sequence 1) error = %v, want nil", gotErr)
				}
				for sequence := uint64(2); sequence <= tc.objects; sequence++ {
					addition := chitManifestEntryFixture(t, chitDerivedEntryFixtureRequest{
						Fixture: fixture, Marker: byte(sequence) + 0x40,
						Sequence: sequence, Name: chitFixtureNameB,
					})
					if gotErr := accumulator.Add(addition); gotErr != nil {
						t.Fatalf("ManifestAccumulator.Add(sequence %d) error = %v, want nil", sequence, gotErr)
					}
				}
				got, gotErr := accumulator.Seal()
				if gotErr != nil || got.Validate() != nil || got.Objects.Uint64() != tc.objects ||
					got.TotalBytes.Uint64() != tc.objects {
					t.Fatalf("ManifestAccumulator.Seal(%d objects) = (%v, %v), want exact %d-object/%d-byte summary",
						tc.objects, got, gotErr, tc.objects, tc.objects)
				}
			})
		}
	})

	t.Run("negative gaps substitutions overflow and terminal reuse fail loudly", func(t *testing.T) {
		t.Parallel()

		fixture := newChitFixture(t, 0x21, 1)
		second := chitManifestEntryFixture(t, chitDerivedEntryFixtureRequest{
			Fixture: fixture, Marker: 0x31, Sequence: 2, Name: chitFixtureNameB,
		})
		other := newChitFixture(t, 0x36, 1)
		cases := []struct {
			wantErr error
			build   func() (*ManifestAccumulator, ManifestAddition)
			name    string
		}{
			{name: "nil accumulator add", build: func() (*ManifestAccumulator, ManifestAddition) { return nil, fixture.addition }, wantErr: core.ErrChitContract},
			{name: "zero accumulator add", build: func() (*ManifestAccumulator, ManifestAddition) { return &ManifestAccumulator{}, fixture.addition }, wantErr: core.ErrChitContract},
			{name: "zero addition", build: func() (*ManifestAccumulator, ManifestAddition) { return NewManifestAccumulator(), ManifestAddition{} }, wantErr: core.ErrChitContract},
			{name: "first sequence starts one above one", build: func() (*ManifestAccumulator, ManifestAddition) {
				value := second
				return NewManifestAccumulator(), value
			}, wantErr: core.ErrChitConflict},
			{name: "first sequence starts far above one", build: func() (*ManifestAccumulator, ManifestAddition) {
				value := second
				value.Entry.Sequence = mustEntrySequence(t, 255)
				return NewManifestAccumulator(), value
			}, wantErr: core.ErrChitConflict},
			{name: "duplicate sequence repeats one", build: func() (*ManifestAccumulator, ManifestAddition) {
				accumulator := NewManifestAccumulator()
				if gotErr := accumulator.Add(fixture.addition); gotErr != nil {
					t.Fatalf("ManifestAccumulator.Add(setup) error = %v, want nil", gotErr)
				}
				return accumulator, fixture.addition
			}, wantErr: core.ErrChitConflict},
			{name: "gap skips sequence two", build: func() (*ManifestAccumulator, ManifestAddition) {
				accumulator := NewManifestAccumulator()
				if gotErr := accumulator.Add(fixture.addition); gotErr != nil {
					t.Fatalf("ManifestAccumulator.Add(setup) error = %v, want nil", gotErr)
				}
				value := second
				value.Entry.Sequence = mustEntrySequence(t, 3)
				return accumulator, value
			}, wantErr: core.ErrChitConflict},
			{name: "verified evidence belongs to another entry", build: func() (*ManifestAccumulator, ManifestAddition) {
				value := fixture.addition
				value.Evidence = other.addition.Evidence
				return NewManifestAccumulator(), value
			}, wantErr: core.ErrChitConflict},
			{name: "unsigned receipt body mutation", build: func() (*ManifestAccumulator, ManifestAddition) {
				value := fixture.addition
				value.Entry.Evidence.Payload.Body = other.addition.Entry.Evidence.Payload.Body
				return NewManifestAccumulator(), value
			}, wantErr: core.ErrChitConflict},
			{name: "add after successful seal", build: func() (*ManifestAccumulator, ManifestAddition) {
				accumulator := NewManifestAccumulator()
				if gotErr := accumulator.Add(fixture.addition); gotErr != nil {
					t.Fatalf("ManifestAccumulator.Add(setup) error = %v, want nil", gotErr)
				}
				if _, gotErr := accumulator.Seal(); gotErr != nil {
					t.Fatalf("ManifestAccumulator.Seal(setup) error = %v, want nil", gotErr)
				}
				return accumulator, second
			}, wantErr: core.ErrChitContract},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				accumulator, addition := tc.build()
				gotErr := accumulator.Add(addition)
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("ManifestAccumulator.Add() error = %v, want errors.Is %v", gotErr, tc.wantErr)
				}
			})
		}

		t.Run("total extent overflow leaves prior fold sealable", func(t *testing.T) {
			t.Parallel()

			maximum := mustChitByteLength(t, math.MaxInt64)
			large := chitEvidenceEntryFixture(t, chitEntryFixtureRequest{
				Private: fixture.private, Trusted: fixture.trusted, Scope: fixture.scope,
				Marker: 0x51, Sequence: 1, Name: chitFixtureNameA, Extent: &maximum,
			})
			one := chitManifestEntryFixture(t, chitDerivedEntryFixtureRequest{
				Fixture: fixture, Marker: 0x52, Sequence: 2, Name: chitFixtureNameB,
			})
			accumulator := NewManifestAccumulator()
			if gotErr := accumulator.Add(large); gotErr != nil {
				t.Fatalf("ManifestAccumulator.Add(maximum setup) error = %v, want nil", gotErr)
			}
			if gotErr := accumulator.Add(one); !errors.Is(gotErr, core.ErrNumericOverflow) {
				t.Fatalf("ManifestAccumulator.Add(one above total maximum) error = %v, want errors.Is %v", gotErr, core.ErrNumericOverflow)
			}
			got, gotErr := accumulator.Seal()
			if gotErr != nil || got.Objects.Uint64() != 1 || got.TotalBytes.Uint64() != math.MaxInt64 {
				t.Fatalf("ManifestAccumulator.Seal(after refused overflow) = (%v, %v), want one-object maximum summary", got, gotErr)
			}
		})
	})

	t.Run("neutral empty and repeated seals emit no plausible summary", func(t *testing.T) {
		t.Parallel()

		accumulator := NewManifestAccumulator()
		got, gotErr := accumulator.Seal()
		if !errors.Is(gotErr, core.ErrChitContract) || got != (ManifestSummary{}) {
			t.Fatalf("empty ManifestAccumulator.Seal() = (%v, %v), want zero and errors.Is %v", got, gotErr, core.ErrChitContract)
		}
		got, gotErr = accumulator.Seal()
		if !errors.Is(gotErr, core.ErrChitContract) || got != (ManifestSummary{}) {
			t.Fatalf("repeated empty ManifestAccumulator.Seal() = (%v, %v), want zero and errors.Is %v", got, gotErr, core.ErrChitContract)
		}
	})
}

func TestManifestDigestChangesWhenAuthenticatedOrderChanges(t *testing.T) {
	t.Parallel()

	fixture := newChitFixture(t, 0x22, 1)
	second := chitManifestEntryFixture(t, chitDerivedEntryFixtureRequest{
		Fixture: fixture, Marker: 0x32, Sequence: 2, Name: chitFixtureNameB,
	})
	firstOrder := manifestSummaryFixture(t, fixture.addition, second)

	secondFirst := second
	secondFirst.Entry.Sequence = mustEntrySequence(t, 1)
	firstSecond := fixture.addition
	firstSecond.Entry.Sequence = mustEntrySequence(t, 2)
	secondOrder := manifestSummaryFixture(t, secondFirst, firstSecond)
	if firstOrder.Digest == secondOrder.Digest {
		t.Fatalf("manifest digests are equal after authenticated order reversal, want distinct")
	}
}

func TestChitIssuanceLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive exact retries across the version domain converge on the persisted signed document", func(t *testing.T) {
		t.Parallel()

		versions := []uint64{1, 2, 3, 4, 5, 99, 100, 101, math.MaxUint32, math.MaxUint64}
		for index, version := range versions {
			fixture := newChitFixture(t, byte(0x71+index), version)
			prior := fixture.document
			got, gotErr := Issue(Issuance{
				Existing: &prior, Signer: fixture.private, TrustedKeys: fixture.trusted,
				Payload: fixture.document.Payload,
			})
			if gotErr != nil || got != prior {
				t.Fatalf("Issue(exact retry version %d) = (%v, %v), want persisted %v and nil", version, got, gotErr, prior)
			}
		}
	})

	t.Run("negative reuse of one logical version with any changed content conflicts", func(t *testing.T) {
		t.Parallel()

		fixture := newChitFixture(t, 0x51, 1)
		other := newChitFixture(t, 0x61, 2)
		cases := []struct {
			name   string
			mutate func(*Payload)
		}{
			{name: "chit identity changed", mutate: func(value *Payload) { value.Identity = other.identity }},
			{name: "collection identity changed", mutate: func(value *Payload) { value.Collection = other.document.Payload.Collection }},
			{name: "account scope changed", mutate: func(value *Payload) { value.Scope.Account = other.scope.Account }},
			{name: "offering scope changed", mutate: func(value *Payload) { value.Scope.Offering = other.scope.Offering }},
			{name: "manifest closure changed", mutate: func(value *Payload) { value.Manifest = other.summary }},
			{name: "acceptance instant moved forward", mutate: func(value *Payload) { value.AcceptedAt = temporal.InstantFromNanoseconds(83) }},
			{name: "acceptance instant moved backward", mutate: func(value *Payload) { value.AcceptedAt = temporal.InstantFromNanoseconds(80) }},
			{name: "retention promise extended", mutate: func(value *Payload) { value.RetainUntil = temporal.InstantFromNanoseconds(500) }},
			{name: "retention promise shortened", mutate: func(value *Payload) { value.RetainUntil = temporal.InstantFromNanoseconds(100) }},
			{name: "version advanced in occupied slot", mutate: func(value *Payload) { value.Version = mustVersion(t, 2) }},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				candidate := fixture.document.Payload
				tc.mutate(&candidate)
				if candidate == fixture.document.Payload {
					t.Fatalf("mutated candidate = %v, want value distinct from persisted %v", candidate, fixture.document.Payload)
				}
				if err := candidate.Validate(); err != nil {
					t.Fatalf("mutated candidate Validate() error = %v, want nil", err)
				}
				prior := fixture.document
				got, gotErr := Issue(Issuance{
					Existing: &prior, Signer: fixture.private, TrustedKeys: fixture.trusted,
					Payload: candidate,
				})
				if !errors.Is(gotErr, core.ErrChitConflict) || got != (Document{}) {
					t.Fatalf("Issue(changed occupied version) = (%v, %v), want zero and errors.Is %v", got, gotErr, core.ErrChitConflict)
				}
			})
		}
	})

	t.Run("neutral zero and unauthenticated prior state produce no signed chit", func(t *testing.T) {
		t.Parallel()

		if got, gotErr := Issue(Issuance{}); !errors.Is(gotErr, core.ErrChitContract) || got != (Document{}) {
			t.Fatalf("Issue(zero) = (%v, %v), want zero and errors.Is %v", got, gotErr, core.ErrChitContract)
		}

		fixture := newChitFixture(t, 0x31, 1)
		other := newChitFixture(t, 0x41, 1)
		got, gotErr := Issue(Issuance{
			Signer: fixture.private, TrustedKeys: other.trusted,
			Payload: fixture.document.Payload,
		})
		if !errors.Is(gotErr, core.ErrChitVerification) || got != (Document{}) {
			t.Fatalf("Issue(fresh untrusted signer) = (%v, %v), want zero and errors.Is %v", got, gotErr, core.ErrChitVerification)
		}

		forged := fixture.document
		forged.Attestation.Signature = other.document.Attestation.Signature
		got, gotErr = Issue(Issuance{
			Existing: &forged, Signer: fixture.private, TrustedKeys: fixture.trusted,
			Payload: fixture.document.Payload,
		})
		if !errors.Is(gotErr, core.ErrChitVerification) || got != (Document{}) {
			t.Fatalf("Issue(unauthenticated prior) = (%v, %v), want zero and errors.Is %v", got, gotErr, core.ErrChitVerification)
		}
	})
}

func TestChitVerificationLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive independent authentic versions return exact signed chits", func(t *testing.T) {
		t.Parallel()

		versions := []uint64{1, 2, 3, 4, 5, 99, 100, 101, math.MaxUint32, math.MaxUint64}
		for index, version := range versions {
			fixture := newChitFixture(t, byte(0x23+index), version)
			verified, gotErr := Verify(Verification{
				Document: fixture.document, Expected: Expectation{Identity: fixture.identity, Scope: fixture.scope},
				TrustedKeys: fixture.trusted,
			})
			got, documentErr := verified.Document()
			if gotErr != nil || documentErr != nil || got != fixture.document {
				t.Fatalf("Verify(version %d)/Document() = (%v, %v, %v), want exact document and nil", version, got, gotErr, documentErr)
			}
		}
	})

	t.Run("negative expectation authority envelope and every signed payload fact reject", func(t *testing.T) {
		t.Parallel()

		fixture := newChitFixture(t, 0x43, 1)
		other := newChitFixture(t, 0x53, 2)
		cases := []struct {
			wantErr error
			mutate  func(*Verification)
			name    string
		}{
			{name: "zero verification", mutate: func(value *Verification) { *value = Verification{} }, wantErr: core.ErrChitContract},
			{name: "expected identity substituted", mutate: func(value *Verification) { value.Expected.Identity = other.identity }, wantErr: core.ErrChitConflict},
			{name: "expected scope substituted", mutate: func(value *Verification) { value.Expected.Scope = other.scope }, wantErr: core.ErrChitConflict},
			{name: "authority trust set substituted", mutate: func(value *Verification) { value.TrustedKeys = other.trusted }, wantErr: core.ErrChitVerification},
			{name: "signed identity substituted", mutate: func(value *Verification) { value.Document.Payload.Identity = other.document.Payload.Identity }, wantErr: core.ErrChitVerification},
			{name: "signed collection substituted", mutate: func(value *Verification) { value.Document.Payload.Collection = other.document.Payload.Collection }, wantErr: core.ErrChitVerification},
			{name: "signed scope substituted", mutate: func(value *Verification) { value.Document.Payload.Scope = other.document.Payload.Scope }, wantErr: core.ErrChitVerification},
			{name: "signed manifest substituted", mutate: func(value *Verification) { value.Document.Payload.Manifest = other.document.Payload.Manifest }, wantErr: core.ErrChitVerification},
			{name: "signed acceptance instant substituted", mutate: func(value *Verification) { value.Document.Payload.AcceptedAt = temporal.InstantFromNanoseconds(1) }, wantErr: core.ErrChitVerification},
			{name: "signed retention instant substituted", mutate: func(value *Verification) { value.Document.Payload.RetainUntil = temporal.InstantFromNanoseconds(1_000) }, wantErr: core.ErrChitVerification},
			{name: "signed version substituted", mutate: func(value *Verification) { value.Document.Payload.Version = other.document.Payload.Version }, wantErr: core.ErrChitVerification},
			{name: "signing domain substituted", mutate: func(value *Verification) { value.Document.Attestation.Domain = SigningDomainCatalogV1 }, wantErr: core.ErrChitVerification},
			{name: "signer substituted", mutate: func(value *Verification) { value.Document.Attestation.Signer = other.document.Attestation.Signer }, wantErr: core.ErrChitVerification},
			{name: "signature substituted", mutate: func(value *Verification) { value.Document.Attestation.Signature = other.document.Attestation.Signature }, wantErr: core.ErrChitVerification},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				input := Verification{
					Document: fixture.document, Expected: Expectation{Identity: fixture.identity, Scope: fixture.scope},
					TrustedKeys: fixture.trusted,
				}
				tc.mutate(&input)
				got, gotErr := Verify(input)
				if !errors.Is(gotErr, tc.wantErr) || got != (Verified{}) {
					t.Fatalf("Verify() = (%v, %v), want zero and errors.Is %v", got, gotErr, tc.wantErr)
				}
			})
		}
	})

	t.Run("neutral zero verified value discloses no chit", func(t *testing.T) {
		t.Parallel()

		got, gotErr := (Verified{}).Document()
		if !errors.Is(gotErr, core.ErrChitVerification) || got != (Document{}) {
			t.Fatalf("zero Verified.Document() = (%v, %v), want zero and errors.Is %v", got, gotErr, core.ErrChitVerification)
		}
	})
}

func TestChitRetentionAndCatalogHostileTemporalEdges(t *testing.T) {
	t.Parallel()

	fixture := newChitFixture(t, 0x25, 1)
	retention := fixture.document.Payload.RetainUntil
	retentionNanoseconds, err := retention.Nanoseconds()
	if err != nil {
		t.Fatalf("RetainUntil.Nanoseconds() error = %v, want nil", err)
	}
	before := temporal.InstantFromNanoseconds(retentionNanoseconds - 1)
	after := temporal.InstantFromNanoseconds(retentionNanoseconds + 1)
	cases := []struct {
		wantErr  error
		name     string
		observed temporal.Instant
		state    CustodyState
	}{
		{name: "stored one nanosecond before retention", state: CustodyStateStored, observed: before},
		{name: "stored exactly at retention", state: CustodyStateStored, observed: retention},
		{name: "stored one nanosecond after retention", state: CustodyStateStored, observed: after},
		{name: "retrieval unavailable one nanosecond before retention", state: CustodyStateRetrievalUnavailable, observed: before, wantErr: core.ErrChitConflict},
		{name: "retrieval unavailable exactly at retention", state: CustodyStateRetrievalUnavailable, observed: retention},
		{name: "retrieval unavailable one nanosecond after retention", state: CustodyStateRetrievalUnavailable, observed: after},
		{name: "deleted one nanosecond before retention", state: CustodyStateDeleted, observed: before, wantErr: core.ErrChitConflict},
		{name: "deleted exactly at retention", state: CustodyStateDeleted, observed: retention},
		{name: "deleted one nanosecond after retention", state: CustodyStateDeleted, observed: after},
		{name: "zero custody state", state: CustodyStateUnknown, observed: retention, wantErr: core.ErrChitContract},
		{name: "zero observation instant", state: CustodyStateStored, observed: temporal.Instant{}, wantErr: core.ErrChitContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := (CatalogEntry{Chit: fixture.document, State: tc.state}).ValidateAt(tc.observed)
			if tc.wantErr == nil && gotErr != nil {
				t.Fatalf("CatalogEntry.ValidateAt() error = %v, want nil", gotErr)
			}
			if tc.wantErr != nil && !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("CatalogEntry.ValidateAt() error = %v, want errors.Is %v", gotErr, tc.wantErr)
			}
		})
	}

	request := catalogQueryPayload(t, fixture.scope, 0x31)
	commitment, err := CommitQuery(request)
	if err != nil {
		t.Fatalf("CommitQuery() error = %v, want nil", err)
	}
	payload := CatalogPayload{
		Entries: []CatalogEntry{}, Scope: fixture.scope,
		Watermark: chitWatermarkFixture(t, fixture.scope), ObservedAt: before,
		Request: commitment, Continuation: End(),
	}
	if gotErr := payload.Validate(); gotErr != nil {
		t.Fatalf("empty terminal CatalogPayload.Validate() error = %v, want nil", gotErr)
	}
	payload.Entries = nil
	if gotErr := payload.Validate(); !errors.Is(gotErr, core.ErrChitContract) {
		t.Fatalf("nil CatalogPayload entries error = %v, want errors.Is %v", gotErr, core.ErrChitContract)
	}

	payload.Entries = make([]CatalogEntry, core.CatalogPageMaximumEntries+1)
	if gotErr := payload.Validate(); !errors.Is(gotErr, core.ErrChitContract) {
		t.Fatalf("oversize CatalogPayload entries error = %v, want errors.Is %v", gotErr, core.ErrChitContract)
	}
}

func TestChitNumericJSONBoundariesAreCanonicalAndTransactional(t *testing.T) {
	t.Parallel()

	before := mustVersion(t, math.MaxUint64)
	cases := []struct {
		name string
		data []byte
	}{
		{name: "zero", data: []byte{'0'}},
		{name: "leading zero", data: []byte{'0', '1'}},
		{name: "leading whitespace", data: []byte{' ', '1'}},
		{name: "negative", data: []byte{'-', '1'}},
		{name: "fraction", data: []byte{'1', '.', '0'}},
		{name: "uint64 overflow", data: []byte("18446744073709551616")},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := before
			gotErr := got.UnmarshalJSON(testCase.data)
			if !errors.Is(gotErr, core.ErrJSONContract) || got != before {
				t.Fatalf("Version.UnmarshalJSON(%q) = (%v, %v), want preserved %v and errors.Is %v",
					testCase.data, got, gotErr, before, core.ErrJSONContract)
			}
		})
	}
}

func newChitFixture(t testing.TB, marker byte, versionValue uint64) chitFixture {
	t.Helper()

	private, trusted := chitSigningFixture(t, marker)
	scope := chitScopeFixture(t, marker+1)
	addition := chitEvidenceEntryFixture(t, chitEntryFixtureRequest{
		Private: private, Trusted: trusted, Scope: scope,
		Marker: marker + 2, Sequence: 1, Name: chitFixtureNameA,
	})
	summary := manifestSummaryFixture(t, addition)
	identity := mustChitID(t, marker+3, int64(marker)+1)
	collection, err := NewCollectionID(mustUUIDv7(t, marker+4, int64(marker)+2))
	if err != nil {
		t.Fatalf("NewCollectionID() error = %v, want nil", err)
	}
	accepted := temporal.InstantFromNanoseconds(int64(marker) + 1)
	retained := temporal.InstantFromNanoseconds(int64(marker) + 100)
	payload := Payload{
		Identity: identity, Collection: collection, Scope: scope, Manifest: summary,
		AcceptedAt: accepted, RetainUntil: retained, Version: mustVersion(t, versionValue),
	}
	document, err := Issue(Issuance{Signer: private, TrustedKeys: trusted, Payload: payload})
	if err != nil {
		t.Fatalf("chit.Issue() error = %v, want nil", err)
	}
	return chitFixture{
		private: private, trusted: trusted, scope: scope, identity: identity,
		document: document, summary: summary, addition: addition,
	}
}

func chitManifestEntryFixture(
	t testing.TB,
	request chitDerivedEntryFixtureRequest,
) ManifestAddition {
	t.Helper()
	return chitEvidenceEntryFixture(t, chitEntryFixtureRequest{
		Private: request.Fixture.private, Trusted: request.Fixture.trusted,
		Scope: request.Fixture.scope, Marker: request.Marker,
		Sequence: request.Sequence, Name: request.Name,
	})
}

func chitEvidenceEntryFixture(
	t testing.TB,
	request chitEntryFixtureRequest,
) ManifestAddition {
	t.Helper()

	payload := []byte{request.Marker}
	extent, err := core.NewByteLength(uint64(len(payload)))
	if err != nil {
		t.Fatalf("core.NewByteLength() error = %v, want nil", err)
	}
	if request.Extent != nil {
		extent = *request.Extent
		if err := extent.Validate(); err != nil {
			t.Fatalf("requested evidence extent Validate() error = %v, want nil", err)
		}
	}
	submission := mustLifecycleIdentity(t, request.Marker+1, receipt.NewSubmissionIdentity)
	object := mustLifecycleIdentity(t, request.Marker+2, receipt.NewObjectIdentity)
	receiptIDBytes := [receipt.ReceiptIDBytes]byte{}
	receiptIDBytes[0] = request.Marker + 3
	receiptID, err := receipt.NewReceiptID(receiptIDBytes)
	if err != nil {
		t.Fatalf("receipt.NewReceiptID() error = %v, want nil", err)
	}
	evidence, err := receipt.IssueEvidence(receipt.IssueEvidenceRequest{
		Key: request.Private, Identity: receiptID,
		Account: request.Scope.Account, Offering: request.Scope.Offering,
		OccurredAt: temporal.InstantFromNanoseconds(int64(request.Marker)),
		Body: receipt.EvidenceBody{
			Extent: extent, SHA256: core.SHA256Of(payload),
			CRC32C:     core.NewCRC32C(crc32.Checksum(payload, crc32.MakeTable(crc32.Castagnoli))),
			Submission: submission, Object: object,
		},
	})
	if err != nil {
		t.Fatalf("receipt.IssueEvidence() error = %v, want nil", err)
	}
	verified, err := receipt.VerifyEvidence(receipt.VerifyEvidenceRequest{
		Document: evidence, TrustedKeys: request.Trusted,
		Expected: receipt.EvidenceExpectation{
			Account: request.Scope.Account, Offering: request.Scope.Offering,
			Body: evidence.Payload.Body,
		},
	})
	if err != nil {
		t.Fatalf("receipt.VerifyEvidence() error = %v, want nil", err)
	}
	path, err := ParseEntryName(request.Name)
	if err != nil {
		t.Fatalf("ParseEntryName(%q) error = %v, want nil", request.Name, err)
	}
	return ManifestAddition{
		Evidence: verified,
		Entry: ManifestEntry{
			Evidence: evidence, Name: path, ContentType: core.HTTPMediaTypeOctetStream(),
			Sequence: mustEntrySequence(t, request.Sequence),
		},
	}
}

func manifestSummaryFixture(t testing.TB, additions ...ManifestAddition) ManifestSummary {
	t.Helper()

	accumulator := NewManifestAccumulator()
	for _, addition := range additions {
		if err := accumulator.Add(addition); err != nil {
			t.Fatalf("ManifestAccumulator.Add() error = %v, want nil", err)
		}
	}
	summary, err := accumulator.Seal()
	if err != nil {
		t.Fatalf("ManifestAccumulator.Seal() error = %v, want nil", err)
	}
	return summary
}

func chitSigningFixture(t testing.TB, marker byte) (ed25519.PrivateKey, attest.TrustedKeys) {
	t.Helper()

	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{marker}, ed25519.SeedSize))
	public, err := core.NewEd25519PublicKey(private.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatalf("core.NewEd25519PublicKey() error = %v, want nil", err)
	}
	trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{Keys: []core.Ed25519PublicKey{public}})
	if err != nil {
		t.Fatalf("attest.NewTrustedKeys() error = %v, want nil", err)
	}
	return private, trusted
}

func chitScopeFixture(t testing.TB, marker byte) receipt.Scope {
	t.Helper()
	return receipt.Scope{
		Account:  mustLifecycleIdentity(t, marker, receipt.NewAccountIdentity),
		Offering: mustOfferingIdentity(t, marker+1),
	}
}

func mustOfferingIdentity(t testing.TB, marker byte) receipt.OfferingIdentity {
	t.Helper()
	offerings := [...]core.Offering{core.OfferingBug, core.OfferingWitness, core.OfferingPeachfuzz}
	offering := offerings[int(marker)%len(offerings)]
	identity, err := receipt.OfferingIdentityFor(offering)
	if err != nil {
		t.Fatalf("receipt.OfferingIdentityFor(%v) error = %v, want nil", offering, err)
	}
	return identity
}

func mustLifecycleIdentity[T core.Validatable](
	t testing.TB,
	marker byte,
	constructor func([receipt.LifecycleIdentityBytes]byte) (T, error),
) T {
	t.Helper()

	value := [receipt.LifecycleIdentityBytes]byte{}
	value[0] = marker
	identity, err := constructor(value)
	if err != nil {
		t.Fatalf("lifecycle identity constructor error = %v, want nil", err)
	}
	return identity
}

func mustUUIDv7(t testing.TB, marker byte, milliseconds int64) id.UUIDv7 {
	t.Helper()

	material, err := core.NewSecretMaterial(bytes.Repeat([]byte{marker}, core.SecretMaterialMinimumBytes))
	if err != nil {
		t.Fatalf("core.NewSecretMaterial() error = %v, want nil", err)
	}
	observation, err := temporal.NewObservation(time.UnixMilli(milliseconds))
	if err != nil {
		t.Fatalf("temporal.NewObservation() error = %v, want nil", err)
	}
	identity, err := id.NewUUIDv7(id.Request{Entropy: material, Observation: observation})
	destroyErr := material.Destroy()
	if err != nil || destroyErr != nil {
		t.Fatalf("id.NewUUIDv7()/SecretMaterial.Destroy() errors = (%v, %v), want nil", err, destroyErr)
	}
	return identity
}

func mustChitID(t testing.TB, marker byte, milliseconds int64) ChitID {
	t.Helper()
	identity, err := NewChitID(mustUUIDv7(t, marker, milliseconds))
	if err != nil {
		t.Fatalf("NewChitID() error = %v, want nil", err)
	}
	return identity
}

func mustVersion(t testing.TB, value uint64) Version {
	t.Helper()
	version, err := NewVersion(value)
	if err != nil {
		t.Fatalf("NewVersion(%d) error = %v, want nil", value, err)
	}
	return version
}

func mustEntrySequence(t testing.TB, value uint64) EntrySequence {
	t.Helper()
	sequence, err := NewEntrySequence(value)
	if err != nil {
		t.Fatalf("NewEntrySequence(%d) error = %v, want nil", value, err)
	}
	return sequence
}

func mustChitByteLength(t testing.TB, value uint64) core.ByteLength {
	t.Helper()

	length, err := core.NewByteLength(value)
	if err != nil {
		t.Fatalf("core.NewByteLength(%d) error = %v, want nil", value, err)
	}
	return length
}

func chitWatermarkFixture(t testing.TB, scope receipt.Scope) receipt.Watermark {
	t.Helper()

	generation, err := receipt.NewGeneration(1)
	if err != nil {
		t.Fatalf("receipt.NewGeneration() error = %v, want nil", err)
	}
	cursor, err := receipt.NewCursorDigest(core.SHA256Of([]byte{1}))
	if err != nil {
		t.Fatalf("receipt.NewCursorDigest() error = %v, want nil", err)
	}
	chain, err := receipt.NewChainHash(core.SHA256Of([]byte{2}))
	if err != nil {
		t.Fatalf("receipt.NewChainHash() error = %v, want nil", err)
	}
	watermark, err := receipt.NewWatermark(receipt.WatermarkRequest{
		Generation: generation, Scope: scope, CursorDigest: cursor, ChainHash: chain,
	})
	if err != nil {
		t.Fatalf("receipt.NewWatermark() error = %v, want nil", err)
	}
	return watermark
}
