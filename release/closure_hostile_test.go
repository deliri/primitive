package release

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestDerivedFilenameIsCanonicalForEveryTarget(t *testing.T) {
	t.Parallel()
	fixture := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)
	wants := [TargetCount]string{
		"witness-2026.7.30-windows-amd64.exe",
		"witness-2026.7.30-darwin-arm64",
		"witness-2026.7.30-linux-amd64",
		"witness-2026.7.30-linux-arm64",
	}
	for index, artifact := range fixture.artifacts {
		t.Run(wants[index], func(t *testing.T) {
			t.Parallel()
			filename, err := artifact.Filename()
			if err != nil {
				t.Fatalf("Artifact.Filename() error = %v", err)
			}
			if got := filename.String(); got != wants[index] {
				t.Fatalf("Artifact.Filename().String() = %q, want %q", got, wants[index])
			}
			if len(filename.String()) > BinaryFilenameMaximumBytes {
				t.Fatalf("filename extent = %d, want <= %d", len(filename.String()), BinaryFilenameMaximumBytes)
			}
		})
	}
}

func TestVerificationClosureRebindsRetainedDocumentBodyBeforeSealing(t *testing.T) {
	t.Parallel()
	first := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)
	second := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 31), 2)

	tamperedManifest := first.manifest
	tamperedManifest.Fact = second.manifest.Fact
	if _, err := VerifyManifest(VerifyManifestRequest{
		Document: tamperedManifest, TrustedKeys: first.manifestTrust,
		ExpectedOffering: core.OfferingWitness,
	}); !errors.Is(err, core.ErrReleaseVerification) {
		t.Fatalf("VerifyManifest(tampered body) error = %v, want %v", err, core.ErrReleaseVerification)
	}

	tamperedLatest := first.latest
	tamperedLatest.Fact = second.latest.Fact
	if _, err := VerifyLatest(VerifyLatestRequest{
		Document: tamperedLatest, LatestKeys: first.latestTrust,
		ManifestKeys: second.manifestTrust, ExpectedOffering: core.OfferingWitness,
	}); !errors.Is(err, core.ErrReleaseVerification) {
		t.Fatalf("VerifyLatest(tampered body) error = %v, want %v", err, core.ErrReleaseVerification)
	}
}

func TestVerifiedCarriersRejectEveryMissingPrivateSeal(t *testing.T) {
	t.Parallel()
	fixture := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)

	manifest := fixture.verified
	manifest.seal = verificationSealUnknown
	if err := manifest.Validate(); !errors.Is(err, core.ErrReleaseVerification) {
		t.Fatalf("VerifiedManifest.Validate(unsealed) error = %v, want %v", err, core.ErrReleaseVerification)
	}

	latest := fixture.verifiedLatest
	latest.seal = verificationSealUnknown
	if err := latest.Validate(); !errors.Is(err, core.ErrReleaseVerification) {
		t.Fatalf("VerifiedLatest.Validate(unsealed) error = %v, want %v", err, core.ErrReleaseVerification)
	}
}

func TestArtifactRejectsAnOtherwiseCompleteValueWithoutItsConstructorBit(t *testing.T) {
	t.Parallel()
	fixture := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)
	artifact := fixture.artifacts[0]
	artifact.valid = false
	if err := artifact.Validate(); !errors.Is(err, core.ErrReleaseManifest) {
		t.Fatalf("Artifact.Validate(unset) error = %v, want %v", err, core.ErrReleaseManifest)
	}
}

// TestArtifactSetDecodeRejectsEveryCardinalityBesidesTargetCount pins the
// exact-arity wire contract. Decoding into a fixed Go array silently discards
// trailing elements, so an over-long signed array previously decoded clean.
func TestArtifactSetDecodeRejectsEveryCardinalityBesidesTargetCount(t *testing.T) {
	t.Parallel()
	fixture := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)
	encoded := make([]string, TargetCount)
	for index, artifact := range fixture.artifacts {
		value, err := json.Marshal(artifact)
		if err != nil {
			t.Fatalf("json.Marshal(artifact %d) error = %v", index, err)
		}
		encoded[index] = string(value)
	}
	array := func(count int) string {
		items := make([]string, count)
		for index := range items {
			items[index] = encoded[index%TargetCount]
		}
		return "[" + strings.Join(items, ",") + "]"
	}

	cases := []struct {
		wantErr error
		name    string
		data    string
	}{
		{name: "empty array carries no target", data: "[]", wantErr: core.ErrJSONContract},
		{name: "one below target count is rejected", data: array(TargetCount - 1), wantErr: core.ErrJSONContract},
		{name: "exact target count is accepted", data: array(TargetCount)},
		{name: "one above target count is rejected", data: array(TargetCount + 1), wantErr: core.ErrJSONContract},
		{name: "double target count is rejected", data: array(2 * TargetCount), wantErr: core.ErrJSONContract},
		{name: "object instead of array is rejected", data: "{}", wantErr: core.ErrJSONContract},
		{name: "null instead of array is rejected", data: "null", wantErr: core.ErrJSONContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := fixture.artifactSet
			err := json.Unmarshal([]byte(tc.data), &got)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("ArtifactSet.UnmarshalJSON() error = %v, want nil", err)
				}
				if got != fixture.artifactSet {
					t.Fatalf("ArtifactSet.UnmarshalJSON() = %v, want the canonical set", got)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ArtifactSet.UnmarshalJSON() error = %v, want %v", err, tc.wantErr)
			}
			if got != fixture.artifactSet {
				t.Fatalf("ArtifactSet.UnmarshalJSON() mutated the receiver on rejection")
			}
		})
	}

	// Trailing data is the decoder's own contract: encoding/json rejects it
	// before the receiver is reached, so assert it against the owned method.
	trailing := fixture.artifactSet
	err := trailing.UnmarshalJSON([]byte(array(TargetCount) + "0"))
	if !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("ArtifactSet.UnmarshalJSON(trailing data) error = %v, want %v", err, core.ErrJSONContract)
	}
	if trailing != fixture.artifactSet {
		t.Fatalf("ArtifactSet.UnmarshalJSON(trailing data) mutated the receiver on rejection")
	}
	var nilSet *ArtifactSet
	if err := nilSet.UnmarshalJSON([]byte(array(TargetCount))); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil ArtifactSet.UnmarshalJSON() error = %v, want %v", err, core.ErrJSONContract)
	}
}

// TestArtifactSetValidateRejectsATamperedSealedTotal proves the sealed total
// is checked against a fresh sum rather than trusted.
func TestArtifactSetValidateRejectsATamperedSealedTotal(t *testing.T) {
	t.Parallel()
	fixture := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)
	honest, err := fixture.artifactSet.TotalExtent()
	if err != nil {
		t.Fatalf("ArtifactSet.TotalExtent() error = %v", err)
	}
	honestValue, err := honest.Uint64()
	if err != nil {
		t.Fatalf("ByteCount.Uint64() error = %v", err)
	}
	for _, delta := range []uint64{honestValue - 1, honestValue + 1, 1 << 40} {
		t.Run("sealed total "+strconv.FormatUint(delta, 10)+" is rejected", func(t *testing.T) {
			t.Parallel()
			tampered := fixture.artifactSet
			tampered.total = mustByteCount(t, delta)
			if err := tampered.Validate(); !errors.Is(err, core.ErrReleaseManifest) {
				t.Fatalf("ArtifactSet.Validate(total %d) error = %v, want %v", delta, err, core.ErrReleaseManifest)
			}
		})
	}
}

// TestVerificationNamesOfferingMismatchDistinctlyFromInvalidDocument keeps the
// two rejection classes distinguishable to an operator. They previously
// collapsed into one diagnostic-free error.
func TestVerificationNamesOfferingMismatchDistinctlyFromInvalidDocument(t *testing.T) {
	t.Parallel()
	fixture := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)

	_, manifestErr := VerifyManifest(VerifyManifestRequest{
		Document: fixture.manifest, TrustedKeys: fixture.manifestTrust,
		ExpectedOffering: core.OfferingBug,
	})
	if !errors.Is(manifestErr, core.ErrReleaseVerification) {
		t.Fatalf("VerifyManifest(wrong offering) error = %v, want %v", manifestErr, core.ErrReleaseVerification)
	}
	var manifestMismatch OfferingMismatchError
	if !errors.As(manifestErr, &manifestMismatch) ||
		manifestMismatch.Observed() != core.OfferingWitness ||
		manifestMismatch.Expected() != core.OfferingBug {
		t.Fatalf(
			"VerifyManifest(wrong offering) detail = (%v, %v, %v), want (%v, %v, true)",
			manifestMismatch.Observed(),
			manifestMismatch.Expected(),
			errors.As(manifestErr, &manifestMismatch),
			core.OfferingWitness,
			core.OfferingBug,
		)
	}

	_, latestErr := VerifyLatest(VerifyLatestRequest{
		Document: fixture.latest, LatestKeys: fixture.latestTrust,
		ManifestKeys: fixture.manifestTrust, ExpectedOffering: core.OfferingBug,
	})
	if !errors.Is(latestErr, core.ErrReleaseVerification) {
		t.Fatalf("VerifyLatest(wrong offering) error = %v, want %v", latestErr, core.ErrReleaseVerification)
	}
	var latestMismatch OfferingMismatchError
	if !errors.As(latestErr, &latestMismatch) ||
		latestMismatch.Observed() != core.OfferingWitness ||
		latestMismatch.Expected() != core.OfferingBug {
		t.Fatalf(
			"VerifyLatest(wrong offering) detail = (%v, %v, %v), want (%v, %v, true)",
			latestMismatch.Observed(),
			latestMismatch.Expected(),
			errors.As(latestErr, &latestMismatch),
			core.OfferingWitness,
			core.OfferingBug,
		)
	}

	// A structurally invalid document must not borrow the offering diagnostic.
	broken := fixture.manifest
	broken.Fact = ManifestFact{}
	_, brokenErr := VerifyManifest(VerifyManifestRequest{
		Document: broken, TrustedKeys: fixture.manifestTrust,
		ExpectedOffering: core.OfferingWitness,
	})
	if !errors.Is(brokenErr, core.ErrReleaseVerification) {
		t.Fatalf("VerifyManifest(unset fact) error = %v, want %v", brokenErr, core.ErrReleaseVerification)
	}
	var brokenMismatch OfferingMismatchError
	if errors.As(brokenErr, &brokenMismatch) {
		t.Fatalf("VerifyManifest(unset fact) detail = %v, want no offering mismatch", brokenMismatch)
	}
}

func TestOfferingMismatchErrorOwnsExactTypedFacts(t *testing.T) {
	t.Parallel()

	mismatch, err := newOfferingMismatchError(core.OfferingWitness, core.OfferingBug)
	if err != nil || mismatch.Validate() != nil ||
		mismatch.Observed() != core.OfferingWitness ||
		mismatch.Expected() != core.OfferingBug ||
		!errors.Is(mismatch, core.ErrReleaseVerification) {
		t.Fatalf("newOfferingMismatchError() = (%v, %v), want validated exact verification detail", mismatch, err)
	}
	cases := []struct {
		name     string
		observed core.Offering
		want     core.Offering
	}{
		{name: "unknown observed", observed: core.OfferingUnknown, want: core.OfferingBug},
		{name: "future observed", observed: core.Offering(255), want: core.OfferingBug},
		{name: "unknown expected", observed: core.OfferingBug, want: core.OfferingUnknown},
		{name: "future expected", observed: core.OfferingBug, want: core.Offering(255)},
		{name: "equal bug", observed: core.OfferingBug, want: core.OfferingBug},
		{name: "equal witness", observed: core.OfferingWitness, want: core.OfferingWitness},
		{name: "equal peachfuzz", observed: core.OfferingPeachfuzz, want: core.OfferingPeachfuzz},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := newOfferingMismatchError(tc.observed, tc.want)
			if got != (OfferingMismatchError{}) || !errors.Is(gotErr, core.ErrReleaseContract) {
				t.Fatalf("newOfferingMismatchError(%v, %v) = (%v, %v), want (zero, %v)", tc.observed, tc.want, got, gotErr, core.ErrReleaseContract)
			}
		})
	}
	if err := (OfferingMismatchError{}).Validate(); !errors.Is(err, core.ErrReleaseContract) {
		t.Fatalf("OfferingMismatchError{}.Validate() error = %v, want %v", err, core.ErrReleaseContract)
	}
}

func TestTaggedUnionsRejectEverySecondActiveArm(t *testing.T) {
	t.Parallel()
	fixture := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)
	assessment, err := AssessLatest(AssessLatestRequest{
		Latest:      fixture.verifiedLatest,
		Observation: temporal.InstantFromNanoseconds(3_000),
	})
	if err != nil {
		t.Fatalf("AssessLatest() error = %v", err)
	}
	current := CurrentRelease{
		manifest:   fixture.manifest.Fact.Identity(),
		artifact:   fixture.artifacts[2].Identity(),
		version:    fixture.manifest.Fact.Version(),
		validUntil: assessment.ValidUntil(),
		valid:      true,
	}
	selection := Selection{
		current: current, refresh: RefreshDirective{valid: true},
		state: SelectionCurrent, valid: true,
	}
	if err := selection.Validate(); !errors.Is(err, core.ErrReleaseContract) {
		t.Fatalf("Selection.Validate(two active arms) error = %v, want %v", err, core.ErrReleaseContract)
	}

	preparation := Preparation{
		refresh:  RefreshDirective{valid: true},
		reassess: ReassessDirective{At: temporal.InstantFromNanoseconds(4_000)},
		state:    SelectionRefreshRequired,
		valid:    true,
	}
	if err := preparation.Validate(); !errors.Is(err, core.ErrReleaseContract) {
		t.Fatalf("Preparation.Validate(two active arms) error = %v, want %v", err, core.ErrReleaseContract)
	}
}
