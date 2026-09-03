package projectversion_test

import (
	"errors"
	"math"
	"testing"

	json "encoding/json/v2"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/projectversion"
)

func TestReleaseOwnsVersionTextTagAndOrdering(t *testing.T) {
	t.Parallel()

	current := projectversion.New(2026, 1, 2)
	if gotErr := current.Validate(); gotErr != nil {
		t.Fatalf("Release.Validate() error = %v, want nil", gotErr)
	}
	if got, want := current.Version(), core.NewReleaseVersion(2026, 1, 2); got != want {
		t.Fatalf("Release.Version() = %v, want %v", got, want)
	}
	if got, want := current.String(), "2026.1.2"; got != want {
		t.Fatalf("Release.String() = %q, want %q", got, want)
	}
	if got, want := current.Tag().String(), "v2026.1.2"; got != want {
		t.Fatalf("Release.Tag().String() = %q, want %q", got, want)
	}
	comparison, err := current.Compare(projectversion.New(2026, 1, 3))
	if err != nil || comparison != core.ComparisonLess {
		t.Fatalf("Release.Compare(next patch) = (%v, %v), want (%v, nil)", comparison, err, core.ComparisonLess)
	}
}

func TestReleaseZeroValueCannotManufactureVersionOrTag(t *testing.T) {
	t.Parallel()

	var release projectversion.Release
	if gotErr := release.Validate(); !errors.Is(gotErr, core.ErrReleaseContract) {
		t.Fatalf("Release{}.Validate() error = %v, want errors.Is %v", gotErr, core.ErrReleaseContract)
	}
	if got := release.Version(); got != (core.ReleaseVersion{}) {
		t.Fatalf("Release{}.Version() = %v, want zero", got)
	}
	if got := release.String(); got != "" {
		t.Fatalf("Release{}.String() = %q, want empty", got)
	}
	if got := release.Tag(); got != (projectversion.Tag{}) {
		t.Fatalf("Release{}.Tag() = %v, want zero", got)
	}
}

func TestParseTagHostileBoundaries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		text    string
		want    projectversion.Release
		wantErr error
	}{
		{name: "zero coordinate is canonical", text: "v0.0.0", want: projectversion.New(0, 0, 0)},
		{name: "ordinary calendar release is canonical", text: "v2026.1.2", want: projectversion.New(2026, 1, 2)},
		{name: "maximum coordinate is canonical", text: "v4294967295.4294967295.4294967295", want: projectversion.New(math.MaxUint32, math.MaxUint32, math.MaxUint32)},
		{name: "empty tag is refused", wantErr: core.ErrReleaseContract},
		{name: "prefix alone is refused", text: "v", wantErr: core.ErrReleaseContract},
		{name: "missing prefix is refused", text: "2026.1.2", wantErr: core.ErrReleaseContract},
		{name: "uppercase prefix is refused", text: "V2026.1.2", wantErr: core.ErrReleaseContract},
		{name: "leading zero is refused", text: "v02026.1.2", wantErr: core.ErrReleaseContract},
		{name: "missing patch is refused", text: "v2026.1", wantErr: core.ErrReleaseContract},
		{name: "extra component is refused", text: "v2026.1.2.0", wantErr: core.ErrReleaseContract},
		{name: "prerelease suffix is refused", text: "v2026.1.2-rc1", wantErr: core.ErrReleaseContract},
		{name: "leading whitespace is refused", text: " v2026.1.2", wantErr: core.ErrReleaseContract},
		{name: "trailing whitespace is refused", text: "v2026.1.2 ", wantErr: core.ErrReleaseContract},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := projectversion.ParseTag(tc.text)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("ParseTag(%q) error = %v, want errors.Is %v", tc.text, gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (projectversion.Tag{}) {
					t.Fatalf("ParseTag(%q) = %v, want zero", tc.text, got)
				}
				return
			}
			if got.Release() != tc.want || got.String() != tc.text {
				t.Fatalf("ParseTag(%q) = (%v, %q), want (%v, %q)", tc.text, got.Release(), got.String(), tc.want, tc.text)
			}
		})
	}
}

func TestTagJSONPreservesReceiverOnTypedRefusal(t *testing.T) {
	t.Parallel()

	want := projectversion.New(2026, 1, 2).Tag()
	encoded, err := json.Marshal(want)
	if err != nil || string(encoded) != `"v2026.1.2"` {
		t.Fatalf("json.Marshal(Tag) = (%s, %v), want (%q, nil)", encoded, err, `"v2026.1.2"`)
	}
	got := want
	gotErr := json.Unmarshal([]byte(`"v2026.1.2-shadow"`), &got)
	if !errors.Is(gotErr, core.ErrJSONContract) || !errors.Is(gotErr, core.ErrReleaseContract) || got != want {
		t.Fatalf("json.Unmarshal(hostile Tag) = (%v, %v), want preserved with %v and %v", got, gotErr, core.ErrJSONContract, core.ErrReleaseContract)
	}
}

func FuzzTagSemanticClosure(f *testing.F) {
	f.Add("v2026.1.2")
	f.Add("")
	f.Add("2026.1.2")
	f.Add("v2026.1.2-shadow")
	f.Fuzz(func(t *testing.T, text string) {
		got, gotErr := projectversion.ParseTag(text)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrReleaseContract) || got != (projectversion.Tag{}) {
				t.Fatalf("ParseTag(%q) = (%v, %v), want zero typed refusal", text, got, gotErr)
			}
			return
		}
		if got.Validate() != nil || got.String() != text || got.Release().Tag() != got {
			t.Fatalf("ParseTag(%q) accepted noncanonical or unstable tag %v", text, got)
		}
	})
}
