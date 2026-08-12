package core_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func absolutePathForTest(t *testing.T, value string) core.AbsolutePath {
	t.Helper()
	path, err := core.ParseAbsolutePath(value)
	if err != nil {
		t.Fatalf("ParseAbsolutePath(%q) error = %v, want nil", value, err)
	}
	return path
}

// TestRelativeToRefusesEveryPathThatEscapesItsBase is the case that matters.
// A caller asks for the path under a root so it can hand it to a rooted
// capability; one that climbs out is not a weaker answer, it is a way to reach
// files the root exists to exclude.
func TestRelativeToRefusesEveryPathThatEscapesItsBase(t *testing.T) {
	t.Parallel()

	root := string(filepath.Separator)
	cases := []struct {
		name    string
		path    string
		base    string
		want    string
		wantErr error
	}{
		{name: "direct child", path: root + "a/b", base: root + "a", want: "b"},
		{name: "nested descendant", path: root + "a/b/c/d", base: root + "a", want: "b/c/d"},
		{name: "path equals its base", path: root + "a", base: root + "a", want: "."},
		{name: "child of the filesystem root", path: root + "a", base: root, want: "a"},
		{name: "sibling escapes", path: root + "a/b", base: root + "a/c", wantErr: core.ErrPrimitiveContract},
		{name: "parent escapes", path: root + "a", base: root + "a/b", wantErr: core.ErrPrimitiveContract},
		{name: "grandparent escapes", path: root + "a", base: root + "a/b/c", wantErr: core.ErrPrimitiveContract},
		{name: "unrelated subtree escapes", path: root + "x/y", base: root + "a/b", wantErr: core.ErrPrimitiveContract},
		{name: "base whose name prefixes the path is not an ancestor", path: root + "ab/c", base: root + "a", wantErr: core.ErrPrimitiveContract},
		{name: "path whose name prefixes the base is not a descendant", path: root + "a", base: root + "ab", wantErr: core.ErrPrimitiveContract},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := absolutePathForTest(t, tc.path).RelativeTo(absolutePathForTest(t, tc.base))
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("RelativeTo(%q, %q) error = %v, want %v", tc.path, tc.base, err, tc.wantErr)
				}
				if got != (core.RelativePath{}) {
					t.Fatalf("RelativeTo() = %q alongside a refusal, want the zero path", got.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("RelativeTo(%q, %q) error = %v, want nil", tc.path, tc.base, err)
			}
			if got.String() != tc.want {
				t.Fatalf("RelativeTo(%q, %q) = %q, want %q", tc.path, tc.base, got.String(), tc.want)
			}
		})
	}
}

// TestRelativeToRoundTripsThroughJoinRelative proves the two halves are
// inverses. A product that measures a path against a root and then resolves it
// back must arrive where it started, or one of the two is lying about the
// tree's shape.
func TestRelativeToRoundTripsThroughJoinRelative(t *testing.T) {
	t.Parallel()

	root := string(filepath.Separator)
	for _, suffix := range []string{"a", "a/b", "a/b/c", "a/b/c/d/e"} {
		base := absolutePathForTest(t, root+"base")
		path := absolutePathForTest(t, filepath.Join(root+"base", suffix))

		relative, err := path.RelativeTo(base)
		if err != nil {
			t.Fatalf("RelativeTo(%q) error = %v, want nil", suffix, err)
		}
		restored, err := base.JoinRelative(relative)
		if err != nil {
			t.Fatalf("JoinRelative(%q) error = %v, want nil", relative.String(), err)
		}
		if restored != path {
			t.Fatalf("round trip of %q = %q, want %q", suffix, restored.String(), path.String())
		}
	}
}

// TestRelativeToRefusesAnUnvalidatedPath proves both sides are gates and not
// decoration.
func TestRelativeToRefusesAnUnvalidatedPath(t *testing.T) {
	t.Parallel()

	real := absolutePathForTest(t, string(filepath.Separator)+"a")
	if got, err := (core.AbsolutePath{}).RelativeTo(real); !errors.Is(err, core.ErrPrimitiveContract) || got != (core.RelativePath{}) {
		t.Fatalf("RelativeTo() on a zero path = (%v, %v), want zero and errors.Is %v", got, err, core.ErrPrimitiveContract)
	}
	if got, err := real.RelativeTo(core.AbsolutePath{}); !errors.Is(err, core.ErrPrimitiveContract) || got != (core.RelativePath{}) {
		t.Fatalf("RelativeTo(zero base) = (%v, %v), want zero and errors.Is %v", got, err, core.ErrPrimitiveContract)
	}
	if _, err := real.RelativeTo(real); err != nil && !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("RelativeTo(self) error = %v, want nil or a typed refusal", err)
	}
}
