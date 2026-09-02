package reviewcontrol

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"github.com/deliri/primitive/v2026/projectstandards"
)

func TestVerifySubjectClassifiesEveryIdentityAndSourceMutation(t *testing.T) {
	t.Parallel()
	baseline := reviewSubject(t)
	otherRepository, err := projectstandards.NewRepositoryIdentity("github.com/deliri/other")
	if err != nil {
		t.Fatalf("NewRepositoryIdentity(other) error = %v, want nil", err)
	}
	otherModule, err := gomodule.ParsePath("github.com/deliri/other/v2026")
	if err != nil {
		t.Fatalf("ParsePath(other) error = %v, want nil", err)
	}
	otherPackage, err := gomodule.ParseImportPath("github.com/deliri/primitive/v2026/core")
	if err != nil {
		t.Fatalf("ParseImportPath(other) error = %v, want nil", err)
	}
	otherFile, err := projectstandards.ParseSourcePath("reviewcontrol/event.go")
	if err != nil {
		t.Fatalf("ParseSourcePath(other) error = %v, want nil", err)
	}
	otherCommit, err := core.ParseBuildCommit("1123456789abcdef0123456789abcdef01234567")
	if err != nil {
		t.Fatalf("ParseBuildCommit(other) error = %v, want nil", err)
	}
	otherBytes, err := core.NewByteCount(513)
	if err != nil {
		t.Fatalf("NewByteCount(other) error = %v, want nil", err)
	}
	cases := []struct {
		name    string
		mutate  func(*Subject)
		wantErr error
	}{
		{name: "project identity changes subject", mutate: func(s *Subject) { s.Project.Project = core.Offering{Token: "other"} }, wantErr: core.ErrReviewControlSubjectMismatch},
		{name: "repository identity changes subject", mutate: func(s *Subject) {
			s.Project.Repository = otherRepository
			s.Source.Repository = otherRepository
		}, wantErr: core.ErrReviewControlSubjectMismatch},
		{name: "module identity changes subject", mutate: func(s *Subject) {
			s.Module = otherModule
			s.Package, _ = gomodule.ParseImportPath("github.com/deliri/other/v2026/reviewcontrol")
		}, wantErr: core.ErrReviewControlSubjectMismatch},
		{name: "package identity changes subject", mutate: func(s *Subject) {
			s.Package = otherPackage
			s.File, _ = projectstandards.ParseSourcePath("core/error_identity.go")
		}, wantErr: core.ErrReviewControlSubjectMismatch},
		{name: "repository-relative file changes subject", mutate: func(s *Subject) { s.File = otherFile }, wantErr: core.ErrReviewControlSubjectMismatch},
		{name: "commit changes exact source", mutate: func(s *Subject) { s.Source.Commit = otherCommit }, wantErr: core.ErrReviewControlStaleSource},
		{name: "tree digest changes exact source", mutate: func(s *Subject) { s.Source.Tree = core.SHA256Of([]byte("other-tree")) }, wantErr: core.ErrReviewControlStaleSource},
		{name: "digest changes exact source", mutate: func(s *Subject) { s.SHA256 = core.SHA256Of([]byte("other source")) }, wantErr: core.ErrReviewControlStaleSource},
		{name: "byte extent changes exact source", mutate: func(s *Subject) { s.Bytes = otherBytes }, wantErr: core.ErrReviewControlStaleSource},
	}
	if gotErr := VerifySubject(baseline, baseline); gotErr != nil {
		t.Fatalf("VerifySubject(exact) error = %v, want nil", gotErr)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			changed := baseline
			tc.mutate(&changed)
			if changed == baseline {
				t.Fatalf("subject mutation = baseline, want one changed load-bearing fact")
			}
			if gotErr := VerifySubject(baseline, changed); !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("VerifySubject(one-fact mutation) error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func TestSubjectAdmitsExactNonGoSourceInTheOwnedPackageDirectory(t *testing.T) {
	t.Parallel()

	subject := reviewSubject(t)
	nonGo, err := projectstandards.ParseSourcePath("reviewcontrol/README.md")
	if err != nil {
		t.Fatalf("ParseSourcePath(non-Go source) error = %v, want nil", err)
	}
	subject.File = nonGo
	if gotErr := subject.Validate(); gotErr != nil {
		t.Fatalf("Subject.Validate(non-Go exact source) error = %v, want nil", gotErr)
	}
	outside, err := projectstandards.ParseSourcePath("proofledger/README.md")
	if err != nil {
		t.Fatalf("ParseSourcePath(outside source) error = %v, want nil", err)
	}
	subject.File = outside
	if gotErr := subject.Validate(); !errors.Is(gotErr, core.ErrReviewControlSubjectMismatch) {
		t.Fatalf("Subject.Validate(outside package) error = %v, want errors.Is(..., %v)", gotErr, core.ErrReviewControlSubjectMismatch)
	}
}

func TestMachineCheckWithoutProbeIsAMalformedContractNotMissingEvidence(t *testing.T) {
	t.Parallel()

	packet := reviewPacket(t)
	check := packet.Contract.RequiredChecks[0]
	check.Probe = nil
	if check.Kind == CheckManualInspection {
		check.Kind = CheckCompile
	}
	gotErr := check.Validate()
	if !errors.Is(gotErr, core.ErrReviewControlContract) || errors.Is(gotErr, core.ErrReviewControlMissingEvidence) {
		t.Fatalf("CheckRequirement.Validate(missing machine probe) error = %v, want contract identity without missing-evidence identity", gotErr)
	}
}

func TestValidateDecisionRejectsEveryAuthoritySourceObservationAndEvidenceMismatch(t *testing.T) {
	t.Parallel()
	packet := reviewPacket(t)
	observation := reviewObservation(t, packet)
	authority := verifiedHuman(t, AuthorityHuman)
	intent := reviewDecision(t, packet, observation, DecisionAccept)
	findingIdentity, _ := NewFindingIdentity(reviewUUID(t, 8))
	summary, _ := NewFindingSummary("Exact source binding inspected")
	detail, _ := NewFindingDetail("The finding is bound to a packet-owned source path.")
	location := SourceLocation{Path: packet.Subject.File, Line: 1}
	finding := Finding{Identity: findingIdentity, Rule: reviewIdentifier(t, "review.source"), Severity: SeverityInformational, Location: &location, Summary: summary, Detail: detail}
	withFinding := observation
	withFinding.Findings = []Finding{finding}
	if gotErr := ValidateDecision(DecisionValidation{Packet: packet, Observation: &withFinding, Authority: authority, Intent: intent}); gotErr != nil {
		t.Fatalf("ValidateDecision(packet-owned finding location) error = %v, want nil", gotErr)
	}
	unable := observation
	unable.Verdict = VerdictUnableToReview
	if gotErr := ValidateDecision(DecisionValidation{Packet: packet, Observation: &unable, Authority: authority, Intent: intent}); gotErr != nil {
		t.Fatalf("ValidateDecision(non-passing advisory observation) error = %v, want nil because acceptance policy belongs to the product", gotErr)
	}
	cases := []struct {
		name    string
		mutate  func(*DecisionValidation)
		wantErr error
	}{
		{name: "wire data cannot supply authority", mutate: func(v *DecisionValidation) { v.Authority = VerifiedHumanAuthority{} }, wantErr: core.ErrReviewControlUnauthorizedAuthority},
		{name: "another review cannot be accepted", mutate: func(v *DecisionValidation) { other, _ := NewReviewIdentity(reviewUUID(t, 7)); v.Intent.Review = other }, wantErr: core.ErrReviewControlSubjectMismatch},
		{name: "changed commit is stale", mutate: func(v *DecisionValidation) {
			changed, _ := core.ParseBuildCommit("1123456789abcdef0123456789abcdef01234567")
			v.Intent.Subject.Source.Commit = changed
		}, wantErr: core.ErrReviewControlStaleSource},
		{name: "changed digest is stale", mutate: func(v *DecisionValidation) { v.Intent.Subject.SHA256 = core.SHA256Of([]byte("changed")) }, wantErr: core.ErrReviewControlStaleSource},
		{name: "changed extent is stale", mutate: func(v *DecisionValidation) { changed, _ := core.NewByteCount(999); v.Intent.Subject.Bytes = changed }, wantErr: core.ErrReviewControlStaleSource},
		{name: "changed contract digest is refused", mutate: func(v *DecisionValidation) { v.Intent.Contract = core.SHA256Of([]byte("other contract")) }, wantErr: core.ErrReviewControlSubjectMismatch},
		{name: "missing observation is refused", mutate: func(v *DecisionValidation) { v.Observation = nil }, wantErr: core.ErrReviewControlObservationMismatch},
		{name: "another observation identity is refused", mutate: func(v *DecisionValidation) {
			other, _ := NewObservationIdentity(reviewUUID(t, 7))
			v.Intent.Observation = &other
		}, wantErr: core.ErrReviewControlObservationMismatch},
		{name: "observation for another review is refused", mutate: func(v *DecisionValidation) {
			other, _ := NewReviewIdentity(reviewUUID(t, 7))
			changed := *v.Observation
			changed.Review = other
			v.Observation = &changed
		}, wantErr: core.ErrReviewControlObservationMismatch},
		{name: "missing required evidence is refused", mutate: func(v *DecisionValidation) {
			changed := *v.Observation
			changed.Evidence = nil
			v.Observation = &changed
		}, wantErr: core.ErrReviewControlMissingEvidence},
		{name: "equal evidence count for an unrelated requirement is refused", mutate: func(v *DecisionValidation) {
			changed := *v.Observation
			changed.Evidence = append([]EvidenceReference(nil), changed.Evidence...)
			changed.Evidence[0].Requirement = reviewIdentifier(t, "unrelated.requirement")
			v.Observation = &changed
		}, wantErr: core.ErrReviewControlMissingEvidence},
		{name: "duplicate evidence cannot satisfy one requirement twice", mutate: func(v *DecisionValidation) {
			changed := *v.Observation
			changed.Evidence = append(append([]EvidenceReference(nil), changed.Evidence...), changed.Evidence[0])
			v.Observation = &changed
		}, wantErr: core.ErrReviewControlMissingEvidence},
		{name: "unpublished machine evidence is refused", mutate: func(v *DecisionValidation) {
			changed := *v.Observation
			changed.Evidence = append([]EvidenceReference(nil), changed.Evidence...)
			changed.Evidence[0].Receipt.Published = false
			v.Observation = &changed
		}, wantErr: core.ErrReviewControlMissingEvidence},
		{name: "finding outside packet source and context is refused", mutate: func(v *DecisionValidation) {
			foreignPath, _ := projectstandards.ParseSourcePath("foreign/source.go")
			foreignLocation := location
			foreignLocation.Path = foreignPath
			foreignFinding := finding
			foreignFinding.Location = &foreignLocation
			changed := *v.Observation
			changed.Findings = []Finding{foreignFinding}
			v.Observation = &changed
		}, wantErr: core.ErrReviewControlSubjectMismatch},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := DecisionValidation{Packet: packet, Observation: &observation, Authority: authority, Intent: intent}
			tc.mutate(&request)
			if gotErr := ValidateDecision(request); !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("ValidateDecision(one-fact mutation) error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}
