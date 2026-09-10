package receipt

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzReceiptSigningDomain(f *testing.F) {
	seed, err := DomainEvidenceV1.MarshalText()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add([]byte{})
	f.Add(bytes.ToUpper(seed))
	f.Fuzz(func(t *testing.T, data []byte) {
		want := bytes.Equal(data, seed)
		got, err := DomainUnknown.ParseCanonicalText(data)
		if (err == nil) != want || (want && got != DomainEvidenceV1) || (!want && (got != DomainUnknown || !errors.Is(err, core.ErrReceiptContract))) {
			t.Fatalf("domain=%v/%v, want admitted=%t and exact typed result", got, err, want)
		}
	})
}

type watermarkMutation uint8

const (
	watermarkShareCursor watermarkMutation = 1 << iota
	watermarkShareChain
	watermarkForeignPrincipal
	watermarkForeignOffering
)

func FuzzWatermarkAdvance(f *testing.F) {
	fixture := newReceiptFixture(f, 201)
	other := newReceiptFixture(f, 202)
	scope := Scope{Principal: fixture.principal, Offering: fixture.offering}
	current := watermarkFixture(f, scope, 1, "current")
	next := watermarkFixture(f, scope, 2, "candidate")
	currentGeneration, err := current.Generation.Uint64()
	if err != nil {
		f.Fatal(err)
	}
	nextGeneration, err := next.Generation.Uint64()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(currentGeneration, nextGeneration, uint8(0))
	f.Add(currentGeneration, currentGeneration, uint8(watermarkShareCursor|watermarkShareChain))
	f.Add(uint64(0), nextGeneration, uint8(watermarkForeignPrincipal))
	f.Fuzz(func(t *testing.T, left, right uint64, raw uint8) {
		a, b := current, next
		var err error
		a.Generation, err = NewGeneration(left)
		if (err == nil) != (left != 0) {
			t.Fatalf("current generation=%v, want admitted=%t", err, left != 0)
		}
		b.Generation, err = NewGeneration(right)
		if (err == nil) != (right != 0) {
			t.Fatalf("candidate generation=%v, want admitted=%t", err, right != 0)
		}
		mutation := watermarkMutation(raw)
		if mutation&watermarkShareCursor != 0 {
			b.CursorDigest = a.CursorDigest
		}
		if mutation&watermarkShareChain != 0 {
			b.ChainHash = a.ChainHash
		}
		if mutation&watermarkForeignPrincipal != 0 {
			b.Scope.Principal = other.principal
		}
		if mutation&watermarkForeignOffering != 0 {
			b.Scope.Offering = other.offering
		}
		var want error
		reason := ConflictReasonUnknown
		state := AdvanceUnknown
		switch {
		case left == 0 || right == 0:
			want = core.ErrReceiptContract
		case mutation&(watermarkForeignPrincipal|watermarkForeignOffering) != 0:
			want = core.ErrReceiptConflict
			reason = ConflictReasonScope
		case right < left:
			want = core.ErrReceiptRollback
		case right == left:
			if mutation&(watermarkShareCursor|watermarkShareChain) == watermarkShareCursor|watermarkShareChain {
				state = AdvanceReplay
			} else {
				want = core.ErrReceiptConflict
				reason = ConflictReasonReplayDivergence
			}
		case mutation&watermarkShareCursor != 0:
			want = core.ErrReceiptConflict
			reason = ConflictReasonCursorUnchanged
		case mutation&watermarkShareChain != 0:
			want = core.ErrReceiptConflict
			reason = ConflictReasonChainUnchanged
		default:
			state = AdvanceAccepted
		}
		got, gotErr := AdvanceWatermark(AdvanceWatermarkRequest{Current: a, Candidate: b})
		if !errors.Is(gotErr, want) || (want != nil && got != (AdvanceResult{})) {
			t.Fatalf("advance=%v/%v, want state=%v error=%v and zero on refusal", got, gotErr, state, want)
		}
		for _, identity := range []error{core.ErrReceiptConflict, core.ErrReceiptRollback, core.ErrReceiptScope, core.ErrReceiptVerification} {
			if errors.Is(gotErr, identity) != (want == identity) {
				t.Fatalf("advance=%v, want exclusive identity %v", gotErr, want)
			}
		}
		if reason != ConflictReasonUnknown {
			var conflict WatermarkConflict
			if !errors.As(gotErr, &conflict) {
				t.Fatalf("advance=%v, want sealed conflict", gotErr)
			}
			actual, err := conflict.Reason()
			if err != nil || actual != reason {
				t.Fatalf("reason=%v/%v, want %v", actual, err, reason)
			}
		}
		if want != nil {
			return
		}
		selected, selectionErr := got.Watermark()
		actual, stateErr := got.State()
		expected := b
		if state == AdvanceReplay {
			expected = a
		}
		if selectionErr != nil || stateErr != nil || actual != state || selected != expected {
			t.Fatalf("selected=%v/%v state=%v/%v, want exact %v with %v", selected, selectionErr, actual, stateErr, expected, state)
		}
	})
}
