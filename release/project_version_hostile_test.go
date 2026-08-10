package release_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/release"
)

func TestPrimitiveVersionIsTheSingleTypedProjectAndGitAuthority(t *testing.T) {
	t.Parallel()

	current := release.PrimitiveVersion
	if err := current.Validate(); err != nil {
		t.Fatalf("release.PrimitiveVersion.Validate() error = %v, want nil", err)
	}
	parsed, err := release.ParseProjectVersion(string(current))
	if err != nil || parsed != current {
		t.Fatalf("release.ParseProjectVersion(current) = (%v, %v), want (%v, nil)", parsed, err, current)
	}
	tag := current.String()
	if tag == "" || tag != string(current) {
		t.Fatalf("release.PrimitiveVersion.String() = %q, want exact compiler constant %q", tag, current)
	}
	want := core.ReleaseVersion{}
	if err := want.UnmarshalText([]byte(tag[1:])); err != nil {
		t.Fatalf("core.ReleaseVersion.UnmarshalText(project tag) error = %v, want nil", err)
	}
	got, err := current.ReleaseVersion()
	if err != nil || got != want {
		t.Fatalf("release.PrimitiveVersion.ReleaseVersion() = (%v, %v), want (%v, nil)", got, err, want)
	}
}

func TestPrimitiveVersionHostileMutationMatrix(t *testing.T) {
	t.Parallel()

	current := string(release.PrimitiveVersion)
	prefix := current[:1]
	body := current[1:]
	lastSeparator := strings.LastIndexByte(current, '.')
	different := []byte(current)
	if different[len(different)-1] == '0' {
		different[len(different)-1] = '1'
	} else {
		different[len(different)-1] = '0'
	}
	cases := []struct {
		name  string
		value release.ProjectVersion
	}{
		{name: "zero value"},
		{name: "tag prefix only", value: release.ProjectVersion(prefix)},
		{name: "missing tag prefix", value: release.ProjectVersion(body)},
		{name: "uppercase tag prefix", value: release.ProjectVersion(strings.ToUpper(prefix) + body)},
		{name: "leading whitespace", value: release.ProjectVersion(" " + current)},
		{name: "trailing whitespace", value: release.ProjectVersion(current + " ")},
		{name: "leading zero component", value: release.ProjectVersion(prefix + "0" + body)},
		{name: "missing patch component", value: release.ProjectVersion(current[:lastSeparator])},
		{name: "extra component", value: release.ProjectVersion(current + ".0")},
		{name: "different canonical version", value: release.ProjectVersion(different)},
		{name: "pre-release suffix", value: release.ProjectVersion(current + "-rc1")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if gotErr := tc.value.Validate(); !errors.Is(gotErr, core.ErrReleaseContract) {
				t.Fatalf("ProjectVersion.Validate() error = %v, want errors.Is %v", gotErr, core.ErrReleaseContract)
			}
			if got, gotErr := release.ParseProjectVersion(string(tc.value)); !errors.Is(gotErr, core.ErrReleaseContract) || got != release.ProjectVersion("") {
				t.Fatalf("ParseProjectVersion() = (%v, %v), want zero and errors.Is %v", got, gotErr, core.ErrReleaseContract)
			}
			if got := tc.value.String(); got != "" {
				t.Fatalf("ProjectVersion.String() = %q, want empty refusal", got)
			}
			if got, gotErr := tc.value.ReleaseVersion(); !errors.Is(gotErr, core.ErrReleaseContract) || got != (core.ReleaseVersion{}) {
				t.Fatalf("ProjectVersion.ReleaseVersion() = (%v, %v), want zero and errors.Is %v",
					got, gotErr, core.ErrReleaseContract)
			}
		})
	}
}
