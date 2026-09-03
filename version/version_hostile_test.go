package version_test

import (
	"errors"
	"math"
	"testing"

	json "encoding/json/v2"

	"github.com/deliri/primitive/v2026/compass"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"github.com/deliri/primitive/v2026/version"
)

func TestReleaseDerivesVersionTextTagAndOrderingFromCompass(t *testing.T) {
	t.Parallel()

	configuration, err := compass.Current()
	if err != nil {
		t.Fatalf("compass.Current() error = %v, want nil", err)
	}
	current, err := version.FromProject(configuration.Project)
	if err != nil {
		t.Fatalf("version.FromProject(Current.Project) error = %v, want nil", err)
	}
	if got, want := current.Version(), core.NewReleaseVersion(2026, 1, 4); got != want {
		t.Fatalf("Release.Version() = %v, want %v", got, want)
	}
	if got, want := current.String(), "2026.1.4"; got != want {
		t.Fatalf("Release.String() = %q, want %q", got, want)
	}
	if got, want := current.Tag().String(), "v2026.1.4"; got != want {
		t.Fatalf("Release.Tag().String() = %q, want %q", got, want)
	}
	next := releaseFromCoordinates(t, 2026, 1, 5)
	comparison, err := current.Compare(next)
	if err != nil || comparison != core.ComparisonLess {
		t.Fatalf("Release.Compare(next patch) = (%v, %v), want (%v, nil)", comparison, err, core.ComparisonLess)
	}
}

func TestReleaseRefusesInvalidCompassInsteadOfManufacturingIdentity(t *testing.T) {
	t.Parallel()

	got, gotErr := version.FromProject(compass.Project{})
	if !errors.Is(gotErr, core.ErrCompassContract) || !errors.Is(gotErr, core.ErrReleaseContract) {
		t.Fatalf("FromProject(zero) error = %v, want %v and %v", gotErr, core.ErrCompassContract, core.ErrReleaseContract)
	}
	if got != (version.Release{}) {
		t.Fatalf("FromProject(zero) = %v, want zero", got)
	}
}

func TestReleaseZeroValueCannotManufactureVersionOrTag(t *testing.T) {
	t.Parallel()

	var release version.Release
	if gotErr := release.Validate(); !errors.Is(gotErr, core.ErrReleaseContract) {
		t.Fatalf("Release{}.Validate() error = %v, want %v", gotErr, core.ErrReleaseContract)
	}
	if got := release.Version(); got != (core.ReleaseVersion{}) {
		t.Fatalf("Release{}.Version() = %v, want zero", got)
	}
	if got := release.String(); got != "" {
		t.Fatalf("Release{}.String() = %q, want empty", got)
	}
	if got := release.Tag(); got != (version.Tag{}) {
		t.Fatalf("Release{}.Tag() = %v, want zero", got)
	}
}

func TestParseTagHostileBoundaries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		text    string
		want    core.ReleaseVersion
		wantErr error
	}{
		{name: "zero coordinate is canonical external input", text: "v0.0.0", want: core.NewReleaseVersion(0, 0, 0)},
		{name: "ordinary calendar release is canonical", text: "v2026.1.3", want: core.NewReleaseVersion(2026, 1, 3)},
		{name: "maximum coordinate is canonical", text: "v4294967295.4294967295.4294967295", want: core.NewReleaseVersion(math.MaxUint32, math.MaxUint32, math.MaxUint32)},
		{name: "empty tag is refused", wantErr: core.ErrReleaseContract},
		{name: "prefix alone is refused", text: "v", wantErr: core.ErrReleaseContract},
		{name: "missing prefix is refused", text: "2026.1.3", wantErr: core.ErrReleaseContract},
		{name: "uppercase prefix is refused", text: "V2026.1.3", wantErr: core.ErrReleaseContract},
		{name: "leading zero is refused", text: "v02026.1.3", wantErr: core.ErrReleaseContract},
		{name: "missing patch is refused", text: "v2026.1", wantErr: core.ErrReleaseContract},
		{name: "extra component is refused", text: "v2026.1.3.0", wantErr: core.ErrReleaseContract},
		{name: "prerelease suffix is refused", text: "v2026.1.3-rc1", wantErr: core.ErrReleaseContract},
		{name: "leading whitespace is refused", text: " v2026.1.3", wantErr: core.ErrReleaseContract},
		{name: "trailing whitespace is refused", text: "v2026.1.3 ", wantErr: core.ErrReleaseContract},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := version.ParseTag(tc.text)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("ParseTag(%q) error = %v, want %v", tc.text, gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (version.Tag{}) {
					t.Fatalf("ParseTag(%q) = %v, want zero", tc.text, got)
				}
				return
			}
			if got.Release().Version() != tc.want || got.String() != tc.text {
				t.Fatalf("ParseTag(%q) = (%v, %q), want (%v, %q)", tc.text, got.Release().Version(), got.String(), tc.want, tc.text)
			}
		})
	}
}

func TestTagJSONPreservesReceiverOnTypedRefusal(t *testing.T) {
	t.Parallel()

	want := releaseFromCoordinates(t, 2026, 1, 3).Tag()
	encoded, err := json.Marshal(want)
	if err != nil || string(encoded) != `"v2026.1.3"` {
		t.Fatalf("json.Marshal(Tag) = (%s, %v), want (%q, nil)", encoded, err, `"v2026.1.3"`)
	}
	got := want
	gotErr := json.Unmarshal([]byte(`"v2026.1.3-shadow"`), &got)
	if !errors.Is(gotErr, core.ErrJSONContract) || !errors.Is(gotErr, core.ErrReleaseContract) || got != want {
		t.Fatalf("json.Unmarshal(hostile Tag) = (%v, %v), want preserved with %v and %v", got, gotErr, core.ErrJSONContract, core.ErrReleaseContract)
	}
}

func FuzzTagSemanticClosure(f *testing.F) {
	f.Add("v2026.1.3")
	f.Add("")
	f.Add("2026.1.3")
	f.Add("v2026.1.3-shadow")
	f.Fuzz(func(t *testing.T, text string) {
		got, gotErr := version.ParseTag(text)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrReleaseContract) || got != (version.Tag{}) {
				t.Fatalf("ParseTag(%q) = (%v, %v), want zero typed refusal", text, got, gotErr)
			}
			return
		}
		if got.Validate() != nil || got.String() != text || got.Release().Tag() != got {
			t.Fatalf("ParseTag(%q) accepted noncanonical or unstable tag %v", text, got)
		}
	})
}

func releaseFromCoordinates(t testing.TB, major, minor, patch uint32) version.Release {
	t.Helper()
	name, err := compass.ParseProjectName("Test Project")
	if err != nil {
		t.Fatalf("ParseProjectName(test fixture) error = %v, want nil", err)
	}
	module, err := gomodule.ParsePath("example.com/test/project")
	if err != nil {
		t.Fatalf("gomodule.ParsePath(test fixture) error = %v, want nil", err)
	}
	repository, err := core.NewRepositoryIdentity("example/test-project")
	if err != nil {
		t.Fatalf("NewRepositoryIdentity(test fixture) error = %v, want nil", err)
	}
	got, err := version.FromProject(compass.Project{
		Name: name, Module: module, Repository: repository,
		Release: compass.ReleaseCoordinates{Major: major, Minor: minor, Patch: patch},
	})
	if err != nil {
		t.Fatalf("FromProject(test fixture) error = %v, want nil", err)
	}
	return got
}
