package core_test

import (
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestReleaseVersionBoundaries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		text    string
		want    core.ReleaseVersion
	}{
		{name: "zero semantic version remains a valid release", text: "0.0.0", want: core.NewReleaseVersion(0, 0, 0)},
		{name: "ordinary calendar version is accepted", text: "2026.7.30", want: core.NewReleaseVersion(2026, 7, 30)},
		{name: "every component reaches uint32 maximum", text: "4294967295.4294967295.4294967295", want: core.NewReleaseVersion(math.MaxUint32, math.MaxUint32, math.MaxUint32)},
		{name: "empty text is rejected", text: "", wantErr: core.ErrPrimitiveContract},
		{name: "missing patch is rejected", text: "2026.7", wantErr: core.ErrPrimitiveContract},
		{name: "extra component is rejected", text: "2026.7.30.1", wantErr: core.ErrPrimitiveContract},
		{name: "leading zero is rejected", text: "02026.7.30", wantErr: core.ErrPrimitiveContract},
		{name: "signed component is rejected", text: "+2026.7.30", wantErr: core.ErrPrimitiveContract},
		{name: "uint32 overflow is rejected", text: "4294967296.0.0", wantErr: core.ErrPrimitiveContract},
		{name: "prerelease convention is not implicit", text: "2026.7.30-rc1", wantErr: core.ErrPrimitiveContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := core.ParseReleaseVersion(tc.text)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ParseReleaseVersion(%q) error = %v, want %v", tc.text, err, tc.wantErr)
			}
			if err == nil && got != tc.want {
				t.Fatalf("ParseReleaseVersion(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}

func TestBuildCommitBoundaries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		text    string
	}{
		{name: "sha1 lower hexadecimal is accepted", text: "0123456789abcdef0123456789abcdef01234567"},
		{name: "sha256 lower hexadecimal is accepted", text: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
		{name: "one byte below sha1 is rejected", text: "0123456789abcdef0123456789abcdef012345", wantErr: core.ErrPrimitiveContract},
		{name: "width between sha1 and sha256 is rejected", text: "0123456789abcdef0123456789abcdef0123456789", wantErr: core.ErrPrimitiveContract},
		{name: "one byte above sha256 is rejected", text: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef00", wantErr: core.ErrPrimitiveContract},
		{name: "uppercase is rejected", text: "0123456789ABCDEF0123456789abcdef01234567", wantErr: core.ErrPrimitiveContract},
		{name: "nonhex is rejected", text: "g123456789abcdef0123456789abcdef01234567", wantErr: core.ErrPrimitiveContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := core.ParseBuildCommit(tc.text)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ParseBuildCommit(%q) error = %v, want %v", tc.text, err, tc.wantErr)
			}
			if err == nil && got.String() != tc.text {
				t.Fatalf("BuildCommit.String() = %q, want %q", got.String(), tc.text)
			}
		})
	}
}

func TestBuildIdentityJSONRejectsLooseProtocol(t *testing.T) {
	t.Parallel()

	identity := releaseIdentityFixture(t, core.OfferingWitness)
	encoded, err := json.Marshal(identity)
	if err != nil {
		t.Fatalf("json.Marshal(BuildIdentity) error = %v", err)
	}
	var got core.BuildIdentity
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("json.Unmarshal(BuildIdentity) error = %v", err)
	}
	if got != identity {
		t.Fatalf("BuildIdentity round trip = %v, want %v", got, identity)
	}

	hostile := [][]byte{
		[]byte(`null`),
		[]byte(`{}`),
		[]byte(`{"offering":"witness","offering":"witness","version":"2026.7.30","commit":"0123456789abcdef0123456789abcdef01234567","platform":"linux-amd64"}`),
		[]byte(`{"offering":"witness","version":"2026.7.30","commit":"0123456789abcdef0123456789abcdef01234567","platform":"linux-amd64","future":true}`),
	}
	for _, data := range hostile {
		var receiver core.BuildIdentity
		err := json.Unmarshal(data, &receiver)
		if !errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("json.Unmarshal(%q) error = %v, want %v", data, err, core.ErrJSONContract)
		}
		if receiver != (core.BuildIdentity{}) {
			t.Fatalf("json.Unmarshal(%q) mutated receiver to %v", data, receiver)
		}
	}
}

func TestOfferingExhaustsCompleteByteDomain(t *testing.T) {
	t.Parallel()

	for value := range 256 {
		offering := core.Offering(value)
		wantValid := offering == core.OfferingBug ||
			offering == core.OfferingWitness ||
			offering == core.OfferingPeachfuzz
		if offering.IsValid() != wantValid {
			t.Fatalf("Offering(%d).IsValid() = %v, want %v", value, offering.IsValid(), wantValid)
		}
		if (offering.String() != "") != wantValid {
			t.Fatalf("Offering(%d).String() = %q, want nonempty=%v", value, offering.String(), wantValid)
		}
		if !wantValid {
			continue
		}
		parsed, err := core.ParseOffering(offering.String())
		if err != nil || parsed != offering {
			t.Fatalf("ParseOffering(%q) = (%v, %v), want (%v, nil)", offering.String(), parsed, err, offering)
		}
		encoded, err := json.Marshal(offering)
		if err != nil {
			t.Fatalf("json.Marshal(Offering(%d)) error = %v", value, err)
		}
		var roundTrip core.Offering
		if err := json.Unmarshal(encoded, &roundTrip); err != nil || roundTrip != offering {
			t.Fatalf("Offering(%d) round trip = (%v, %v), want (%v, nil)", value, roundTrip, err, offering)
		}
	}

	if _, err := core.ParseOffering("Witness"); !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("ParseOffering(alternate case) error = %v, want %v", err, core.ErrPrimitiveContract)
	}
	var receiver *core.Offering
	if err := receiver.UnmarshalJSON([]byte(`"witness"`)); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil Offering.UnmarshalJSON() error = %v, want %v", err, core.ErrJSONContract)
	}
	if _, err := json.Marshal(core.OfferingUnknown); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("json.Marshal(OfferingUnknown) error = %v, want %v", err, core.ErrJSONContract)
	}
}

func TestReleaseOfferingMismatchErrorOwnsExactTypedFacts(t *testing.T) {
	t.Parallel()

	mismatch, err := core.NewReleaseOfferingMismatchError(
		core.OfferingWitness,
		core.OfferingBug,
	)
	if err != nil {
		t.Fatalf("NewReleaseOfferingMismatchError() error = %v, want nil", err)
	}
	if err := mismatch.Validate(); err != nil {
		t.Fatalf("ReleaseOfferingMismatchError.Validate() error = %v, want nil", err)
	}
	if mismatch.Observed() != core.OfferingWitness ||
		mismatch.Expected() != core.OfferingBug {
		t.Fatalf(
			"ReleaseOfferingMismatchError facts = (%v, %v), want (%v, %v)",
			mismatch.Observed(),
			mismatch.Expected(),
			core.OfferingWitness,
			core.OfferingBug,
		)
	}
	if !errors.Is(mismatch, core.ErrReleaseVerification) {
		t.Fatalf(
			"ReleaseOfferingMismatchError identity = %v, want %v",
			mismatch,
			core.ErrReleaseVerification,
		)
	}

	cases := []struct {
		name         string
		observed     core.Offering
		wantOffering core.Offering
	}{
		{name: "unknown observed offering is rejected", observed: core.OfferingUnknown, wantOffering: core.OfferingBug},
		{name: "future observed offering is rejected", observed: core.Offering(255), wantOffering: core.OfferingBug},
		{name: "unknown expected offering is rejected", observed: core.OfferingBug, wantOffering: core.OfferingUnknown},
		{name: "future expected offering is rejected", observed: core.OfferingBug, wantOffering: core.Offering(255)},
		{name: "equal bug offerings are not a mismatch", observed: core.OfferingBug, wantOffering: core.OfferingBug},
		{name: "equal witness offerings are not a mismatch", observed: core.OfferingWitness, wantOffering: core.OfferingWitness},
		{name: "equal peachfuzz offerings are not a mismatch", observed: core.OfferingPeachfuzz, wantOffering: core.OfferingPeachfuzz},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := core.NewReleaseOfferingMismatchError(tc.observed, tc.wantOffering)
			if !errors.Is(err, core.ErrPrimitiveContract) {
				t.Fatalf(
					"NewReleaseOfferingMismatchError(%v, %v) error = %v, want %v",
					tc.observed,
					tc.wantOffering,
					err,
					core.ErrPrimitiveContract,
				)
			}
			if got != (core.ReleaseOfferingMismatchError{}) {
				t.Fatalf(
					"NewReleaseOfferingMismatchError(%v, %v) = %v, want zero",
					tc.observed,
					tc.wantOffering,
					got,
				)
			}
		})
	}
	if err := (core.ReleaseOfferingMismatchError{}).Validate(); !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("zero ReleaseOfferingMismatchError.Validate() error = %v, want %v", err, core.ErrPrimitiveContract)
	}
}

func TestReleaseVersionComparePressuresEveryComponent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		left  core.ReleaseVersion
		right core.ReleaseVersion
		want  core.Comparison
	}{
		{name: "exact equality", left: core.NewReleaseVersion(2026, 7, 30), right: core.NewReleaseVersion(2026, 7, 30), want: core.ComparisonEqual},
		{name: "patch one below", left: core.NewReleaseVersion(2026, 7, 29), right: core.NewReleaseVersion(2026, 7, 30), want: core.ComparisonLess},
		{name: "patch one above", left: core.NewReleaseVersion(2026, 7, 31), right: core.NewReleaseVersion(2026, 7, 30), want: core.ComparisonGreater},
		{name: "minor dominates maximum patch", left: core.NewReleaseVersion(2026, 6, math.MaxUint32), right: core.NewReleaseVersion(2026, 7, 0), want: core.ComparisonLess},
		{name: "minor above dominates zero patch", left: core.NewReleaseVersion(2026, 8, 0), right: core.NewReleaseVersion(2026, 7, math.MaxUint32), want: core.ComparisonGreater},
		{name: "major dominates maximum minor and patch", left: core.NewReleaseVersion(2025, math.MaxUint32, math.MaxUint32), right: core.NewReleaseVersion(2026, 0, 0), want: core.ComparisonLess},
		{name: "maximum release exceeds zero release", left: core.NewReleaseVersion(math.MaxUint32, math.MaxUint32, math.MaxUint32), right: core.NewReleaseVersion(0, 0, 0), want: core.ComparisonGreater},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.left.Compare(tc.right)
			if err != nil || got != tc.want {
				t.Fatalf("ReleaseVersion.Compare() = (%v, %v), want (%v, nil)", got, err, tc.want)
			}
		})
	}
	if got, err := (core.ReleaseVersion{}).Compare(core.NewReleaseVersion(1, 0, 0)); !errors.Is(err, core.ErrPrimitiveContract) ||
		got != core.ComparisonUnknown {
		t.Fatalf("zero ReleaseVersion.Compare() = (%v, %v), want (%v, %v)", got, err, core.ComparisonUnknown, core.ErrPrimitiveContract)
	}
}

func TestBuildIdentityAccessorsAreExactCompilerOwnedFacts(t *testing.T) {
	t.Parallel()

	identity := releaseIdentityFixture(t, core.OfferingWitness)
	if identity.Offering() != core.OfferingWitness ||
		identity.Version() != core.NewReleaseVersion(2026, 7, 30) ||
		identity.Commit().String() != "0123456789abcdef0123456789abcdef01234567" ||
		identity.Platform().String() != "linux-amd64" {
		t.Fatalf("BuildIdentity accessors do not return exact constructed facts: %v", identity)
	}
	var receiver *core.BuildIdentity
	if err := receiver.UnmarshalJSON([]byte(`{}`)); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil BuildIdentity.UnmarshalJSON() error = %v, want %v", err, core.ErrJSONContract)
	}
	var versionReceiver *core.ReleaseVersion
	if err := versionReceiver.UnmarshalJSON([]byte(`"1.0.0"`)); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil ReleaseVersion.UnmarshalJSON() error = %v, want %v", err, core.ErrJSONContract)
	}
	var commitReceiver *core.BuildCommit
	if err := commitReceiver.UnmarshalJSON([]byte(`"0123456789abcdef0123456789abcdef01234567"`)); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil BuildCommit.UnmarshalJSON() error = %v, want %v", err, core.ErrJSONContract)
	}
}

func releaseIdentityFixture(t *testing.T, offering core.Offering) core.BuildIdentity {
	t.Helper()
	commit, err := core.ParseBuildCommit("0123456789abcdef0123456789abcdef01234567")
	if err != nil {
		t.Fatalf("ParseBuildCommit() error = %v", err)
	}
	platform, err := core.NewPlatform(core.OperatingSystemLinux, core.CPUArchitectureAMD64)
	if err != nil {
		t.Fatalf("NewPlatform() error = %v", err)
	}
	identity, err := core.NewBuildIdentity(core.BuildIdentityRequest{
		Offering: offering,
		Version:  core.NewReleaseVersion(2026, 7, 30),
		Commit:   commit,
		Platform: platform,
	})
	if err != nil {
		t.Fatalf("NewBuildIdentity() error = %v", err)
	}
	return identity
}
