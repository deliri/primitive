package core_test

import (
	jsontext "encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

const offeringFixtureToken = "core-fixture"

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
			var got core.ReleaseVersion
			err := json.Unmarshal([]byte(strconv.Quote(tc.text)), &got)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("json.Unmarshal(ReleaseVersion %q) error = %v, want %v", tc.text, err, tc.wantErr)
			}
			if err == nil && got != tc.want {
				t.Fatalf("json.Unmarshal(ReleaseVersion %q) = %v, want %v", tc.text, got, tc.want)
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

	identity := releaseIdentityFixture(t, offeringFixture(t))
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

	hostile := []struct {
		name       string
		data       []byte
		wantNative bool
	}{
		{name: "null document reaches the typed contract", data: []byte(`null`)},
		{name: "missing fields reach the typed contract", data: []byte(`{}`)},
		{name: "duplicate member stops at JSON v2 syntax", data: []byte(`{"offering":"core-fixture","offering":"core-fixture","version":"2026.7.30","commit":"0123456789abcdef0123456789abcdef01234567","platform":"linux-amd64"}`), wantNative: true},
		{name: "unknown member reaches the typed contract", data: []byte(`{"offering":"core-fixture","version":"2026.7.30","commit":"0123456789abcdef0123456789abcdef01234567","platform":"linux-amd64","future":true}`)},
	}
	for _, tc := range hostile {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var receiver core.BuildIdentity
			err := json.Unmarshal(tc.data, &receiver)
			if tc.wantNative {
				if _, ok := errors.AsType[*jsontext.SyntacticError](err); !ok {
					t.Fatalf("json.Unmarshal(%q) error = %v, want JSON v2 syntactic error", tc.data, err)
				}
			} else if !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("json.Unmarshal(%q) error = %v, want %v", tc.data, err, core.ErrJSONContract)
			}
			if receiver != (core.BuildIdentity{}) {
				t.Fatalf("json.Unmarshal(%q) mutated receiver to %v", tc.data, receiver)
			}
		})
	}
}

func TestOfferingHostileCanonicalDomain(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		text   string
		wantOK bool
	}{
		{name: "minimum one-letter identity", text: "a", wantOK: true},
		{name: "last lowercase letter identity", text: "z", wantOK: true},
		{name: "single trailing digit", text: "a0", wantOK: true},
		{name: "one internal separator", text: "a-b", wantOK: true},
		{name: "letters and digits in segments", text: "a0-b9", wantOK: true},
		{name: "several canonical segments", text: "a-b-c-d", wantOK: true},
		{name: "ordinary product-neutral identity", text: offeringFixtureToken, wantOK: true},
		{name: "calendar suffix identity", text: "product-2026", wantOK: true},
		{name: "long canonical segment", text: strings.Repeat("a", 24), wantOK: true},
		{name: "long segmented identity", text: "product-alpha-2026-release", wantOK: true},
		{name: "empty identity", text: ""},
		{name: "obviously oversized identity", text: strings.Repeat("a", 1<<10)},
		{name: "leading digit", text: "1product"},
		{name: "leading separator", text: "-product"},
		{name: "trailing separator", text: "product-"},
		{name: "adjacent separators", text: "product--alpha"},
		{name: "uppercase first byte", text: "Product"},
		{name: "uppercase interior byte", text: "proDuct"},
		{name: "underscore separator", text: "product_alpha"},
		{name: "slash separator", text: "product/alpha"},
		{name: "dot separator", text: "product.alpha"},
		{name: "colon separator", text: "product:alpha"},
		{name: "leading space", text: " product"},
		{name: "trailing space", text: "product "},
		{name: "interior tab", text: "product\talpha"},
		{name: "interior newline", text: "product\nalpha"},
		{name: "NUL byte", text: "product\x00alpha"},
		{name: "non-ASCII letter", text: "prøduct"},
		{name: "invalid UTF-8 byte", text: string([]byte{0xff})},
		{name: "oversized segmented identity", text: strings.Repeat("a", 1<<10) + "-a"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := core.Offering{Token: tc.text}
			gotErr := got.Validate()
			if !tc.wantOK {
				if !errors.Is(gotErr, core.ErrPrimitiveContract) {
					t.Fatalf("Offering{%q}.Validate() error = %v, want %v", tc.text, gotErr, core.ErrPrimitiveContract)
				}
				receiver := offeringFixture(t)
				if err := receiver.UnmarshalText([]byte(tc.text)); !errors.Is(err, core.ErrPrimitiveContract) || receiver != offeringFixture(t) {
					t.Fatalf("Offering.UnmarshalText(%q) = (%v, %v), want preserved and %v", tc.text, receiver, err, core.ErrPrimitiveContract)
				}
				return
			}
			if gotErr != nil || got.String() != tc.text {
				t.Fatalf("Offering{%q}.Validate() = %v, want exact valid identity", tc.text, gotErr)
			}
			encoded, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("json.Marshal(Offering %q) error = %v, want nil", tc.text, err)
			}
			var roundTrip core.Offering
			if err := json.Unmarshal(encoded, &roundTrip); err != nil || roundTrip != got {
				t.Fatalf("Offering %q JSON round trip = (%v, %v), want (%v, nil)", tc.text, roundTrip, err, got)
			}
		})
	}

	var receiver *core.Offering
	if err := receiver.UnmarshalJSON([]byte(`"core-fixture"`)); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil Offering.UnmarshalJSON() error = %v, want %v", err, core.ErrJSONContract)
	}
	if _, err := json.Marshal(core.Offering{}); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("json.Marshal(zero Offering) error = %v, want %v", err, core.ErrJSONContract)
	}
}

func TestOfferingTraversesBuildIdentityWithoutProductInterpretation(t *testing.T) {
	t.Parallel()

	offering := offeringFixture(t)
	identity := releaseIdentityFixture(t, offering)
	encoded, err := json.Marshal(identity)
	if err != nil {
		t.Fatalf("json.Marshal(BuildIdentity) error = %v", err)
	}
	var roundTrip core.BuildIdentity
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatalf("json.Unmarshal(BuildIdentity) error = %v", err)
	}
	if roundTrip != identity || roundTrip.Offering() != offering {
		t.Fatalf("BuildIdentity round trip = %v, want %v", roundTrip, identity)
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

	offering := offeringFixture(t)
	identity := releaseIdentityFixture(t, offering)
	if identity.Offering() != offering ||
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

func TestReleaseIdentityTextDecodersBoundInputAndPreserveReceivers(t *testing.T) {
	t.Parallel()

	offering := offeringFixture(t)
	if gotErr := offering.UnmarshalText([]byte(strings.Repeat("x", 1<<20))); !errors.Is(gotErr, core.ErrPrimitiveContract) || offering != offeringFixture(t) {
		t.Fatalf("Offering.UnmarshalText(oversized) = (%v, %v), want preserved %v and %v", offering, gotErr, offeringFixture(t), core.ErrPrimitiveContract)
	}
	version := core.NewReleaseVersion(2026, 8, 2)
	if gotErr := version.UnmarshalText([]byte(strings.Repeat("9", 1<<20))); !errors.Is(gotErr, core.ErrPrimitiveContract) || version != core.NewReleaseVersion(2026, 8, 2) {
		t.Fatalf("ReleaseVersion.UnmarshalText(oversized) = (%v, %v), want preserved receiver and %v", version, gotErr, core.ErrPrimitiveContract)
	}
	if gotErr := (*core.Offering)(nil).UnmarshalText(nil); !errors.Is(gotErr, core.ErrPrimitiveContract) {
		t.Fatalf("nil Offering.UnmarshalText() error = %v, want %v", gotErr, core.ErrPrimitiveContract)
	}
	if gotErr := (*core.ReleaseVersion)(nil).UnmarshalText(nil); !errors.Is(gotErr, core.ErrPrimitiveContract) {
		t.Fatalf("nil ReleaseVersion.UnmarshalText() error = %v, want %v", gotErr, core.ErrPrimitiveContract)
	}
}

func releaseIdentityFixture(t *testing.T, offering core.Offering) core.BuildIdentity {
	t.Helper()
	commit, err := core.ParseBuildCommit("0123456789abcdef0123456789abcdef01234567")
	if err != nil {
		t.Fatalf("ParseBuildCommit() error = %v", err)
	}
	platform := core.Platform{
		OperatingSystem: core.OperatingSystemLinux,
		Architecture:    core.CPUArchitectureAMD64,
	}
	if err := platform.Validate(); err != nil {
		t.Fatalf("Platform.Validate() error = %v", err)
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

func offeringFixture(t testing.TB) core.Offering {
	t.Helper()
	offering := core.Offering{Token: offeringFixtureToken}
	if err := offering.Validate(); err != nil {
		t.Fatalf("Offering{%q}.Validate() error = %v, want nil", offeringFixtureToken, err)
	}
	return offering
}
