package release

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzLatestDocumentJSON(f *testing.F) {
	fixture := newReleaseFixture(f, core.NewReleaseVersion(2026, 7, 30), 1)
	seed, err := json.Marshal(fixture.latest)
	if err != nil {
		f.Fatalf("json.Marshal(LatestDocument) error = %v", err)
	}
	f.Add(seed)
	f.Add([]byte(`null`))
	f.Add([]byte(`{"fact":{}}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var got LatestDocument
		err := got.UnmarshalJSON(data)
		if err != nil {
			if !errors.Is(err, core.ErrReleaseContract) {
				t.Fatalf("LatestDocument.UnmarshalJSON() error = %v, want %v", err, core.ErrReleaseContract)
			}
			if got != (LatestDocument{}) {
				t.Fatalf("rejected LatestDocument mutated its receiver")
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("LatestDocument.Validate() error = %v, want nil", err)
		}
		canonical, err := got.MarshalJSON()
		if err != nil {
			t.Fatalf("LatestDocument.MarshalJSON() error = %v, want nil", err)
		}
		if len(canonical) > documentExtentMaximum {
			t.Fatalf("canonical LatestDocument extent = %d, want <= %d", len(canonical), documentExtentMaximum)
		}
		var roundTrip LatestDocument
		if err := roundTrip.UnmarshalJSON(canonical); err != nil {
			t.Fatalf("canonical LatestDocument.UnmarshalJSON() error = %v, want nil", err)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil {
			t.Fatalf("second LatestDocument.MarshalJSON() error = %v, want nil", err)
		}
		if roundTrip != got || !bytes.Equal(canonical, second) {
			t.Fatalf("accepted LatestDocument lacks an idempotent canonical round trip")
		}
		verified, verifyErr := VerifyLatest(VerifyLatestRequest{
			Document: roundTrip, LatestKeys: fixture.latestTrust,
			ManifestKeys:     fixture.manifestTrust,
			ExpectedOffering: core.OfferingWitness,
		})
		if verifyErr != nil {
			if !errors.Is(verifyErr, core.ErrReleaseVerification) {
				t.Fatalf("VerifyLatest() error = %v, want %v", verifyErr, core.ErrReleaseVerification)
			}
			if verified != (VerifiedLatest{}) {
				t.Fatalf("rejected VerifyLatest() returned a nonzero proof")
			}
			return
		}
		if roundTrip != fixture.latest {
			t.Fatalf("VerifyLatest() authenticated a document other than the signed fixture")
		}
	})
}
