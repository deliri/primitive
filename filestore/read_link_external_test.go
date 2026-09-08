package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

type symbolicObservationIngress uint8

const (
	symbolicObservationAdmitted symbolicObservationIngress = iota
	symbolicObservationNilRoot
	symbolicObservationZeroPath
	symbolicObservationClosedRoot
	symbolicObservationNilContext
	symbolicObservationCanceled
)

func TestReadSymbolicLinkNativeObservationLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, path, wantTarget string
		ingress                symbolicObservationIngress
		wantErr, wantCause     error
		absoluteTarget         bool
	}{
		{name: "relative target is observed without reading binary referent", path: "link", wantTarget: "file"},
		{name: "chain returns first hop rather than resolved target", path: "chain", wantTarget: "link"},
		{name: "absolute outside target is opaque and never followed", path: "absolute", absoluteTarget: true},
		{name: "dangling link remains a present observation", path: "dangling", wantTarget: "missing"},
		{name: "self cycle is observed without attempting resolution", path: "self", wantTarget: "self"},
		{name: "nonpath native target is admitted", path: "opaque", wantTarget: "socket:[123]"},
		{name: "dot components are preserved rather than cleaned", path: "lexical", wantTarget: "./directory/../file"},
		{name: "directory referent does not change target observation", path: "directory-link", wantTarget: "directory"},
		{name: "confined ancestor link resolves before observing final target", path: "parent/child-link", wantTarget: "entry"},
		{name: "escaping ancestor cannot grant outside observation", path: "outside/child-link", wantErr: core.ErrFilestoreSource},
		{name: "regular file is not interpreted as link text", path: "file", wantErr: core.ErrFilestoreSource},
		{name: "directory is not an empty link target", path: "directory", wantErr: core.ErrFilestoreSource},
		{name: "missing final name remains native absence", path: "missing", wantErr: core.ErrFilestoreSource, wantCause: os.ErrNotExist},
		{name: "non-directory ancestor preserves native refusal", path: "file/child", wantErr: core.ErrFilestoreSource},
		{name: "cycle in ancestor cannot escape as a target", path: "self/child", wantErr: core.ErrFilestoreSource},
		{name: "nil root is contract refusal before source observation", path: "link", ingress: symbolicObservationNilRoot, wantErr: core.ErrFilestoreContract},
		{name: "zero path is contract refusal before source observation", path: "link", ingress: symbolicObservationZeroPath, wantErr: core.ErrFilestoreContract},
		{name: "closed capability retains native closed identity", path: "link", ingress: symbolicObservationClosedRoot, wantErr: core.ErrFilestoreSource, wantCause: os.ErrClosed},
		{name: "nil context refuses before a resolvable link", path: "link", ingress: symbolicObservationNilContext, wantErr: core.ErrNilContext},
		{name: "canceled context cannot return stored target", path: "link", ingress: symbolicObservationCanceled, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			container := t.TempDir()
			if err := createSymbolicLinkNativeFixture(container); err != nil {
				t.Fatal(err)
			}
			directory := filepath.Join(container, "root")
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			if tc.ingress == symbolicObservationClosedRoot {
				if err := root.Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				t.Cleanup(func() {
					if err := root.Close(); err != nil {
						t.Error(err)
					}
				})
			}
			path, err := core.ParseRelativePath(filepath.FromSlash(tc.path))
			if err != nil {
				t.Fatal(err)
			}
			location := filestore.Location{Root: root, Path: path}
			ctx := t.Context()
			switch tc.ingress {
			case symbolicObservationAdmitted, symbolicObservationClosedRoot:
			case symbolicObservationNilRoot:
				location.Root = nil
			case symbolicObservationZeroPath:
				location.Path = core.RelativePath{}
			case symbolicObservationNilContext:
				ctx = nil
			case symbolicObservationCanceled:
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			default:
				t.Fatalf("ingress = %v, want declared fixture", tc.ingress)
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
			nativeTarget, nativeErr := root.Readlink(path.String())
			got, gotErr := filestore.ReadSymbolicLink(ctx, location)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("ReadSymbolicLink() error = %v, want %v", gotErr, tc.wantErr)
			}
			if !errors.Is(tc.wantErr, core.ErrFilestoreSource) && errors.Is(gotErr, core.ErrFilestoreSource) {
				t.Fatalf("ingress refusal = %v, want no source classification", gotErr)
			}
			if tc.wantCause != nil && !errors.Is(gotErr, tc.wantCause) {
				t.Fatalf("native cause = %v, want %v", gotErr, tc.wantCause)
			}
			if errors.Is(tc.wantErr, core.ErrFilestoreSource) {
				var nativePath, gotPath *os.PathError
				if !errors.As(nativeErr, &nativePath) || !errors.As(gotErr, &gotPath) || !errors.Is(gotErr, nativePath.Err) || gotPath.Op != nativePath.Op || gotPath.Path != nativePath.Path {
					t.Fatalf("native refusal = (%v,%v), want exact Go Readlink cause and operation", gotErr, nativeErr)
				}
			}
			if tc.wantErr == nil {
				want := tc.wantTarget
				if tc.absoluteTarget {
					want = filepath.Join(container, "outside", "entry")
				}
				if nativeErr != nil || nativeTarget != want || got.String() != want || got.Validate() != nil {
					t.Fatalf("target = (%q,%v), native = (%q,%v), want %q", got.String(), gotErr, nativeTarget, nativeErr, want)
				}
			} else if got != (filestore.SymbolicLinkTarget{}) {
				t.Fatalf("refused target = %+v, want zero", got)
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
