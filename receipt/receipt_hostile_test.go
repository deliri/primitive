package receipt

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"hash/crc32"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type receiptFixture struct {
	private     ed25519.PrivateKey
	trusted     attest.TrustedKeys
	account     AccountIdentity
	offering    OfferingIdentity
	submission  SubmissionIdentity
	object      ObjectIdentity
	receipt     ReceiptID
	body        EvidenceBody
	occurredAt  temporal.Instant
	expectation EvidenceExpectation
}

func newReceiptFixture(t testing.TB, marker byte) receiptFixture {
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
	account := lifecycleFixture(t, marker+1, NewAccountIdentity)
	offering := offeringFixture(t, marker+2)
	submission := lifecycleFixture(t, marker+3, NewSubmissionIdentity)
	object := lifecycleFixture(t, marker+4, NewObjectIdentity)
	var receiptBytes [ReceiptIDBytes]byte
	receiptBytes[0] = marker + 5
	receipt, err := NewReceiptID(receiptBytes)
	if err != nil {
		t.Fatalf("NewReceiptID() error = %v, want nil", err)
	}
	occurredAt := temporal.InstantFromNanoseconds(int64(marker))
	payload := []byte{marker}
	body := EvidenceBody{
		Submission: submission,
		Object:     object,
		Extent:     mustByteLength(t, uint64(len(payload))),
		SHA256:     core.NewSHA256Digest(sha256.Sum256(payload)),
		CRC32C:     core.NewCRC32C(crc32.Checksum(payload, crc32.MakeTable(crc32.Castagnoli))),
	}
	expectation := EvidenceExpectation{
		Account: account, Offering: offering, Body: body,
	}
	return receiptFixture{
		private: private, trusted: trusted, account: account, offering: offering,
		submission: submission, object: object, receipt: receipt, body: body,
		occurredAt: occurredAt, expectation: expectation,
	}
}

func offeringFixture(t testing.TB, marker byte) OfferingIdentity {
	t.Helper()
	offerings := [...]core.Offering{
		core.OfferingBug,
		core.OfferingWitness,
		core.OfferingPeachfuzz,
	}
	offering := offerings[int(marker)%len(offerings)]
	identity, err := OfferingIdentityFor(offering)
	if err != nil {
		t.Fatalf("OfferingIdentityFor(%v) error = %v, want nil", offering, err)
	}
	return identity
}

func lifecycleFixture[T core.Validatable](
	t testing.TB,
	marker byte,
	construct func([LifecycleIdentityBytes]byte) (T, error),
) T {
	t.Helper()
	var value [LifecycleIdentityBytes]byte
	value[0] = marker
	got, err := construct(value)
	if err != nil {
		t.Fatalf("lifecycle identity constructor error = %v, want nil", err)
	}
	return got
}

func issueFixture(t testing.TB, fixture receiptFixture) EvidenceDocument {
	t.Helper()
	document, err := IssueEvidence(IssueEvidenceRequest{
		Identity: fixture.receipt, Account: fixture.account,
		Offering: fixture.offering, OccurredAt: fixture.occurredAt,
		Body: fixture.body, Key: fixture.private,
	})
	if err != nil {
		t.Fatalf("IssueEvidence() error = %v, want nil", err)
	}
	return document
}

func TestEvidenceIssueVerificationLayerTriad(t *testing.T) {
	t.Parallel()

	fixture := newReceiptFixture(t, 10)
	document := issueFixture(t, fixture)
	verified, err := VerifyEvidence(VerifyEvidenceRequest{
		Document: document, TrustedKeys: fixture.trusted,
		Expected: fixture.expectation,
	})
	if err != nil || verified.Validate() != nil {
		t.Fatalf("VerifyEvidence(authentic) = (%v, %v), want valid and nil", verified, err)
	}
	gotDocument, documentErr := verified.Document()
	gotHeader, headerErr := verified.Header()
	gotBody, bodyErr := verified.Body()
	if documentErr != nil || headerErr != nil || bodyErr != nil {
		t.Fatalf(
			"VerifiedEvidence projection errors = (%v, %v, %v), want all nil",
			documentErr, headerErr, bodyErr,
		)
	}
	if gotDocument != document || gotHeader != document.Payload.Header ||
		gotBody != fixture.body {
		t.Fatalf("VerifiedEvidence projections moved from the authenticated document")
	}

	replayed, err := VerifyEvidence(VerifyEvidenceRequest{
		Document: document, TrustedKeys: fixture.trusted,
		Expected: fixture.expectation,
	})
	gotReplayed, replayErr := replayed.Document()
	if err != nil || replayErr != nil || gotReplayed != document {
		t.Fatalf("VerifyEvidence(replay) = (%v, %v), want exact document and nil", gotReplayed, err)
	}

	wrong := fixture.expectation
	wrong.Body.Object = lifecycleFixture(t, 99, NewObjectIdentity)
	rejected, err := VerifyEvidence(VerifyEvidenceRequest{
		Document: document, TrustedKeys: fixture.trusted, Expected: wrong,
	})
	gotField := requireScopeField(t, err)
	if rejected != (VerifiedEvidence{}) ||
		!errors.Is(err, core.ErrReceiptScope) ||
		gotField != ScopeFieldObject {
		t.Fatalf(
			"VerifyEvidence(wrong object) = (%v, %v, %v), want zero typed object mismatch",
			rejected, err, gotField,
		)
	}
}

// requireScopeField extracts the sealed mismatch field, failing when the error
// is not an authentic Receipt-built scope mismatch.
func requireScopeField(t *testing.T, err error) ScopeField {
	t.Helper()

	var mismatch ScopeMismatch
	if !errors.As(err, &mismatch) {
		t.Fatalf("errors.As(%v, ScopeMismatch) = false, want a sealed mismatch", err)
	}
	got, fieldErr := mismatch.Field()
	if fieldErr != nil {
		t.Fatalf("ScopeMismatch.Field() error = %v, want nil", fieldErr)
	}
	return got
}

// requireConflictReason extracts the sealed conflict reason, failing when the
// error is not an authentic Receipt-built watermark conflict.
func requireConflictReason(t *testing.T, err error) ConflictReason {
	t.Helper()

	var conflict WatermarkConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("errors.As(%v, WatermarkConflict) = false, want a sealed conflict", err)
	}
	got, reasonErr := conflict.Reason()
	if reasonErr != nil {
		t.Fatalf("WatermarkConflict.Reason() error = %v, want nil", reasonErr)
	}
	return got
}

func TestEvidenceBodyHostileBoundaryMatrix(t *testing.T) {
	t.Parallel()

	fixture := newReceiptFixture(t, 20)
	empty := fixture.body
	empty.Extent = mustByteLength(t, 0)
	empty.SHA256 = core.NewSHA256Digest(sha256.Sum256(nil))
	empty.CRC32C = core.NewCRC32C(crc32.Checksum(nil, crc32.MakeTable(crc32.Castagnoli)))
	maximum := fixture.body
	maximum.Extent = mustByteLength(t, math.MaxInt64)
	other := newReceiptFixture(t, 21)
	body := func(mutate func(*EvidenceBody)) EvidenceBody {
		got := fixture.body
		mutate(&got)
		return got
	}
	fromEmpty := func(mutate func(*EvidenceBody)) EvidenceBody {
		got := empty
		mutate(&got)
		return got
	}
	// twoByteExtent pairs a nonempty extent with the integrity of a different
	// nonempty stream. The body is internally consistent as far as Receipt can
	// tell, so it must be admitted: Receipt authenticates statements, it does
	// not recompute the object.
	twoByteExtent := body(func(v *EvidenceBody) { v.Extent = mustByteLength(t, 2) })
	cases := []struct {
		wantErr error
		name    string
		body    EvidenceBody
	}{
		{name: "ordinary nonempty evidence is admitted", body: fixture.body},
		{name: "canonical empty evidence is admitted", body: empty},
		{name: "maximum extent is admitted", body: maximum},
		{name: "one above the empty extent is admitted", body: twoByteExtent},
		{name: "foreign submission is admitted", body: body(func(v *EvidenceBody) { v.Submission = other.submission })},
		{name: "foreign object is admitted", body: body(func(v *EvidenceBody) { v.Object = other.object })},
		{name: "foreign SHA-256 is admitted", body: body(func(v *EvidenceBody) { v.SHA256 = other.body.SHA256 })},
		{name: "foreign CRC32C is admitted", body: body(func(v *EvidenceBody) { v.CRC32C = other.body.CRC32C })},
		{name: "zero CRC32C over a nonempty extent is admitted", body: body(func(v *EvidenceBody) { v.CRC32C = core.NewCRC32C(0) })},
		{name: "maximum CRC32C is admitted", body: body(func(v *EvidenceBody) { v.CRC32C = core.NewCRC32C(math.MaxUint32) })},
		{name: "one below the maximum extent is admitted", body: body(func(v *EvidenceBody) { v.Extent = mustByteLength(t, math.MaxInt64-1) })},

		{name: "unset submission is refused", body: body(func(v *EvidenceBody) { v.Submission = SubmissionIdentity{} }), wantErr: core.ErrReceiptContract},
		{name: "unset object is refused", body: body(func(v *EvidenceBody) { v.Object = ObjectIdentity{} }), wantErr: core.ErrReceiptContract},
		{name: "unset SHA-256 is refused", body: body(func(v *EvidenceBody) { v.SHA256 = core.SHA256Digest{} }), wantErr: core.ErrReceiptContract},
		{name: "unset CRC32C is refused", body: body(func(v *EvidenceBody) { v.CRC32C = core.CRC32C{} }), wantErr: core.ErrReceiptContract},
		{name: "wholly unset body is refused", body: EvidenceBody{}, wantErr: core.ErrReceiptContract},
		{name: "empty extent with nonempty SHA-256 is refused", body: fromEmpty(func(v *EvidenceBody) { v.SHA256 = fixture.body.SHA256 }), wantErr: core.ErrReceiptContract},
		{name: "empty extent with nonempty CRC32C is refused", body: fromEmpty(func(v *EvidenceBody) { v.CRC32C = fixture.body.CRC32C }), wantErr: core.ErrReceiptContract},
		{name: "empty extent with both integrity fields replaced is refused", body: fromEmpty(func(v *EvidenceBody) { v.SHA256 = fixture.body.SHA256; v.CRC32C = fixture.body.CRC32C }), wantErr: core.ErrReceiptContract},
		// The inverse of the empty-stream rule. A statement that one byte was
		// accepted, carrying the integrity of zero bytes, is exactly as
		// contradictory as the empty case above and was silently admitted
		// before this table pressed the boundary from both sides.
		{name: "one byte claiming empty-stream integrity is refused", body: fromEmpty(func(v *EvidenceBody) { v.Extent = mustByteLength(t, 1) }), wantErr: core.ErrReceiptContract},
		{name: "maximum extent claiming empty-stream integrity is refused", body: fromEmpty(func(v *EvidenceBody) { v.Extent = mustByteLength(t, math.MaxInt64) }), wantErr: core.ErrReceiptContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotErr := tc.body.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("EvidenceBody.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func TestEvidenceVerificationMutationMatrix(t *testing.T) {
	t.Parallel()

	fixture := newReceiptFixture(t, 30)
	document := issueFixture(t, fixture)
	other := newReceiptFixture(t, 40)
	cases := []struct {
		want            error
		mutate          func(*EvidenceDocument)
		name            string
		trusted         attest.TrustedKeys
		wantExpectation EvidenceExpectation
		wantField       ScopeField
	}{
		{name: "authentic exact document verifies", trusted: fixture.trusted, wantExpectation: fixture.expectation},
		{name: "untrusted signer is verification failure", trusted: other.trusted, wantExpectation: fixture.expectation, want: core.ErrReceiptVerification},
		{name: "receipt identity mutation is verification failure", trusted: fixture.trusted, wantExpectation: fixture.expectation, want: core.ErrReceiptVerification, mutate: func(v *EvidenceDocument) { v.Payload.Header.Identity = other.receipt }},
		{name: "occurrence mutation is verification failure", trusted: fixture.trusted, wantExpectation: fixture.expectation, want: core.ErrReceiptVerification, mutate: func(v *EvidenceDocument) { v.Payload.Header.OccurredAt = other.occurredAt }},
		{name: "body extent mutation is verification failure", trusted: fixture.trusted, wantExpectation: fixture.expectation, want: core.ErrReceiptVerification, mutate: func(v *EvidenceDocument) { v.Payload.Body.Extent = mustByteLength(t, 2) }},
		{name: "signature mutation is verification failure", trusted: fixture.trusted, wantExpectation: fixture.expectation, want: core.ErrReceiptVerification, mutate: func(v *EvidenceDocument) { v.Attestation.Signature = attest.Signature{} }},
		{name: "account mismatch is typed scope failure", trusted: fixture.trusted, wantExpectation: func() EvidenceExpectation { v := fixture.expectation; v.Account = other.account; return v }(), want: core.ErrReceiptScope, wantField: ScopeFieldAccount},
		{name: "offering mismatch is typed scope failure", trusted: fixture.trusted, wantExpectation: func() EvidenceExpectation { v := fixture.expectation; v.Offering = other.offering; return v }(), want: core.ErrReceiptScope, wantField: ScopeFieldOffering},
		{name: "submission mismatch is typed scope failure", trusted: fixture.trusted, wantExpectation: func() EvidenceExpectation { v := fixture.expectation; v.Body.Submission = other.submission; return v }(), want: core.ErrReceiptScope, wantField: ScopeFieldSubmission},
		{name: "object mismatch is typed scope failure", trusted: fixture.trusted, wantExpectation: func() EvidenceExpectation { v := fixture.expectation; v.Body.Object = other.object; return v }(), want: core.ErrReceiptScope, wantField: ScopeFieldObject},
		{name: "extent mismatch is typed scope failure", trusted: fixture.trusted, wantExpectation: func() EvidenceExpectation { v := fixture.expectation; v.Body.Extent = mustByteLength(t, 2); return v }(), want: core.ErrReceiptScope, wantField: ScopeFieldExtent},
		{name: "SHA-256 mismatch is typed scope failure", trusted: fixture.trusted, wantExpectation: func() EvidenceExpectation { v := fixture.expectation; v.Body.SHA256 = other.body.SHA256; return v }(), want: core.ErrReceiptScope, wantField: ScopeFieldSHA256},
		{name: "CRC32C mismatch is typed scope failure", trusted: fixture.trusted, wantExpectation: func() EvidenceExpectation { v := fixture.expectation; v.Body.CRC32C = core.NewCRC32C(0); return v }(), want: core.ErrReceiptScope, wantField: ScopeFieldCRC32C},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			candidate := document
			if tc.mutate != nil {
				tc.mutate(&candidate)
			}
			got, gotErr := VerifyEvidence(VerifyEvidenceRequest{
				Document: candidate, TrustedKeys: tc.trusted, Expected: tc.wantExpectation,
			})
			if !errors.Is(gotErr, tc.want) {
				t.Fatalf("VerifyEvidence() error = %v, want %v", gotErr, tc.want)
			}
			if tc.want == nil {
				if got.Validate() != nil {
					t.Fatalf("VerifyEvidence() result Validate() error = %v, want nil", got.Validate())
				}
				return
			}
			if got != (VerifiedEvidence{}) {
				t.Fatalf("VerifyEvidence(rejected) result = %v, want zero", got)
			}
			if tc.wantField != ScopeFieldUnknown {
				if gotField := requireScopeField(t, gotErr); gotField != tc.wantField {
					t.Fatalf("VerifyEvidence() mismatch field = %v, want %v", gotField, tc.wantField)
				}
			}
		})
	}
}

func TestWatermarkAdvanceLayerTriad(t *testing.T) {
	t.Parallel()

	fixture := newReceiptFixture(t, 50)
	scope := Scope{Account: fixture.account, Offering: fixture.offering}
	current := watermarkFixture(t, scope, 2, "current")
	next := watermarkFixture(t, scope, 3, "next")

	replay, err := AdvanceWatermark(AdvanceWatermarkRequest{Current: current, Candidate: current})
	replayState, replayStateErr := replay.State()
	replayWatermark, replayWatermarkErr := replay.Watermark()
	if err != nil || replayStateErr != nil || replayWatermarkErr != nil ||
		replayState != AdvanceReplay || replayWatermark != current {
		t.Fatalf("AdvanceWatermark(replay) = (%v, %v), want current replay and nil", replay, err)
	}
	accepted, err := AdvanceWatermark(AdvanceWatermarkRequest{Current: current, Candidate: next})
	acceptedState, acceptedStateErr := accepted.State()
	acceptedWatermark, acceptedWatermarkErr := accepted.Watermark()
	if err != nil || acceptedStateErr != nil || acceptedWatermarkErr != nil ||
		acceptedState != AdvanceAccepted || acceptedWatermark != next {
		t.Fatalf("AdvanceWatermark(higher) = (%v, %v), want candidate accepted and nil", accepted, err)
	}
	rejected, err := AdvanceWatermark(AdvanceWatermarkRequest{Current: next, Candidate: current})
	if rejected != (AdvanceResult{}) || !errors.Is(err, core.ErrReceiptRollback) {
		t.Fatalf("AdvanceWatermark(lower) = (%v, %v), want zero and %v", rejected, err, core.ErrReceiptRollback)
	}
}

func TestWatermarkAdvanceHostileMatrix(t *testing.T) {
	t.Parallel()

	fixture := newReceiptFixture(t, 60)
	other := newReceiptFixture(t, 70)
	scope := Scope{Account: fixture.account, Offering: fixture.offering}
	first := watermarkFixture(t, scope, 1, "first")
	current := watermarkFixture(t, scope, 2, "current")
	next := watermarkFixture(t, scope, 3, "next")
	far := watermarkFixture(t, scope, math.MaxUint64, "far")
	from := func(base Watermark, mutate func(*Watermark)) Watermark {
		got := base
		mutate(&got)
		return got
	}
	cases := []struct {
		want       error
		name       string
		current    Watermark
		candidate  Watermark
		wantState  AdvanceState
		wantReason ConflictReason
	}{
		{name: "identical generation and closures replay", current: current, candidate: current, wantState: AdvanceReplay},
		{name: "lowest generation replays itself", current: first, candidate: first, wantState: AdvanceReplay},
		{name: "maximum generation replays itself", current: far, candidate: far, wantState: AdvanceReplay},
		{name: "one step forward with both new closures accepts", current: current, candidate: next, wantState: AdvanceAccepted},
		{name: "first to second generation accepts", current: first, candidate: current, wantState: AdvanceAccepted},
		{name: "first to maximum generation accepts", current: first, candidate: far, wantState: AdvanceAccepted},
		{name: "one below maximum to maximum accepts", current: from(current, func(v *Watermark) { v.Generation = mustGeneration(t, math.MaxUint64-1) }), candidate: far, wantState: AdvanceAccepted},
		{name: "accepted advance selects the candidate not the current", current: first, candidate: next, wantState: AdvanceAccepted},
		{name: "advance across a wide generation gap accepts", current: first, candidate: from(next, func(v *Watermark) { v.Generation = mustGeneration(t, 1<<40) }), wantState: AdvanceAccepted},
		{name: "advance keeping neither closure accepts", current: current, candidate: from(far, func(v *Watermark) { v.CursorDigest = next.CursorDigest }), wantState: AdvanceAccepted},

		{name: "one generation below rolls back", current: next, candidate: current, want: core.ErrReceiptRollback},
		{name: "maximum to first rolls back", current: far, candidate: first, want: core.ErrReceiptRollback},
		{name: "one below the maximum rolls back", current: far, candidate: from(next, func(v *Watermark) { v.Generation = mustGeneration(t, math.MaxUint64-1) }), want: core.ErrReceiptRollback},
		{name: "equal generation with cursor divergence conflicts", current: current, candidate: from(current, func(v *Watermark) { v.CursorDigest = next.CursorDigest }), want: core.ErrReceiptConflict, wantReason: ConflictReasonReplayDivergence},
		{name: "equal generation with chain divergence conflicts", current: current, candidate: from(current, func(v *Watermark) { v.ChainHash = next.ChainHash }), want: core.ErrReceiptConflict, wantReason: ConflictReasonReplayDivergence},
		{name: "higher generation with stale cursor conflicts", current: current, candidate: from(next, func(v *Watermark) { v.CursorDigest = current.CursorDigest }), want: core.ErrReceiptConflict, wantReason: ConflictReasonCursorUnchanged},
		{name: "higher generation with stale chain conflicts", current: current, candidate: from(next, func(v *Watermark) { v.ChainHash = current.ChainHash }), want: core.ErrReceiptConflict, wantReason: ConflictReasonChainUnchanged},
		{name: "higher generation reusing both closures reports the cursor first", current: current, candidate: from(next, func(v *Watermark) { v.CursorDigest = current.CursorDigest; v.ChainHash = current.ChainHash }), want: core.ErrReceiptConflict, wantReason: ConflictReasonCursorUnchanged},
		{name: "foreign account conflicts before generation", current: next, candidate: watermarkFixture(t, Scope{Account: other.account, Offering: fixture.offering}, 1, "foreign-account"), want: core.ErrReceiptConflict, wantReason: ConflictReasonScope},
		{name: "foreign offering conflicts before generation", current: next, candidate: watermarkFixture(t, Scope{Account: fixture.account, Offering: other.offering}, 1, "foreign-offering"), want: core.ErrReceiptConflict, wantReason: ConflictReasonScope},
		{name: "foreign scope outranks an otherwise valid advance", current: current, candidate: watermarkFixture(t, Scope{Account: other.account, Offering: other.offering}, 3, "foreign-both"), want: core.ErrReceiptConflict, wantReason: ConflictReasonScope},
		{name: "foreign scope outranks a rollback", current: far, candidate: watermarkFixture(t, Scope{Account: other.account, Offering: fixture.offering}, 1, "foreign-rollback"), want: core.ErrReceiptConflict, wantReason: ConflictReasonScope},
		{name: "zero current is contract failure", candidate: next, want: core.ErrReceiptContract},
		{name: "zero candidate is contract failure", current: current, want: core.ErrReceiptContract},
		{name: "both zero is contract failure", want: core.ErrReceiptContract},
	}
	var coveredReasons [conflictReasonLimit]bool
	for _, tc := range cases {
		if tc.wantReason.IsValid() {
			coveredReasons[tc.wantReason] = true
		}
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := AdvanceWatermark(AdvanceWatermarkRequest{
				Current: tc.current, Candidate: tc.candidate,
			})
			if !errors.Is(gotErr, tc.want) {
				t.Fatalf("AdvanceWatermark() error = %v, want %v", gotErr, tc.want)
			}
			if tc.want != nil {
				if got != (AdvanceResult{}) {
					t.Fatalf("AdvanceWatermark(rejected) result = %v, want zero", got)
				}
				if tc.wantReason != ConflictReasonUnknown {
					if gotReason := requireConflictReason(t, gotErr); gotReason != tc.wantReason {
						t.Fatalf("conflict reason = %v, want %v", gotReason, tc.wantReason)
					}
				}
				return
			}
			gotState, stateErr := got.State()
			gotWatermark, watermarkErr := got.Watermark()
			if stateErr != nil || watermarkErr != nil {
				t.Fatalf("AdvanceResult accessors = (%v, %v), want nil", stateErr, watermarkErr)
			}
			if gotState != tc.wantState {
				t.Fatalf("AdvanceWatermark() state = %v, want %v", gotState, tc.wantState)
			}
			wantWatermark := tc.candidate
			if tc.wantState == AdvanceReplay {
				wantWatermark = tc.current
			}
			if gotWatermark != wantWatermark {
				t.Fatalf("AdvanceWatermark() watermark = %v, want %v", gotWatermark, wantWatermark)
			}
		})
	}
	for reason := ConflictReasonUnknown + 1; reason < conflictReasonLimit; reason++ {
		if !coveredReasons[reason] {
			t.Errorf("ConflictReason %v has no behaviorally reachable advance case", reason)
		}
	}
}

func mustGeneration(t testing.TB, value uint64) Generation {
	t.Helper()

	got, err := NewGeneration(value)
	if err != nil {
		t.Fatalf("NewGeneration(%d) error = %v, want nil", value, err)
	}
	return got
}

func watermarkFixture(t testing.TB, scope Scope, generation uint64, marker string) Watermark {
	t.Helper()
	gotGeneration, err := NewGeneration(generation)
	if err != nil {
		t.Fatalf("NewGeneration() error = %v, want nil", err)
	}
	cursor, err := NewCursorDigest(core.NewSHA256Digest(sha256.Sum256([]byte("cursor-" + marker))))
	if err != nil {
		t.Fatalf("NewCursorDigest() error = %v, want nil", err)
	}
	chain, err := NewChainHash(core.NewSHA256Digest(sha256.Sum256([]byte("chain-" + marker))))
	if err != nil {
		t.Fatalf("NewChainHash() error = %v, want nil", err)
	}
	got, err := NewWatermark(WatermarkRequest{
		Scope: scope, Generation: gotGeneration, CursorDigest: cursor, ChainHash: chain,
	})
	if err != nil {
		t.Fatalf("NewWatermark() error = %v, want nil", err)
	}
	return got
}

func TestReceiptClosedEnumsExhaustBackingDomain(t *testing.T) {
	t.Parallel()

	for raw := 0; raw <= math.MaxUint8; raw++ {
		revision := Revision(raw)
		wantRevision := revision == RevisionV1
		if revision.IsValid() != wantRevision || (revision.String() != "") != wantRevision {
			t.Fatalf("Revision(%d) validity/text = (%t, %q), want admitted=%t", raw, revision.IsValid(), revision.String(), wantRevision)
		}
		domain := Domain(raw)
		wantDomain := domain == DomainEvidenceV1
		if domain.IsValid() != wantDomain || (domain.String() != "") != wantDomain {
			t.Fatalf("Domain(%d) validity/text = (%t, %q), want admitted=%t", raw, domain.IsValid(), domain.String(), wantDomain)
		}
		state := AdvanceState(raw)
		wantState := state == AdvanceAccepted || state == AdvanceReplay
		if state.IsValid() != wantState || (state.String() != "") != wantState {
			t.Fatalf("AdvanceState(%d) validity/text = (%t, %q), want admitted=%t", raw, state.IsValid(), state.String(), wantState)
		}
		field := ScopeField(raw)
		wantField := field > ScopeFieldUnknown && field < scopeFieldLimit
		if field.IsValid() != wantField || (field.String() != "") != wantField {
			t.Fatalf("ScopeField(%d) validity/text = (%t, %q), want admitted=%t", raw, field.IsValid(), field.String(), wantField)
		}
		reason := ConflictReason(raw)
		wantReason := reason > ConflictReasonUnknown && reason < conflictReasonLimit
		if reason.IsValid() != wantReason || (reason.String() != "") != wantReason {
			t.Fatalf("ConflictReason(%d) validity/text = (%t, %q), want admitted=%t", raw, reason.IsValid(), reason.String(), wantReason)
		}
	}
}

// TestReceiptEnumLabelsAreExactAndDistinct pins label content, not just label
// presence. Exhausting the backing domain proves an unknown value has no text;
// it cannot see two admitted values whose texts were swapped, and a swapped
// label is a silently wrong diagnostic on every rejection path.
func TestReceiptEnumLabelsAreExactAndDistinct(t *testing.T) {
	t.Parallel()

	cases := []struct {
		value interface{ String() string }
		name  string
		want  string
	}{
		{name: "revision v1", value: RevisionV1, want: "v1"},
		{name: "domain evidence v1", value: DomainEvidenceV1, want: evidenceDomainToken},
		{name: "scope field account", value: ScopeFieldAccount, want: "account"},
		{name: "scope field offering", value: ScopeFieldOffering, want: "offering"},
		{name: "scope field submission", value: ScopeFieldSubmission, want: "submission"},
		{name: "scope field object", value: ScopeFieldObject, want: "object"},
		{name: "scope field extent", value: ScopeFieldExtent, want: "extent"},
		{name: "scope field sha256", value: ScopeFieldSHA256, want: "sha256"},
		{name: "scope field crc32c", value: ScopeFieldCRC32C, want: "crc32c"},
		{name: "advance accepted", value: AdvanceAccepted, want: "accepted"},
		{name: "advance replay", value: AdvanceReplay, want: "replay"},
		{name: "conflict scope", value: ConflictReasonScope, want: "scope"},
		{name: "conflict replay divergence", value: ConflictReasonReplayDivergence, want: "replay-divergence"},
		{name: "conflict cursor unchanged", value: ConflictReasonCursorUnchanged, want: "cursor-unchanged"},
		{name: "conflict chain unchanged", value: ConflictReasonChainUnchanged, want: "chain-unchanged"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.value.String(); got != tc.want {
				t.Fatalf("String() = %q, want %q", got, tc.want)
			}
		})
	}

	scopeFields := scopeFieldDiagnostics()
	advanceStates := advanceStateDiagnostics()
	conflictReasons := conflictReasonDiagnostics()
	for _, labels := range [][]string{
		scopeFields[1:], advanceStates[1:], conflictReasons[1:],
	} {
		seen := make(map[string]int, len(labels))
		for index, label := range labels {
			if label == unknownText {
				t.Errorf("admitted label at index %d is the unknown sentinel", index+1)
			}
			if first, duplicated := seen[label]; duplicated {
				t.Errorf("label %q is shared by indexes %d and %d", label, first, index+1)
			}
			seen[label] = index + 1
		}
	}
}

func TestEvidenceDocumentJSONLayerTriad(t *testing.T) {
	t.Parallel()

	fixture := newReceiptFixture(t, 80)
	document := issueFixture(t, fixture)
	canonical, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("json.Marshal(EvidenceDocument) error = %v, want nil", err)
	}
	var decoded EvidenceDocument
	if err := json.Unmarshal(canonical, &decoded); err != nil || decoded != document {
		t.Fatalf("json.Unmarshal(canonical) = (%v, %v), want exact document and nil", decoded, err)
	}
	padded := append(append([]byte{' ', '\n'}, canonical...), '\n')
	if err := json.Unmarshal(padded, &decoded); err != nil || decoded != document {
		t.Fatalf("json.Unmarshal(padded) = (%v, %v), want normalized document and nil", decoded, err)
	}
	for _, data := range [][]byte{
		nil,
		[]byte("null"),
		canonical[:len(canonical)-1],
		append(append([]byte{}, canonical...), []byte(`{}`)...),
		[]byte{'"', 0xff, '"'},
		bytes.Repeat([]byte{' '}, EvidenceDocumentJSONMaximumBytes+1),
	} {
		receiver := document
		gotErr := receiver.UnmarshalJSON(data)
		if !errors.Is(gotErr, core.ErrJSONContract) || receiver != document {
			t.Fatalf("EvidenceDocument.UnmarshalJSON(%q) = (%v, %v), want preserved receiver and %v", data, receiver, gotErr, core.ErrJSONContract)
		}
	}
}
