package retrieval

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/chit"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestGrantDocumentJSONLayerTriad(t *testing.T) {
	t.Parallel()

	fixture := newDownloadCallFixture(t, downloadCallFixtureRequest{Payload: []byte{0x11, 0x12}})
	projection, gotErr := IssueGrant(GrantIssuance{
		Signer: fixture.private, Capability: fixture.capability, Payload: fixture.grantPayload,
		Entry: fixture.membership, Chit: fixture.chit, Request: fixture.request,
	})
	if gotErr != nil {
		t.Fatalf("IssueGrant() setup error = %v, want nil", gotErr)
	}
	canonical, gotErr := projection.MarshalJSON()
	if gotErr != nil {
		t.Fatalf("GrantProjection.MarshalJSON() setup error = %v, want nil", gotErr)
	}
	strict, gotErr := core.EncodeValidatedJSON(projection, core.DefaultStrictJSONLimits())
	if gotErr != nil || !bytes.Equal(strict, canonical) {
		t.Fatalf("core.EncodeValidatedJSON(GrantProjection) = (%d bytes, %v), want exact %d-byte receive-only projection", len(strict), gotErr, len(canonical))
	}

	t.Run("positive exact bearer document and extent boundaries preserve facts", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			data []byte
		}{
			{name: "canonical issuer projection", data: canonical},
			{name: "leading whitespace", data: append([]byte(" \n\t"), canonical...)},
			{name: "trailing whitespace", data: append(append([]byte(nil), canonical...), ' ', '\n', '\t')},
			{name: "both-side whitespace", data: append(append([]byte(" \n"), canonical...), '\n', ' ')},
			{name: "top-level members reordered", data: marshalReorderedGrantProjection(t, projection)},
			{name: "one below document ceiling", data: grantPadJSON(canonical, GrantDocumentJSONMaximumBytes-1)},
			{name: "at document ceiling", data: grantPadJSON(canonical, GrantDocumentJSONMaximumBytes)},
			{name: "one trailing carriage return", data: append(append([]byte(nil), canonical...), '\r')},
			{name: "four leading whitespace forms", data: append([]byte("\t\r\n "), canonical...)},
			{name: "four trailing whitespace forms", data: append(append([]byte(nil), canonical...), " \n\r\t"...)},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				var got GrantDocument
				gotErr := got.UnmarshalJSON(tc.data)
				if gotErr != nil || got.Validate() != nil {
					t.Fatalf("GrantDocument.UnmarshalJSON() = (%v, %v), want valid bearer document and nil", got, gotErr)
				}
			})
		}
	})

	t.Run("negative malformed missing duplicate and type-wrong bearer documents reject", func(t *testing.T) {
		t.Parallel()

		before := fixture.document
		cases := []struct {
			name string
			data []byte
		}{
			{name: "empty document", data: nil},
			{name: "whitespace-only document", data: []byte(" \n\t")},
			{name: "null document", data: []byte("null")},
			{name: "array instead of structure", data: []byte("[]")},
			{name: "string instead of structure", data: []byte(`"grant"`)},
			{name: "number instead of structure", data: []byte("1")},
			{name: "boolean instead of structure", data: []byte("true")},
			{name: "truncated opening brace", data: []byte("{")},
			{name: "truncated inside bearer", data: canonical[:len(canonical)/2]},
			{name: "truncated before final brace", data: canonical[:len(canonical)-1]},
			{name: "trailing object", data: append(append([]byte(nil), canonical...), '{', '}')},
			{name: "two concatenated documents", data: append(append([]byte(nil), canonical...), canonical...)},
			{name: "unknown top-level member", data: bytes.Replace(canonical, []byte(`{"capability"`), []byte(`{"unknown":1,"capability"`), 1)},
			{name: "duplicate capability member", data: bytes.Replace(canonical, []byte(`{"capability":`), []byte(`{"capability":null,"capability":`), 1)},
			{name: "missing every member", data: []byte("{}")},
			{name: "missing capability", data: []byte(`{"payload":null,"attestation":null}`)},
			{name: "missing payload", data: []byte(`{"capability":null,"attestation":null}`)},
			{name: "missing attestation", data: []byte(`{"capability":null,"payload":null}`)},
			{name: "capability has wrong scalar type", data: []byte(`{"capability":1,"payload":null,"attestation":null}`)},
			{name: "payload has wrong scalar type", data: []byte(`{"capability":null,"payload":1,"attestation":null}`)},
			{name: "attestation has wrong scalar type", data: []byte(`{"capability":null,"payload":null,"attestation":1}`)},
			{name: "one above document ceiling", data: grantPadJSON(canonical, GrantDocumentJSONMaximumBytes+1)},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got := before
				gotErr := got.UnmarshalJSON(tc.data)
				if !errors.Is(gotErr, core.ErrJSONContract) || !sameGrantDocument(got, before) {
					t.Fatalf("GrantDocument.UnmarshalJSON() = (%v, %v), want preserved receiver and errors.Is %v", got, gotErr, core.ErrJSONContract)
				}
			})
		}
	})

	t.Run("neutral rejected input discloses no bearer from zero receiver", func(t *testing.T) {
		t.Parallel()

		var got GrantDocument
		gotErr := got.UnmarshalJSON(nil)
		if !errors.Is(gotErr, core.ErrJSONContract) || !got.Capability.IsZero() || got.Payload != (GrantPayload{}) {
			t.Fatalf("zero GrantDocument.UnmarshalJSON(nil) = (%v, %v), want zero bearer and errors.Is %v", got, gotErr, core.ErrJSONContract)
		}
	})
}

func TestGrantIssuanceLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive exact authenticated inputs issue verifiable bearer documents", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			size int
		}{
			{name: "one byte", size: 1},
			{name: "two bytes", size: 2},
			{name: "three bytes", size: 3},
			{name: "one below first stream boundary", size: 32<<10 - 1},
			{name: "at first stream boundary", size: 32 << 10},
			{name: "one above first stream boundary", size: 32<<10 + 1},
			{name: "one below second stream boundary", size: 64<<10 - 1},
			{name: "at second stream boundary", size: 64 << 10},
			{name: "one above second stream boundary", size: 64<<10 + 1},
			{name: "one above third stream boundary", size: 96<<10 + 1},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				fixture := newDownloadCallFixture(t, downloadCallFixtureRequest{Payload: bytes.Repeat([]byte{0x4d}, tc.size)})
				got, gotErr := IssueGrant(GrantIssuance{
					Signer: fixture.private, Capability: fixture.capability, Payload: fixture.grantPayload,
					Entry: fixture.membership, Chit: fixture.chit, Request: fixture.request,
				})
				if gotErr != nil || got.Validate() != nil {
					t.Fatalf("IssueGrant(%d-byte entry) = (%v, %v), want valid projection and nil", tc.size, got, gotErr)
				}
				encoded, marshalErr := got.MarshalJSON()
				if marshalErr != nil {
					t.Fatalf("GrantProjection.MarshalJSON() error = %v, want nil", marshalErr)
				}
				var received GrantDocument
				if decodeErr := received.UnmarshalJSON(encoded); decodeErr != nil {
					t.Fatalf("GrantDocument.UnmarshalJSON() error = %v, want nil", decodeErr)
				}
				verified, verifyErr := VerifyGrant(GrantExpectation{
					Document: received, Request: fixture.request, Chit: fixture.chit,
					ObservedAt: temporal.InstantFromNanoseconds(retrievalGrantObserved), TrustedKeys: fixture.trusted,
				})
				if verifyErr != nil || verified.Validate() != nil {
					t.Fatalf("VerifyGrant(issued projection) = (%v, %v), want authenticated grant and nil", verified, verifyErr)
				}
			})
		}
	})

	t.Run("negative unset and cross-bound inputs issue no projection", func(t *testing.T) {
		t.Parallel()

		fixture := newDownloadCallFixture(t, downloadCallFixtureRequest{Payload: []byte{0x01}})
		other := newDownloadCallFixture(t, downloadCallFixtureRequest{Payload: []byte{0x02, 0x03}})
		expanded := other.addition
		expanded.Entry.Sequence = grantEntrySequence(t, 2)
		expandedManifest := chit.NewManifestAccumulator()
		expandedVerifier, err := chit.NewManifestEntryVerifier(fixture.addition.Entry.Sequence)
		if err != nil {
			t.Fatalf("chit.NewManifestEntryVerifier(expanded) error = %v, want nil", err)
		}
		for _, addition := range []chit.ManifestAddition{fixture.addition, expanded} {
			if gotErr := expandedManifest.Add(addition); gotErr != nil {
				t.Fatalf("expanded ManifestAccumulator.Add() error = %v, want nil", gotErr)
			}
			if gotErr := expandedVerifier.Add(addition); gotErr != nil {
				t.Fatalf("expanded ManifestEntryVerifier.Add() error = %v, want nil", gotErr)
			}
		}
		expandedSummary, err := expandedManifest.Seal()
		if err != nil {
			t.Fatalf("expanded ManifestAccumulator.Seal() error = %v, want nil", err)
		}
		expandedMembership, err := expandedVerifier.Seal(expandedSummary)
		if err != nil {
			t.Fatalf("expanded ManifestEntryVerifier.Seal() error = %v, want nil", err)
		}
		cases := []struct {
			wantErr error
			mutate  func(*GrantIssuance)
			name    string
		}{
			{name: "zero issuance", mutate: func(value *GrantIssuance) { *value = GrantIssuance{} }, wantErr: core.ErrRetrievalContract},
			{name: "nil signer", mutate: func(value *GrantIssuance) { value.Signer = nil }, wantErr: core.ErrRetrievalContract},
			{name: "zero capability", mutate: func(value *GrantIssuance) { value.Capability = objectstore.DownloadCapabilityProjection{} }, wantErr: core.ErrRetrievalContract},
			{name: "zero payload", mutate: func(value *GrantIssuance) { value.Payload = GrantPayload{} }, wantErr: core.ErrRetrievalContract},
			{name: "zero manifest entry proof", mutate: func(value *GrantIssuance) { value.Entry = chit.VerifiedManifestEntry{} }, wantErr: core.ErrRetrievalContract},
			{name: "zero authenticated chit", mutate: func(value *GrantIssuance) { value.Chit = chit.Verified{} }, wantErr: core.ErrRetrievalContract},
			{name: "zero authenticated request", mutate: func(value *GrantIssuance) { value.Request = RequestPayload{} }, wantErr: core.ErrRetrievalContract},
			{name: "entry from another authenticated manifest", mutate: func(value *GrantIssuance) { value.Entry = other.membership }, wantErr: core.ErrRetrievalBinding},
			{name: "same entry proven under an expanded foreign manifest", mutate: func(value *GrantIssuance) { value.Entry = expandedMembership }, wantErr: core.ErrRetrievalBinding},
			{name: "chit with another manifest", mutate: func(value *GrantIssuance) { value.Chit = other.chit }, wantErr: core.ErrRetrievalBinding},
			{name: "payload names another chit", mutate: func(value *GrantIssuance) { value.Payload.Chit = mustRetrievalChitID(t, retrievalFixtureChitB) }, wantErr: core.ErrRetrievalBinding},
			{name: "payload carries another entry", mutate: func(value *GrantIssuance) { value.Payload.Entry = other.addition.Entry }, wantErr: core.ErrRetrievalBinding},
			{name: "payload carries another manifest digest", mutate: func(value *GrantIssuance) { value.Payload.Manifest = other.grantPayload.Manifest }, wantErr: core.ErrRetrievalBinding},
			{name: "payload capability commitment is unset", mutate: func(value *GrantIssuance) { value.Payload.Capability = objectstore.DownloadCapabilityCommitment{} }, wantErr: core.ErrRetrievalContract},
			{name: "payload expiry differs from bearer expiry", mutate: func(value *GrantIssuance) {
				value.Payload.ExpiresAt = temporal.InstantFromNanoseconds(retrievalGrantExpiresAt - 1)
			}, wantErr: core.ErrRetrievalBinding},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				input := GrantIssuance{
					Signer: fixture.private, Capability: fixture.capability, Payload: fixture.grantPayload,
					Entry: fixture.membership, Chit: fixture.chit, Request: fixture.request,
				}
				tc.mutate(&input)
				got, gotErr := IssueGrant(input)
				if !errors.Is(gotErr, tc.wantErr) || !grantProjectionIsZero(got) {
					t.Fatalf("IssueGrant() = (%v, %v), want zero and errors.Is %v", got, gotErr, tc.wantErr)
				}
			})
		}
	})

	t.Run("neutral zero issuance emits no bearer projection", func(t *testing.T) {
		t.Parallel()

		got, gotErr := IssueGrant(GrantIssuance{})
		if !errors.Is(gotErr, core.ErrRetrievalContract) || !grantProjectionIsZero(got) {
			t.Fatalf("IssueGrant(zero) = (%v, %v), want zero and errors.Is %v", got, gotErr, core.ErrRetrievalContract)
		}
	})
}

func TestGrantTraversalAuthenticatesEveryEntryAndExactTermination(t *testing.T) {
	t.Parallel()

	afterOne, err := ContinueAll(grantEntrySequence(t, 1))
	if err != nil {
		t.Fatalf("ContinueAll(1) setup error = %v, want nil", err)
	}
	specificOne, err := Specific(grantEntrySequence(t, 1))
	if err != nil {
		t.Fatalf("Specific(1) setup error = %v, want nil", err)
	}
	specificTwo, err := Specific(grantEntrySequence(t, 2))
	if err != nil {
		t.Fatalf("Specific(2) setup error = %v, want nil", err)
	}
	cases := []struct {
		name    string
		request downloadCallFixtureRequest
	}{
		{
			name: "all start is the sole entry and ends",
			request: downloadCallFixtureRequest{
				Payload: []byte{1}, Selection: StartAll(), EntrySequence: 1,
				ManifestEntries: 1, Continuation: core.CatalogContinuationEnd,
			},
		},
		{
			name: "all start is first of two and continues",
			request: downloadCallFixtureRequest{
				Payload: []byte{1}, Selection: StartAll(), EntrySequence: 1,
				ManifestEntries: 2, Continuation: core.CatalogContinuationMore,
			},
		},
		{
			name: "all after one is second of two and ends",
			request: downloadCallFixtureRequest{
				Payload: []byte{2}, Selection: afterOne, EntrySequence: 2,
				ManifestEntries: 2, Continuation: core.CatalogContinuationEnd,
			},
		},
		{
			name: "specific first entry ends despite later entry",
			request: downloadCallFixtureRequest{
				Payload: []byte{1}, Selection: specificOne, EntrySequence: 1,
				ManifestEntries: 2, Continuation: core.CatalogContinuationEnd,
			},
		},
		{
			name: "specific final entry ends",
			request: downloadCallFixtureRequest{
				Payload: []byte{2}, Selection: specificTwo, EntrySequence: 2,
				ManifestEntries: 2, Continuation: core.CatalogContinuationEnd,
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			fixture := newDownloadCallFixture(t, testCase.request)
			payload, gotErr := fixture.grant.Payload()
			if gotErr != nil || payload.Entry.Sequence.Uint64() != testCase.request.EntrySequence ||
				payload.Continuation != testCase.request.Continuation || payload.Validate() != nil {
				t.Fatalf("VerifiedGrant.Payload() = (%v, %v), want sequence %d continuation %v and nil",
					payload, gotErr, testCase.request.EntrySequence, testCase.request.Continuation)
			}
			continuation, gotErr := fixture.grant.Continuation()
			if gotErr != nil || continuation.State != testCase.request.Continuation || continuation.Validate() != nil {
				t.Fatalf("VerifiedGrant.Continuation() = (%v, %v), want state %v and nil",
					continuation, gotErr, testCase.request.Continuation)
			}
			if testCase.request.Continuation == core.CatalogContinuationEnd {
				if continuation.Selection != (Selection{}) {
					t.Fatalf("ended continuation selection = %v, want zero", continuation.Selection)
				}
				return
			}
			wantSelection, setupErr := ContinueAll(payload.Entry.Sequence)
			if setupErr != nil {
				t.Fatalf("ContinueAll(granted sequence) error = %v, want nil", setupErr)
			}
			if continuation.Selection != wantSelection {
				t.Fatalf("continued selection = %v, want %v", continuation.Selection, wantSelection)
			}
		})
	}
}

func TestGrantContinuationTaggedUnionRefusesEveryContradictoryArm(t *testing.T) {
	t.Parallel()

	afterOne, err := ContinueAll(grantEntrySequence(t, 1))
	if err != nil {
		t.Fatalf("ContinueAll(1) setup error = %v, want nil", err)
	}
	specificOne, err := Specific(grantEntrySequence(t, 1))
	if err != nil {
		t.Fatalf("Specific(1) setup error = %v, want nil", err)
	}
	cases := []GrantContinuation{
		{},
		{State: core.CatalogContinuationEnd, Selection: afterOne},
		{State: core.CatalogContinuationMore},
		{State: core.CatalogContinuationMore, Selection: StartAll()},
		{State: core.CatalogContinuationMore, Selection: specificOne},
		{State: core.CatalogContinuationState(255), Selection: afterOne},
	}
	for _, candidate := range cases {
		if gotErr := candidate.Validate(); !errors.Is(gotErr, core.ErrRetrievalContract) {
			t.Fatalf("GrantContinuation.Validate(%v) error = %v, want errors.Is %v",
				candidate, gotErr, core.ErrRetrievalContract)
		}
	}
	continued := GrantContinuation{State: core.CatalogContinuationMore, Selection: afterOne}
	ended := GrantContinuation{State: core.CatalogContinuationEnd}
	if continued.Validate() != nil || ended.Validate() != nil || continued == ended {
		t.Fatalf("valid continuations = (%v, %v), want distinct valid more and end arms", continued, ended)
	}
	continuation, err := (VerifiedGrant{}).Continuation()
	if !errors.Is(err, core.ErrRetrievalBinding) || continuation != (GrantContinuation{}) {
		t.Fatalf("zero VerifiedGrant.Continuation() = (%v, %v), want zero and errors.Is %v",
			continuation, err, core.ErrRetrievalBinding)
	}
}

func TestGrantIssuanceRefusesEveryTraversalContradiction(t *testing.T) {
	t.Parallel()

	afterOne, err := ContinueAll(grantEntrySequence(t, 1))
	if err != nil {
		t.Fatalf("ContinueAll(1) setup error = %v, want nil", err)
	}
	startMore := newDownloadCallFixture(t, downloadCallFixtureRequest{
		Payload: []byte{1}, Selection: StartAll(), EntrySequence: 1,
		ManifestEntries: 2, Continuation: core.CatalogContinuationMore,
	})
	startEnd := newDownloadCallFixture(t, downloadCallFixtureRequest{
		Payload: []byte{1}, Selection: StartAll(), EntrySequence: 1,
		ManifestEntries: 1, Continuation: core.CatalogContinuationEnd,
	})
	continued := newDownloadCallFixture(t, downloadCallFixtureRequest{
		Payload: []byte{2}, Selection: afterOne, EntrySequence: 2,
		ManifestEntries: 2, Continuation: core.CatalogContinuationEnd,
	})
	specific, err := Specific(grantEntrySequence(t, 1))
	if err != nil {
		t.Fatalf("Specific(1) setup error = %v, want nil", err)
	}
	specificFixture := newDownloadCallFixture(t, downloadCallFixtureRequest{
		Payload: []byte{1}, Selection: specific, EntrySequence: 1,
		ManifestEntries: 2, Continuation: core.CatalogContinuationEnd,
	})
	cases := []struct {
		mutate  func(*GrantIssuance)
		name    string
		fixture downloadCallFixture
	}{
		{
			name: "all start falsely ends before manifest end", fixture: startMore,
			mutate: func(value *GrantIssuance) { value.Payload.Continuation = core.CatalogContinuationEnd },
		},
		{
			name: "all start falsely continues at manifest end", fixture: startEnd,
			mutate: func(value *GrantIssuance) { value.Payload.Continuation = core.CatalogContinuationMore },
		},
		{
			name: "all start returns second entry", fixture: continued,
			mutate: func(value *GrantIssuance) { rebindRetrievalIssuance(t, value, StartAll()) },
		},
		{
			name: "all continuation repeats prior entry", fixture: startMore,
			mutate: func(value *GrantIssuance) { rebindRetrievalIssuance(t, value, afterOne) },
		},
		{
			name: "specific selection claims another entry", fixture: specificFixture,
			mutate: func(value *GrantIssuance) {
				other, setupErr := Specific(grantEntrySequence(t, 2))
				if setupErr != nil {
					t.Fatalf("Specific(2) setup error = %v, want nil", setupErr)
				}
				rebindRetrievalIssuance(t, value, other)
			},
		},
		{
			name: "specific selection falsely advertises continuation", fixture: specificFixture,
			mutate: func(value *GrantIssuance) { value.Payload.Continuation = core.CatalogContinuationMore },
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			input := GrantIssuance{
				Signer: testCase.fixture.private, Capability: testCase.fixture.capability,
				Payload: testCase.fixture.grantPayload, Entry: testCase.fixture.membership,
				Chit: testCase.fixture.chit, Request: testCase.fixture.request,
			}
			testCase.mutate(&input)
			projection, gotErr := IssueGrant(input)
			if !errors.Is(gotErr, core.ErrRetrievalBinding) || !grantProjectionIsZero(projection) {
				t.Fatalf("IssueGrant(contradictory traversal) = (%v, %v), want zero and errors.Is %v",
					projection, gotErr, core.ErrRetrievalBinding)
			}
		})
	}
}

func rebindRetrievalIssuance(t testing.TB, issuance *GrantIssuance, selection Selection) {
	t.Helper()
	issuance.Request.Selection = selection
	commitment, err := CommitRequest(issuance.Request)
	if err != nil {
		t.Fatalf("CommitRequest(rebound selection) error = %v, want nil", err)
	}
	issuance.Payload.Request = commitment
}

func TestGrantVerificationLayerTriad(t *testing.T) {
	t.Parallel()

	fixture := newDownloadCallFixture(t, downloadCallFixtureRequest{Payload: []byte{0x01, 0x02, 0x03}})

	t.Run("positive exact current instants authenticate the same grant", func(t *testing.T) {
		t.Parallel()

		midpoint := retrievalGrantIssuedAt + (retrievalGrantExpiresAt-retrievalGrantIssuedAt)/2
		cases := []struct {
			name       string
			observedAt int64
		}{
			{name: "exact issue instant", observedAt: retrievalGrantIssuedAt},
			{name: "one nanosecond after issue", observedAt: retrievalGrantIssuedAt + 1},
			{name: "two nanoseconds after issue", observedAt: retrievalGrantIssuedAt + 2},
			{name: "one nanosecond before midpoint", observedAt: midpoint - 1},
			{name: "exact midpoint", observedAt: midpoint},
			{name: "one nanosecond after midpoint", observedAt: midpoint + 1},
			{name: "three nanoseconds before expiry", observedAt: retrievalGrantExpiresAt - 3},
			{name: "two nanoseconds before expiry", observedAt: retrievalGrantExpiresAt - 2},
			{name: "one nanosecond before expiry", observedAt: retrievalGrantExpiresAt - 1},
			{name: "fixture observation instant", observedAt: retrievalGrantObserved},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got, gotErr := VerifyGrant(GrantExpectation{
					Document: fixture.document, Request: fixture.request, Chit: fixture.chit,
					ObservedAt: temporal.InstantFromNanoseconds(tc.observedAt), TrustedKeys: fixture.trusted,
				})
				if gotErr != nil || got.Validate() != nil {
					t.Fatalf("VerifyGrant(observed %d) = (%v, %v), want authenticated grant and nil", tc.observedAt, got, gotErr)
				}
			})
		}
	})

	t.Run("negative authority request chit payload and lifetime substitutions reject", func(t *testing.T) {
		t.Parallel()

		_, otherTrusted := retrievalAuthority(t, 0x72)
		otherNonceRequest := newRetrievalRequestFixture(t, retrievalRequestFixtureRequest{Selection: StartAll(), NonceByte: 2}).payload
		otherBuildRequest := newRetrievalRequestFixture(t, retrievalRequestFixtureRequest{Selection: StartAll(), VersionPatch: 2}).payload
		otherChit := newDownloadCallFixture(t, downloadCallFixtureRequest{Payload: []byte{0x09, 0x08, 0x07}}).chit
		otherCommitment, gotErr := CommitRequest(otherNonceRequest)
		if gotErr != nil {
			t.Fatalf("CommitRequest(other nonce setup) error = %v, want nil", gotErr)
		}
		otherAuthorization := grantAuthorityNonce(t, 0x73)
		cases := []struct {
			wantErr error
			mutate  func(*GrantExpectation)
			name    string
		}{
			{name: "zero expectation", mutate: func(value *GrantExpectation) { *value = GrantExpectation{} }, wantErr: core.ErrRetrievalContract},
			{name: "different authority trust set", mutate: func(value *GrantExpectation) { value.TrustedKeys = otherTrusted }, wantErr: core.ErrRetrievalBinding},
			{name: "observation one before issue", mutate: func(value *GrantExpectation) {
				value.ObservedAt = temporal.InstantFromNanoseconds(retrievalGrantIssuedAt - 1)
			}, wantErr: core.ErrRetrievalBinding},
			{name: "observation at expiry", mutate: func(value *GrantExpectation) {
				value.ObservedAt = temporal.InstantFromNanoseconds(retrievalGrantExpiresAt)
			}, wantErr: core.ErrRetrievalBinding},
			{name: "observation one after expiry", mutate: func(value *GrantExpectation) {
				value.ObservedAt = temporal.InstantFromNanoseconds(retrievalGrantExpiresAt + 1)
			}, wantErr: core.ErrRetrievalBinding},
			{name: "request names another chit", mutate: func(value *GrantExpectation) { value.Request.Chit = mustRetrievalChitID(t, retrievalFixtureChitB) }, wantErr: core.ErrRetrievalBinding},
			{name: "request nonce differs from signed commitment", mutate: func(value *GrantExpectation) { value.Request = otherNonceRequest }, wantErr: core.ErrRetrievalBinding},
			{name: "request build differs from signed commitment", mutate: func(value *GrantExpectation) { value.Request = otherBuildRequest }, wantErr: core.ErrRetrievalBinding},
			{name: "authenticated chit has another manifest", mutate: func(value *GrantExpectation) { value.Chit = otherChit }, wantErr: core.ErrRetrievalBinding},
			{name: "signed payload request commitment substituted", mutate: func(value *GrantExpectation) { value.Document.Payload.Request = otherCommitment }, wantErr: core.ErrRetrievalBinding},
			{name: "signed payload authorization nonce substituted", mutate: func(value *GrantExpectation) { value.Document.Payload.Authorization = otherAuthorization }, wantErr: core.ErrRetrievalBinding},
			{name: "signed payload entry sequence substituted", mutate: func(value *GrantExpectation) { value.Document.Payload.Entry.Sequence = grantEntrySequence(t, 2) }, wantErr: core.ErrRetrievalBinding},
			{name: "signed payload expiry contradicts capability", mutate: func(value *GrantExpectation) {
				value.Document.Payload.ExpiresAt = temporal.InstantFromNanoseconds(retrievalGrantExpiresAt - 1)
			}, wantErr: core.ErrRetrievalBinding},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				input := GrantExpectation{
					Document: fixture.document, Request: fixture.request, Chit: fixture.chit,
					ObservedAt: temporal.InstantFromNanoseconds(retrievalGrantObserved), TrustedKeys: fixture.trusted,
				}
				tc.mutate(&input)
				got, gotErr := VerifyGrant(input)
				if !errors.Is(gotErr, tc.wantErr) || !verifiedGrantIsZero(got) {
					t.Fatalf("VerifyGrant() = (%v, %v), want zero and errors.Is %v", got, gotErr, tc.wantErr)
				}
			})
		}
	})

	t.Run("neutral zero verified grant discloses no capability or payload", func(t *testing.T) {
		t.Parallel()

		capability, capabilityErr := (VerifiedGrant{}).Capability()
		payload, payloadErr := (VerifiedGrant{}).Payload()
		if !errors.Is(capabilityErr, core.ErrRetrievalBinding) || !capability.IsZero() ||
			!errors.Is(payloadErr, core.ErrRetrievalBinding) || payload != (GrantPayload{}) {
			t.Fatalf("zero VerifiedGrant accessors = (%v, %v, %v, %v), want zero values and errors.Is %v",
				capability, capabilityErr, payload, payloadErr, core.ErrRetrievalBinding)
		}
	})
}

func verifiedGrantIsZero(value VerifiedGrant) bool {
	return value.document.Capability.IsZero() && value.document.Payload == (GrantPayload{}) &&
		value.document.Attestation.Validate() != nil && value.proof.Validate() != nil
}

func grantProjectionIsZero(value GrantProjection) bool {
	return value.Capability.Validate() != nil && value.Payload == (GrantPayload{}) &&
		value.Attestation.Validate() != nil
}

func marshalReorderedGrantProjection(t *testing.T, projection GrantProjection) []byte {
	t.Helper()

	encoded, gotErr := core.MarshalCanonicalJSONDocument(struct {
		Capability  objectstore.DownloadCapabilityProjection `json:"capability"`
		Payload     GrantPayload                             `json:"payload"`
		Attestation attest.Envelope[SigningDomain]           `json:"attestation"`
	}{Payload: projection.Payload, Attestation: projection.Attestation, Capability: projection.Capability})
	if gotErr != nil {
		t.Fatalf("core.MarshalCanonicalJSONDocument(reordered grant) error = %v, want nil", gotErr)
	}
	return encoded
}

func grantPadJSON(document []byte, wantBytes int) []byte {
	if len(document) >= wantBytes {
		return append([]byte(nil), document...)
	}
	return append(append([]byte(nil), document...), bytes.Repeat([]byte{' '}, wantBytes-len(document))...)
}

func sameGrantDocument(got GrantDocument, want GrantDocument) bool {
	gotCommitment, gotErr := got.Capability.Commitment()
	wantCommitment, wantErr := want.Capability.Commitment()
	return gotErr == nil && wantErr == nil && gotCommitment == wantCommitment &&
		got.Payload == want.Payload && got.Attestation == want.Attestation
}

func grantAuthorityNonce(t *testing.T, marker byte) controlwire.AuthorityNonce {
	t.Helper()

	raw := [controlwire.NonceBytes]byte{marker}
	got, gotErr := controlwire.NewAuthorityNonce(raw)
	if gotErr != nil {
		t.Fatalf("controlwire.NewAuthorityNonce() error = %v, want nil", gotErr)
	}
	return got
}

func grantEntrySequence(t *testing.T, value uint64) chit.EntrySequence {
	t.Helper()

	got, gotErr := chit.NewEntrySequence(value)
	if gotErr != nil {
		t.Fatalf("chit.NewEntrySequence(%d) error = %v, want nil", value, gotErr)
	}
	return got
}
