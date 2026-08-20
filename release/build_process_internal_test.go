package release

import (
	"errors"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestComposeExactSearchPathExhaustsDirectoryCombinations(t *testing.T) {
	t.Parallel()

	goDir := parseAbsolutePathForSearchTest(t, t.TempDir())
	gitDir := parseAbsolutePathForSearchTest(t, t.TempDir())
	cases := []struct {
		wantErr error
		name    string
		want    string
		in      []core.AbsolutePath
	}{
		{name: "one go directory is admitted", in: []core.AbsolutePath{goDir}, want: goDir.String()},
		{name: "identical go and git directories collapse", in: []core.AbsolutePath{goDir, goDir}, want: goDir.String()},
		{name: "distinct go then git directories keep order", in: []core.AbsolutePath{goDir, gitDir}, want: goDir.String() + string(os.PathListSeparator) + gitDir.String()},
		{name: "distinct git then go directories keep caller order", in: []core.AbsolutePath{gitDir, goDir}, want: gitDir.String() + string(os.PathListSeparator) + goDir.String()},
		{name: "three entries with a consecutive duplicate collapse the duplicate", in: []core.AbsolutePath{goDir, goDir, gitDir}, want: goDir.String() + string(os.PathListSeparator) + gitDir.String()},
		{name: "empty directory list is rejected", wantErr: core.ErrReleaseContract},
		{name: "nil directory list is rejected", in: nil, wantErr: core.ErrReleaseContract},
		{name: "zero first directory is rejected", in: []core.AbsolutePath{{}, gitDir}, wantErr: core.ErrReleaseContract},
		{name: "zero second directory is rejected", in: []core.AbsolutePath{goDir, {}}, wantErr: core.ErrReleaseContract},
		{name: "only a zero directory is rejected", in: []core.AbsolutePath{{}}, wantErr: core.ErrReleaseContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := composeExactSearchPath(tc.in)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("composeExactSearchPath() error = %v, want errors.Is %v", err, tc.wantErr)
				}
				if got != "" {
					t.Fatalf("composeExactSearchPath() = %q, want empty on refusal", got)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("composeExactSearchPath() = (%q, %v), want (%q, nil)", got, err, tc.want)
			}
		})
	}
}

func parseAbsolutePathForSearchTest(t *testing.T, value string) core.AbsolutePath {
	t.Helper()
	path, err := core.ParseAbsolutePath(value)
	if err != nil {
		t.Fatalf("ParseAbsolutePath(%q) error = %v, want nil", value, err)
	}
	return path
}
