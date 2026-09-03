package sourceclaim_test

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/sourceclaim"
)

func TestClaimStreamRejectsDuplicateAtomicClaimsInsteadOfCollapsingPackageMeaning(t *testing.T) {
	t.Parallel()

	path, pathErr := core.ParseSourcePath("exchange")
	if pathErr != nil {
		t.Fatalf("core.ParseSourcePath(exchange) error = %v, want nil", pathErr)
	}
	subject, subjectErr := core.NewSourceSubject(core.SourceSubjectPackage, path)
	if subjectErr != nil {
		t.Fatalf("core.NewSourceSubject(package) error = %v, want nil", subjectErr)
	}
	claim := fixtureClaim(t, subject, "exchange-bounds-http")

	gotSummary, gotErr := sourceclaim.Consume(func(emit sourceclaim.Emit) error {
		if err := emit(claim); err != nil {
			return err
		}
		return emit(claim)
	}, func(sourceclaim.Claim) error { return nil })
	if !errors.Is(gotErr, core.ErrSourceClaimConflict) {
		t.Fatalf("Consume(duplicate claim identity) = (%+v, %v), want (zero, %v)", gotSummary, gotErr, core.ErrSourceClaimConflict)
	}
}

func TestClaimStreamReportsCardinalityWithoutAClaimQuota(t *testing.T) {
	t.Parallel()

	path, pathErr := core.ParseSourcePath("exchange")
	if pathErr != nil {
		t.Fatalf("core.ParseSourcePath(exchange) error = %v, want nil", pathErr)
	}
	subject, subjectErr := core.NewSourceSubject(core.SourceSubjectPackage, path)
	if subjectErr != nil {
		t.Fatalf("core.NewSourceSubject(package) error = %v, want nil", subjectErr)
	}
	const claimCount = 513
	wantStream := core.NewDigestWriter()

	gotSummary, gotErr := sourceclaim.Consume(func(emit sourceclaim.Emit) error {
		for index := range claimCount {
			claim := fixtureClaim(t, subject, fmt.Sprintf("claim-%04d", index))
			if err := emit(claim); err != nil {
				return err
			}
		}
		return nil
	}, func(claim sourceclaim.Claim) error {
		return writeClaimProjection(wantStream, claim)
	})
	wantDigest, wantBytes, wantDigestErr := wantStream.Seal()
	if wantDigestErr != nil {
		t.Fatalf("claim stream digest error = %v, want nil", wantDigestErr)
	}
	wantSummary := sourceclaim.Summary{
		Digest: wantDigest, Bytes: wantBytes,
		Packages: 1, Subjects: 1, PackageClaims: claimCount, Claims: claimCount,
	}
	if gotErr != nil || gotSummary != wantSummary {
		t.Fatalf("Consume(%d package claims) = (%+v, %v), want (%+v, nil)", claimCount, gotSummary, gotErr, wantSummary)
	}
}

func writeClaimProjection(destination *core.DigestWriter, claim sourceclaim.Claim) error {
	encoded, err := claim.MarshalJSON()
	if err != nil {
		return err
	}
	if _, err := destination.Write(encoded); err != nil {
		return err
	}
	_, err = destination.Write([]byte{'\n'})
	return err
}

func TestClaimStreamCannotEraseAnIgnoredDestinationFailure(t *testing.T) {
	t.Parallel()

	subject, subjectErr := core.NewSourceSubject(core.SourceSubjectPackage, mustSourcePath(t, "exchange"))
	if subjectErr != nil {
		t.Fatalf("core.NewSourceSubject(package) error = %v, want nil", subjectErr)
	}
	claim := fixtureClaim(t, subject, "destination-failure")
	wantErr := errors.New("destination unavailable")
	gotSummary, gotErr := sourceclaim.Consume(func(emit sourceclaim.Emit) error {
		_ = emit(claim)
		return nil
	}, func(sourceclaim.Claim) error {
		return wantErr
	})
	if !errors.Is(gotErr, wantErr) || gotSummary != (sourceclaim.Summary{}) {
		t.Fatalf("sourceclaim.Consume(ignored destination failure) = (%+v, %v), want (zero, %v)", gotSummary, gotErr, wantErr)
	}
}

func TestClaimAcceptsLargeProblemNarrativeWithoutATextQuota(t *testing.T) {
	t.Parallel()

	path, pathErr := core.ParseSourcePath("exchange")
	if pathErr != nil {
		t.Fatalf("core.ParseSourcePath(exchange) error = %v, want nil", pathErr)
	}
	subject, subjectErr := core.NewSourceSubject(core.SourceSubjectPackage, path)
	if subjectErr != nil {
		t.Fatalf("core.NewSourceSubject(package) error = %v, want nil", subjectErr)
	}
	claim := fixtureClaim(t, subject, "large-problem")
	claim.Problem.Summary = mustClaimText(t, strings.Repeat("large but meaningful problem detail ", 256)+"end")

	gotErr := claim.Validate()
	if gotErr != nil {
		t.Fatalf("Claim.Validate(large problem narrative) error = %v, want nil", gotErr)
	}
}

func TestClaimRejectsNonCanonicalBoundariesAndRequirements(t *testing.T) {
	t.Parallel()

	path, pathErr := core.ParseSourcePath("exchange")
	if pathErr != nil {
		t.Fatalf("core.ParseSourcePath(exchange) error = %v, want nil", pathErr)
	}
	subject, subjectErr := core.NewSourceSubject(core.SourceSubjectPackage, path)
	if subjectErr != nil {
		t.Fatalf("core.NewSourceSubject(package) error = %v, want nil", subjectErr)
	}

	cases := []struct {
		mutate func(testing.TB, *sourceclaim.Claim)
		name   string
	}{
		{
			name: "owned boundaries run backward",
			mutate: func(t testing.TB, claim *sourceclaim.Claim) {
				claim.Owns = []sourceclaim.Boundary{
					{ID: mustClaimID(t, "transport"), Detail: mustClaimText(t, "Typed transport mechanics.")},
					{ID: mustClaimID(t, "bounds"), Detail: mustClaimText(t, "Byte and field ceilings.")},
				}
			},
		},
		{
			name: "one boundary is both owned and excluded",
			mutate: func(t testing.TB, claim *sourceclaim.Claim) {
				claim.DoesNotOwn = []sourceclaim.Boundary{
					{ID: claim.Owns[0].ID, Detail: mustClaimText(t, "Conflicting ownership statement.")},
				}
			},
		},
		{
			name: "proof requirements run backward",
			mutate: func(t testing.TB, claim *sourceclaim.Claim) {
				claim.Requirements = []sourceclaim.Requirement{
					{ID: mustClaimID(t, "review-z"), Statement: mustClaimText(t, "Review the ending boundary."), Mode: sourceclaim.RequirementHumanReview},
					{ID: mustClaimID(t, "review-a"), Statement: mustClaimText(t, "Review the starting boundary."), Mode: sourceclaim.RequirementHumanReview},
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			claim := fixtureClaim(t, subject, "canonical-contract")
			tc.mutate(t, &claim)
			gotErr := claim.Validate()
			if !errors.Is(gotErr, core.ErrSourceClaimConflict) {
				t.Fatalf("Claim.Validate(non-canonical contract) error = %v, want %v", gotErr, core.ErrSourceClaimConflict)
			}
		})
	}
}

func fixtureClaim(t *testing.T, subject core.SourceSubject, identity string) sourceclaim.Claim {
	t.Helper()

	return sourceclaim.Claim{
		ID:         mustClaimID(t, identity),
		Author:     mustClaimAuthority(t, 1),
		Subject:    subject,
		Title:      mustClaimText(t, "Bounded HTTP exchange"),
		Problem:    sourceclaim.Narrative{Summary: mustClaimText(t, "Callers otherwise repeat transport limits and lose one mechanical truth.")},
		Solution:   sourceclaim.Narrative{Summary: mustClaimText(t, "Own bounded HTTP request and response mechanics behind typed structures.")},
		Benefit:    sourceclaim.Narrative{Summary: mustClaimText(t, "Every consumer crosses the same validated and observable transport wall.")},
		Removal:    sourceclaim.Narrative{Summary: mustClaimText(t, "Remove when Go itself owns the complete bounded and receipted exchange contract.")},
		Owns:       []sourceclaim.Boundary{{ID: mustClaimID(t, "http-mechanics"), Detail: mustClaimText(t, "HTTP shape, limits, transport execution, and exact observations.")}},
		DoesNotOwn: []sourceclaim.Boundary{{ID: mustClaimID(t, "product-policy"), Detail: mustClaimText(t, "Whether a product operation is permitted or meaningful.")}},
		Requirements: []sourceclaim.Requirement{{
			ID:        mustClaimID(t, "human-value-review"),
			Statement: mustClaimText(t, "A human confirms the package remains a useful shared mechanical boundary."),
			Mode:      sourceclaim.RequirementHumanReview,
		}},
	}
}

func mustClaimAuthority(t testing.TB, value byte) core.Ed25519PublicKey {
	t.Helper()

	got, err := core.NewEd25519PublicKey(ed25519.PublicKey(bytes.Repeat([]byte{value}, ed25519.PublicKeySize)))
	if err != nil {
		t.Fatalf("core.NewEd25519PublicKey() error = %v, want nil", err)
	}
	return got
}

func mustClaimID(t testing.TB, value string) sourceclaim.ID {
	t.Helper()

	got, err := sourceclaim.NewID(value)
	if err != nil {
		t.Fatalf("sourceclaim.NewID(%q) error = %v, want nil", value, err)
	}
	return got
}

func mustClaimText(t testing.TB, value string) sourceclaim.Text {
	t.Helper()

	got, err := sourceclaim.NewText(value)
	if err != nil {
		t.Fatalf("sourceclaim.NewText(%q) error = %v, want nil", value, err)
	}
	return got
}

func mustSourcePath(t testing.TB, value string) core.SourcePath {
	t.Helper()

	got, err := core.ParseSourcePath(value)
	if err != nil {
		t.Fatalf("core.ParseSourcePath(%q) error = %v, want nil", value, err)
	}
	return got
}
