package sourceproof_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/sourceclaim"
	"github.com/deliri/primitive/v2026/sourceproof"
)

func BenchmarkStateUnmarshalJSON(b *testing.B) {
	data := []byte(`"proven"`)
	var wantErr error
	b.ReportAllocs()
	var last sourceproof.State
	for b.Loop() {
		var got sourceproof.State
		err := got.UnmarshalJSON(data)
		if !errors.Is(err, wantErr) {
			b.Fatalf("sourceproof.State.UnmarshalJSON() error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if last != sourceproof.StateProven {
		b.Fatalf("sourceproof.State.UnmarshalJSON() = %v, want %v", last, sourceproof.StateProven)
	}
}

// orderedProofResolver answers claims in authored order from a fixed slice.
type orderedProofResolver struct {
	results []sourceproof.Result
	next    int
}

func (r *orderedProofResolver) ResolveResult(_ context.Context, claim sourceclaim.Claim) (sourceproof.Result, error) {
	result := r.results[r.next%len(r.results)]
	r.next++
	if result.Claim != claim.ID {
		return sourceproof.Result{}, errors.New("benchmark resolver order differs from authored stream")
	}
	return result, nil
}

var benchmarkVerifyClaimsSink sourceproof.Summary

// VerifyClaims is the proof join: per claim it resolves, validates against
// the authored claim, folds the summary and digests the canonical record.
func BenchmarkVerifyClaims(b *testing.B) {
	const claimCount = 64
	subject := proofSubject(b, core.SourceSubjectPackage, "exchange")
	snapshot := proofSnapshot(b, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	claims := make([]sourceclaim.Claim, 0, claimCount)
	resolver := &orderedProofResolver{results: make([]sourceproof.Result, 0, claimCount)}
	for index := range claimCount {
		claim := proofClaimWithID(b, subject, fmt.Sprintf("boundary-%03d", index))
		claims = append(claims, claim)
		resolver.results = append(resolver.results, proofResult(b, claim, snapshot, sourceproof.StateHumanReviewRequired, nil))
	}
	stream := func(emit sourceclaim.Emit) error {
		for _, claim := range claims {
			if err := emit(claim); err != nil {
				return err
			}
		}
		return nil
	}
	forwarded := 0
	destination := func(sourceproof.Result) error { forwarded++; return nil }
	b.ReportAllocs()
	for b.Loop() {
		summary, err := sourceproof.VerifyClaims(b.Context(), stream, resolver, destination)
		if err != nil {
			b.Fatalf("VerifyClaims() error = %v, want nil", err)
		}
		benchmarkVerifyClaimsSink = summary
	}
	if benchmarkVerifyClaimsSink.Claims != claimCount || forwarded%claimCount != 0 || forwarded == 0 {
		b.Fatalf("VerifyClaims() summary = %+v forwarded=%d, want %d claims per pass", benchmarkVerifyClaimsSink, forwarded, claimCount)
	}
}
