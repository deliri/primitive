package sourceproof_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/sourceclaim"
	"github.com/deliri/primitive/v2026/sourceproof"
)

func TestResultRejectsClaimAuthorIssuingItsOwnProof(t *testing.T) {
	t.Parallel()

	path, pathErr := core.ParseSourcePath("exchange")
	if pathErr != nil {
		t.Fatalf("core.ParseSourcePath(exchange) error = %v, want nil", pathErr)
	}
	subject, subjectErr := core.NewSourceSubject(core.SourceSubjectPackage, path)
	if subjectErr != nil {
		t.Fatalf("core.NewSourceSubject(package) error = %v, want nil", subjectErr)
	}
	claim := proofClaim(t, subject)
	snapshot := proofSnapshot(t, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if snapshot.Validate() != nil {
		t.Fatalf("core.SourceSnapshot.Validate() error = %v, want nil", snapshot.Validate())
	}
	claimDigest, claimDigestErr := claim.Digest()
	if claimDigestErr != nil {
		t.Fatalf("Claim.Digest() error = %v, want nil", claimDigestErr)
	}
	result := sourceproof.Result{
		Claim:       claim.ID,
		Subject:     claim.Subject,
		ClaimDigest: claimDigest,
		Snapshot:    snapshot,
		Verifier:    claim.Author,
		Requirements: []sourceproof.RequirementResult{{
			Requirement: claim.Requirements[0].ID,
			State:       sourceproof.StateHumanReviewRequired,
		}},
	}

	gotErr := result.ValidateAgainst(claim)
	if !errors.Is(gotErr, core.ErrSourceProofConflict) {
		t.Fatalf("Result.ValidateAgainst(self-issued proof) error = %v, want %v", gotErr, core.ErrSourceProofConflict)
	}
}

func TestResultPreservesEveryProofStateAndSnapshotMeaning(t *testing.T) {
	t.Parallel()

	path, pathErr := core.ParseSourcePath("exchange")
	if pathErr != nil {
		t.Fatalf("core.ParseSourcePath(exchange) error = %v, want nil", pathErr)
	}
	subject, subjectErr := core.NewSourceSubject(core.SourceSubjectPackage, path)
	if subjectErr != nil {
		t.Fatalf("core.NewSourceSubject(package) error = %v, want nil", subjectErr)
	}
	claim := proofClaim(t, subject)
	current := proofSnapshot(t, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	older := proofSnapshot(t, "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210")

	cases := []struct {
		state            sourceproof.State
		evidenceSnapshot core.SourceSnapshot
		withEvidence     bool
		wantErr          error
		name             string
	}{
		{name: "proven result cites current snapshot", state: sourceproof.StateProven, evidenceSnapshot: current, withEvidence: true},
		{name: "contradicted result cites current counterevidence", state: sourceproof.StateContradicted, evidenceSnapshot: current, withEvidence: true},
		{name: "unproven result preserves absence of proof", state: sourceproof.StateUnproven},
		{name: "stale result preserves older snapshot", state: sourceproof.StateStale, evidenceSnapshot: older, withEvidence: true},
		{name: "unavailable result preserves unavailable evidence", state: sourceproof.StateUnavailable},
		{name: "human review remains explicitly required", state: sourceproof.StateHumanReviewRequired},
		{name: "proven result cannot cite older snapshot", state: sourceproof.StateProven, evidenceSnapshot: older, withEvidence: true, wantErr: core.ErrSourceProofConflict},
		{name: "stale result cannot cite current snapshot", state: sourceproof.StateStale, evidenceSnapshot: current, withEvidence: true, wantErr: core.ErrSourceProofConflict},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var evidence []sourceproof.EvidenceReference
			if tc.withEvidence {
				evidence = []sourceproof.EvidenceReference{proofEvidence(t, claim.Subject, tc.evidenceSnapshot)}
			}
			result := proofResult(t, claim, current, tc.state, evidence)
			gotErr := result.ValidateAgainst(claim)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Result.ValidateAgainst(%s evidence) error = %v, want %v", tc.state, gotErr, tc.wantErr)
			}
		})
	}
}

func TestResultRejectsClaimDigestAndRequirementAccountingDrift(t *testing.T) {
	t.Parallel()

	claim := proofClaim(t, proofSubject(t, core.SourceSubjectPackage, "exchange"))
	snapshot := proofSnapshot(t, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")

	cases := []struct {
		mutate  func(*sourceproof.Result)
		wantErr error
		name    string
	}{
		{
			name: "claim digest names different authored bytes",
			mutate: func(result *sourceproof.Result) {
				result.ClaimDigest = core.SHA256Of([]byte("different claim"))
			},
			wantErr: core.ErrSourceProofConflict,
		},
		{
			name: "requirement result is missing",
			mutate: func(result *sourceproof.Result) {
				result.Requirements = nil
			},
			wantErr: core.ErrSourceProofContract,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := proofResult(t, claim, snapshot, sourceproof.StateHumanReviewRequired, nil)
			tc.mutate(&result)
			gotErr := result.ValidateAgainst(claim)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Result.ValidateAgainst(drifted proof) error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func TestRequirementResultRejectsIntrinsicStateEvidenceContradictions(t *testing.T) {
	t.Parallel()

	requirement := proofID(t, "human-value-review")
	snapshot := proofSnapshot(t, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	evidence := []sourceproof.EvidenceReference{proofEvidence(t, proofSubject(t, core.SourceSubjectPackage, "exchange"), snapshot)}
	cases := []struct {
		name    string
		result  sourceproof.RequirementResult
		wantErr error
	}{
		{
			name: "proven state without evidence is refused at the document wall",
			result: sourceproof.RequirementResult{
				Requirement: requirement, State: sourceproof.StateProven,
			},
			wantErr: core.ErrSourceProofContract,
		},
		{
			name: "contradicted state without counterevidence is refused at the document wall",
			result: sourceproof.RequirementResult{
				Requirement: requirement, State: sourceproof.StateContradicted,
			},
			wantErr: core.ErrSourceProofContract,
		},
		{
			name: "stale state without historical evidence is refused at the document wall",
			result: sourceproof.RequirementResult{
				Requirement: requirement, State: sourceproof.StateStale,
			},
			wantErr: core.ErrSourceProofContract,
		},
		{
			name: "human review required cannot carry an already issued receipt",
			result: sourceproof.RequirementResult{
				Requirement: requirement, Evidence: evidence, State: sourceproof.StateHumanReviewRequired,
			},
			wantErr: core.ErrSourceProofConflict,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.result.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("RequirementResult.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

type proofResultKey struct {
	subject core.SourceSubject
	claim   sourceclaim.ID
}

type proofResultResolver struct {
	results map[proofResultKey]sourceproof.Result
}

func (r proofResultResolver) ResolveResult(_ context.Context, claim sourceclaim.Claim) (sourceproof.Result, error) {
	result, ok := r.results[proofResultKey{subject: claim.Subject, claim: claim.ID}]
	if !ok {
		return sourceproof.Result{}, errors.New("proof result unavailable")
	}
	return result, nil
}

func TestProofStreamForwardsEveryAtomicResultAndDerivesLosslessRollup(t *testing.T) {
	t.Parallel()

	project := proofSubject(t, core.SourceSubjectProject, ".")
	packageSubject := proofSubject(t, core.SourceSubjectPackage, "exchange")
	file := proofSubject(t, core.SourceSubjectFile, "exchange/client.go")
	claims := []sourceclaim.Claim{
		proofClaimWithID(t, project, "project-boundary"),
		proofClaimWithID(t, packageSubject, "package-boundary"),
		proofClaimWithID(t, file, "file-boundary"),
	}
	snapshot := proofSnapshot(t, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	resolver := proofResultResolver{results: make(map[proofResultKey]sourceproof.Result, len(claims))}
	for _, claim := range claims {
		resolver.results[proofResultKey{subject: claim.Subject, claim: claim.ID}] = proofResult(t, claim, snapshot, sourceproof.StateHumanReviewRequired, nil)
	}
	var gotClaims []sourceclaim.ID
	wantStream := core.NewDigestWriter()
	gotSummary, gotErr := sourceproof.VerifyClaims(context.Background(), func(emit sourceclaim.Emit) error {
		for _, claim := range claims {
			if err := emit(claim); err != nil {
				return err
			}
		}
		return nil
	}, resolver, func(result sourceproof.Result) error {
		gotClaims = append(gotClaims, result.Claim)
		return writeProofProjection(wantStream, result)
	})
	wantDigest, wantBytes, wantDigestErr := wantStream.Seal()
	if wantDigestErr != nil {
		t.Fatalf("proof stream digest error = %v, want nil", wantDigestErr)
	}
	wantSummary := sourceproof.Summary{
		Digest: wantDigest, Bytes: wantBytes,
		ProjectClaims: 1, PackageClaims: 1, FileClaims: 1, Claims: 3,
		HumanReviewRequiredRequirements: 3, Requirements: 3,
	}
	if gotErr != nil || gotSummary != wantSummary {
		t.Fatalf("VerifyClaims(project/package/file results) = (%+v, %v), want (%+v, nil)", gotSummary, gotErr, wantSummary)
	}
	if len(gotClaims) != len(claims) {
		t.Fatalf("VerifyClaims() forwarded result count = %d, want %d", len(gotClaims), len(claims))
	}
	for index := range claims {
		if gotClaims[index] != claims[index].ID {
			t.Fatalf("VerifyClaims() result[%d] claim = %s, want %s", index, gotClaims[index].String(), claims[index].ID.String())
		}
	}
}

func writeProofProjection(destination *core.DigestWriter, result sourceproof.Result) error {
	encoded, err := result.MarshalJSON()
	if err != nil {
		return err
	}
	if _, err := destination.Write(encoded); err != nil {
		return err
	}
	_, err = destination.Write([]byte{'\n'})
	return err
}

func proofClaim(t testing.TB, subject core.SourceSubject) sourceclaim.Claim {
	t.Helper()
	return proofClaimWithID(t, subject, "exchange-bounds-http")
}

func proofClaimWithID(t testing.TB, subject core.SourceSubject, identity string) sourceclaim.Claim {
	t.Helper()

	claimID := proofID(t, identity)
	text := func(value string) sourceclaim.Text {
		got, err := sourceclaim.NewText(value)
		if err != nil {
			t.Fatalf("sourceclaim.NewText(%q) error = %v, want nil", value, err)
		}
		return got
	}
	return sourceclaim.Claim{
		ID:       claimID,
		Author:   proofAuthority(t, 1),
		Subject:  subject,
		Title:    text("Bounded HTTP exchange"),
		Problem:  sourceclaim.Narrative{Summary: text("Callers otherwise duplicate transport mechanics.")},
		Solution: sourceclaim.Narrative{Summary: text("Own one typed and bounded HTTP wall.")},
		Benefit:  sourceclaim.Narrative{Summary: text("Consumers share one observable transport contract.")},
		Removal:  sourceclaim.Narrative{Summary: text("Remove if Go owns the complete agreement directly.")},
		Owns:     []sourceclaim.Boundary{{ID: proofID(t, "http-mechanics"), Detail: text("HTTP mechanics and observations.")}},
		DoesNotOwn: []sourceclaim.Boundary{{
			ID: proofID(t, "product-policy"), Detail: text("Product meaning and permission."),
		}},
		Requirements: []sourceclaim.Requirement{{
			ID: proofID(t, "human-value-review"), Statement: text("A human confirms the shared boundary remains useful."), Mode: sourceclaim.RequirementHumanReview,
		}},
	}
}

func proofResult(t testing.TB, claim sourceclaim.Claim, snapshot core.SourceSnapshot, state sourceproof.State, evidence []sourceproof.EvidenceReference) sourceproof.Result {
	t.Helper()

	digest, err := claim.Digest()
	if err != nil {
		t.Fatalf("Claim.Digest() error = %v, want nil", err)
	}
	return sourceproof.Result{
		Claim: claim.ID, Subject: claim.Subject, ClaimDigest: digest, Snapshot: snapshot,
		Verifier:     proofAuthority(t, 2),
		Requirements: []sourceproof.RequirementResult{{Requirement: claim.Requirements[0].ID, Evidence: evidence, State: state}},
	}
}

func proofEvidence(t testing.TB, subject core.SourceSubject, snapshot core.SourceSnapshot) sourceproof.EvidenceReference {
	t.Helper()
	return sourceproof.EvidenceReference{
		Subject: subject, Digest: core.SHA256Of([]byte("review receipt")), Snapshot: snapshot,
		Authority: proofAuthority(t, 3), Kind: sourceproof.EvidenceHumanReviewReceipt,
	}
}

func proofSubject(t testing.TB, kind core.SourceSubjectKind, value string) core.SourceSubject {
	t.Helper()
	path, pathErr := core.ParseSourcePath(value)
	subject, subjectErr := core.NewSourceSubject(kind, path)
	if err := errors.Join(pathErr, subjectErr); err != nil {
		t.Fatalf("source subject fixture error = %v, want nil", err)
	}
	return subject
}

func proofSnapshot(t testing.TB, value string) core.SourceSnapshot {
	t.Helper()
	return core.SourceSnapshot{Digest: core.SHA256Of([]byte(value))}
}

func proofAuthority(t testing.TB, value byte) core.Ed25519PublicKey {
	t.Helper()

	got, err := core.NewEd25519PublicKey(ed25519.PublicKey(bytes.Repeat([]byte{value}, ed25519.PublicKeySize)))
	if err != nil {
		t.Fatalf("core.NewEd25519PublicKey() error = %v, want nil", err)
	}
	return got
}

func proofID(t testing.TB, value string) sourceclaim.ID {
	t.Helper()

	got, err := sourceclaim.NewID(value)
	if err != nil {
		t.Fatalf("sourceclaim.NewID(%q) error = %v, want nil", value, err)
	}
	return got
}
