package core

import (
	"errors"
	"path/filepath"
	"testing"
)

// FuzzResolveMatchesPerComponentJoin holds the single-allocation Resolve equal
// to admitting and joining each name in turn, for the filesystem root, a
// nested absolute base, the relative root and a nested relative base, and for
// refused names at every position.
func FuzzResolveMatchesPerComponentJoin(f *testing.F) {
	for _, seed := range [][3]string{
		{"a", "b", "c"},
		{"objects", "ab", "entry-0001.json"},
		{"é", " space ", "line\nfeed"},
		{"a", "..", "c"},
		{"a", ".", "c"},
		{"a", "b/c", "d"},
		{"", "b", "c"},
		{"a", "b", "nul\x00"},
		{"a", "b", "\xff"},
	} {
		f.Add(seed[0], seed[1], seed[2])
	}
	root := string(filepath.Separator)
	absoluteBases := []AbsolutePath{
		{value: root},
		{value: filepath.Join(root, "srv", "primitive")},
	}
	relativeBases := []RelativePath{
		{value: "."},
		{value: filepath.Join("stable", "child")},
	}
	f.Fuzz(func(t *testing.T, first, second, third string) {
		names := []string{first, second, third}
		for _, base := range absoluteBases {
			want, wantErr := base, error(nil)
			for _, name := range names {
				component, err := ParsePathComponent(name)
				if err != nil {
					want, wantErr = AbsolutePath{}, err
					break
				}
				if want, wantErr = want.Join(component); wantErr != nil {
					break
				}
			}
			got, gotErr := base.Resolve(names...)
			if got != want || (gotErr == nil) != (wantErr == nil) ||
				gotErr != nil && (!errors.Is(gotErr, ErrPrimitiveContract) || gotErr.Error() != wantErr.Error()) {
				t.Fatalf("%q.Resolve(%q) = %q, %v; want %q, %v", base, names, got, gotErr, want, wantErr)
			}
		}
		for _, base := range relativeBases {
			want, wantErr := base, error(nil)
			for _, name := range names {
				component, err := ParsePathComponent(name)
				if err != nil {
					want, wantErr = RelativePath{}, err
					break
				}
				if want, wantErr = want.Join(component); wantErr != nil {
					break
				}
			}
			got, gotErr := base.Resolve(names...)
			if got != want || (gotErr == nil) != (wantErr == nil) ||
				gotErr != nil && (!errors.Is(gotErr, ErrPrimitiveContract) || gotErr.Error() != wantErr.Error()) {
				t.Fatalf("%q.Resolve(%q) = %q, %v; want %q, %v", base, names, got, gotErr, want, wantErr)
			}
		}
	})
}
