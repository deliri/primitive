package retrievalauth

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestRetrievalResponseBoundaryRefusesEveryNeutralInput(t *testing.T) {
	t.Parallel()

	issuance := ResponseIssuance{}
	if err := issuance.Validate(); !errors.Is(err, core.ErrControlPlaneResponseDocument) || !errors.Is(err, core.ErrControlPlaneResponseHeader) {
		t.Fatalf("ResponseIssuance.Validate() error = %v, want %v/%v", err, core.ErrControlPlaneResponseDocument, core.ErrControlPlaneResponseHeader)
	}
	projection, err := IssueResponse(issuance)
	if !errors.Is(err, core.ErrControlPlaneResponseDocument) || projection.Validate() == nil {
		t.Fatalf("IssueResponse(zero) = (%v, %v), want invalid zero projection and %v", projection, err, core.ErrControlPlaneResponseDocument)
	}
	verification := ResponseVerification{}
	if err := verification.Validate(); !errors.Is(err, core.ErrControlPlaneResponseDocument) {
		t.Fatalf("ResponseVerification.Validate() error = %v, want %v", err, core.ErrControlPlaneResponseDocument)
	}
	verified, err := VerifyResponse(verification)
	if !errors.Is(err, core.ErrControlPlaneResponseDocument) || verified.Validate() == nil {
		t.Fatalf("VerifyResponse(zero) = (%v, %v), want invalid zero proof and %v", verified, err, core.ErrControlPlaneResponseDocument)
	}
}
