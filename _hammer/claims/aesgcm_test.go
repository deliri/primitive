package claims

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/sourceclaim"
)

func TestAESGCMAuthoredClaimLayerTriad(t *testing.T) {
	t.Parallel()
	var gotAESGCM, gotAttest uint64
	_, err := sourceclaim.Consume(Stream, func(claim sourceclaim.Claim) error {
		if claim.Subject.Kind != core.SourceSubjectPackage {
			return nil
		}
		switch claim.Subject.Path.String() {
		case core.PackageAESGCM.String():
			gotAESGCM++
		case core.PackageAttest.String():
			gotAttest++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("authored claim stream error=%v, want nil", err)
	}
	if gotAESGCM == 0 || gotAttest == 0 {
		t.Fatalf("authored claims aesgcm=%d attest=%d, want both independently present", gotAESGCM, gotAttest)
	}
	_, err = sourceclaim.Consume(func(emit sourceclaim.Emit) error {
		return Stream(func(claim sourceclaim.Claim) error {
			if err := emit(claim); err != nil {
				return err
			}
			if claim.Subject.Kind == core.SourceSubjectPackage && claim.Subject.Path.String() == core.PackageAESGCM.String() {
				return emit(claim)
			}
			return nil
		})
	}, func(sourceclaim.Claim) error { return nil })
	if !errors.Is(err, core.ErrSourceClaimConflict) {
		t.Fatalf("duplicate AESGCM claim error=%v, want canonical-order refusal", err)
	}
}
