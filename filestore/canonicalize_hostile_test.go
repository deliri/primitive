package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestCanonicalizeGoResolutionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, path, wantRelative string
		ingress                  symbolicObservationIngress
		wantErr, wantCause       error
	}{
		{name: "real binary file remains an existing absolute observation", path: "file", wantRelative: "root/file"},
		{name: "directory target remains directory without invented file rule", path: "directory", wantRelative: "root/directory"},
		{name: "final relative link is resolved", path: "link", wantRelative: "root/file"},
		{name: "all hops of a final chain are resolved", path: "chain", wantRelative: "root/file"},
		{name: "ancestor link and final child link both resolve", path: "parent/child-link", wantRelative: "root/directory/entry"},
		{name: "absolute outside link is deliberately followed", path: "absolute", wantRelative: "outside/entry"},
		{name: "outside ancestor is followed without inventing root confinement", path: "outside/child-link", wantRelative: "outside/entry"},
		{name: "dot components in stored target are resolved", path: "lexical", wantRelative: "root/file"},
		{name: "dangling final link preserves native absence", path: "dangling", wantErr: core.ErrFilestoreSource, wantCause: os.ErrNotExist},
		{name: "missing path is not lexical success", path: "missing", wantErr: core.ErrFilestoreSource, wantCause: os.ErrNotExist},
		{name: "regular file ancestor preserves native non-directory refusal", path: "file/child", wantErr: core.ErrFilestoreSource},
		{name: "self cycle cannot produce a canonical path", path: "self", wantErr: core.ErrFilestoreSource},
		{name: "multi-name cycle cannot produce a canonical path", path: "first", wantErr: core.ErrFilestoreSource},
		{name: "zero path cannot turn into working directory", path: "file", ingress: symbolicObservationZeroPath, wantErr: core.ErrFilestoreContract},
		{name: "nil context prevents native resolution", path: "file", ingress: symbolicObservationNilContext, wantErr: core.ErrNilContext},
		{name: "canceled context cannot resolve an existing chain", path: "chain", ingress: symbolicObservationCanceled, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			container := t.TempDir()
			if err := createSymbolicLinkNativeFixture(container); err != nil {
				t.Fatal(err)
			}
			path, err := core.ParseAbsolutePath(filepath.Join(container, "root", filepath.FromSlash(tc.path)))
			if err != nil {
				t.Fatal(err)
			}
			ctx := t.Context()
			switch tc.ingress {
			case symbolicObservationAdmitted:
			case symbolicObservationZeroPath:
				path = core.AbsolutePath{}
			case symbolicObservationNilContext:
				ctx = nil
			case symbolicObservationCanceled:
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			default:
				t.Fatalf("ingress = %v, want applicable fixture", tc.ingress)
			}
			before, err := removalFixtureSnapshot(container)
			if err != nil {
				t.Fatal(err)
			}
			metadata := make([]fs.FileInfo, len(before))
			for i, entry := range before {
				metadata[i], err = os.Lstat(filepath.Join(container, entry.name))
				if err != nil {
					t.Fatal(err)
				}
			}
			native, nativeErr := filepath.EvalSymlinks(path.String())
			got, gotErr := filestore.Canonicalize(ctx, path)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Canonicalize() error = %v, want %v", gotErr, tc.wantErr)
			}
			if !errors.Is(tc.wantErr, core.ErrFilestoreSource) && errors.Is(gotErr, core.ErrFilestoreSource) {
				t.Fatalf("ingress refusal = %v, want no source classification", gotErr)
			}
			if tc.wantCause != nil && !errors.Is(gotErr, tc.wantCause) {
				t.Fatalf("native cause = %v, want %v", gotErr, tc.wantCause)
			}
			if errors.Is(tc.wantErr, core.ErrFilestoreSource) {
				if nativeErr == nil {
					t.Fatalf("native resolution = %q, want refusal", native)
				}
				// witness:waiver test/errors -- filepath.EvalSymlinks exposes its link-count refusal as a fresh untyped error; Filestore's Source identity is the stable boundary.
				var nativePath, gotPath *os.PathError
				if errors.As(nativeErr, &nativePath) && (!errors.As(gotErr, &gotPath) || !errors.Is(gotErr, nativePath.Err) || gotPath.Op != nativePath.Op || gotPath.Path != nativePath.Path) {
					t.Fatalf("native refusal = (%v,%v), want exact Go PathError", gotErr, nativeErr)
				}
				var nativeErrno syscall.Errno
				if errors.As(nativeErr, &nativeErrno) && !errors.Is(gotErr, nativeErrno) {
					t.Fatalf("native errno = %v, want %v", gotErr, nativeErrno)
				}
			}
			if tc.wantErr == nil {
				containerCanonical, err := filepath.EvalSymlinks(container)
				if err != nil {
					t.Fatal(err)
				}
				want := filepath.Join(containerCanonical, filepath.FromSlash(tc.wantRelative))
				if nativeErr != nil || native != want || got.String() != want || got.Validate() != nil {
					t.Fatalf("canonical path = (%q,%v), native = (%q,%v), want %q", got.String(), gotErr, native, nativeErr, want)
				}
				sourceInfo, err := os.Stat(path.String())
				if err != nil {
					t.Fatal(err)
				}
				gotInfo, err := os.Stat(got.String())
				if err != nil || !os.SameFile(sourceInfo, gotInfo) {
					t.Fatalf("resolved identity = (%v,%v), want %v", gotInfo, err, sourceInfo)
				}
				again, err := filestore.Canonicalize(ctx, got)
				if err != nil || again != got {
					t.Fatalf("second resolution = (%v,%v), want %v", again, err, got)
				}
			} else if got != (core.AbsolutePath{}) {
				t.Fatalf("refused canonical path = %v, want zero", got)
			}
			after, err := removalFixtureSnapshot(container)
			if err != nil || len(after) != len(before) {
				t.Fatalf("namespace = (%v,%v), want %v", after, err, before)
			}
			for i, want := range before {
				got := after[i]
				if got.name != want.name || got.mode != want.mode || got.target != want.target || !bytes.Equal(got.data, want.data) {
					t.Fatalf("namespace entry = %+v, want %+v", got, want)
				}
				info, err := os.Lstat(filepath.Join(container, want.name))
				if err != nil || !os.SameFile(metadata[i], info) || metadata[i].ModTime().UnixNano() != info.ModTime().UnixNano() {
					t.Fatalf("retained inode/time = (%v,%v), want %v", info, err, metadata[i])
				}
			}
		})
	}
}
