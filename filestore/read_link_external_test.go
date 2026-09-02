package filestore_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestReadSymbolicLinkEffectLayerTriad(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		wantErr      error
		name         string
		target       string
		plant        bool
		zeroLocation bool
	}{
		{name: "positive exact opaque target is observed without following", target: "socket:[123]", plant: true},
		{name: "negative absent link preserves source refusal", wantErr: core.ErrFilestoreSource},
		{name: "neutral zero location is refused before observation", zeroLocation: true, wantErr: core.ErrFilestoreContract},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()
			if testCase.plant {
				if err := os.Symlink(testCase.target, filepath.Join(directory, "subject")); err != nil {
					t.Fatalf("os.Symlink() setup error = %v, want nil", err)
				}
			}
			var location filestore.Location
			if !testCase.zeroLocation {
				root, err := filestore.OpenRoot(t.Context(), openRootAbsolute(t, directory))
				if err != nil {
					t.Fatalf("filestore.OpenRoot() setup error = %v, want nil", err)
				}
				defer func() {
					if closeErr := root.Close(); closeErr != nil {
						t.Errorf("root.Close() error = %v, want nil", closeErr)
					}
				}()
				path, err := core.ParseRelativePath("subject")
				if err != nil {
					t.Fatalf("core.ParseRelativePath() setup error = %v, want nil", err)
				}
				location = filestore.Location{Root: root, Path: path}
			}
			got, gotErr := filestore.ReadSymbolicLink(t.Context(), location)
			if !errors.Is(gotErr, testCase.wantErr) {
				t.Fatalf("ReadSymbolicLink() error = %v, want %v", gotErr, testCase.wantErr)
			}
			if testCase.wantErr != nil {
				if got != (filestore.SymbolicLinkTarget{}) {
					t.Fatalf("ReadSymbolicLink(rejected) = %q, want zero", got.String())
				}
				return
			}
			if got.String() != testCase.target || got.Validate() != nil {
				t.Fatalf("ReadSymbolicLink() = %q validation:%v, want %q and nil", got.String(), got.Validate(), testCase.target)
			}
		})
	}
}
