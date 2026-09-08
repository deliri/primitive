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

type parentAcquisitionMutation uint8

const (
	parentAcquisitionUnchanged parentAcquisitionMutation = iota
	parentAcquisitionNilContext
	parentAcquisitionCanceled
	parentAcquisitionZeroPath
	parentAcquisitionRootPath
	parentAcquisitionParentItself
	parentAcquisitionReplacedName
)

func TestOpenParentNativeCapabilityLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, parent, child, wantBase string
		createChild                   bool
		mutation                      parentAcquisitionMutation
		wantErr, wantExcluded         error
	}{
		{name: "binary child remains usable through the exact returned parent", parent: "parent", child: "child", wantBase: "child", createChild: true},
		{name: "absent child still returns its real parent", parent: "parent", child: "child", wantBase: "child"},
		{name: "parent symlink selects the actual native directory", parent: "alias", child: "child", wantBase: "child", createChild: true},
		{name: "absolute parent authority may follow an outside directory link", parent: "outside-parent", child: "retained", wantBase: "retained"},
		{name: "space child component is not trimmed or renamed", parent: "parent", child: " ", wantBase: " ", createChild: true},
		{name: "directory itself is returned as an entry in its containing root", parent: "parent", child: "child", wantBase: "parent", createChild: true, mutation: parentAcquisitionParentItself},
		{name: "replaced parent name cannot retarget the returned rooted capability", parent: "parent", child: "child", wantBase: "child", createChild: true, mutation: parentAcquisitionReplacedName},
		{name: "nil context returns no partial location", parent: "parent", child: "child", createChild: true, mutation: parentAcquisitionNilContext, wantErr: core.ErrNilContext, wantExcluded: core.ErrFilestoreContract},
		{name: "cancellation returns no rooted capability", parent: "parent", child: "child", createChild: true, mutation: parentAcquisitionCanceled, wantErr: context.Canceled, wantExcluded: core.ErrFilestoreContract},
		{name: "zero absolute path cannot select an ambient directory", parent: "parent", child: "child", createChild: true, mutation: parentAcquisitionZeroPath, wantErr: core.ErrFilestoreContract, wantExcluded: core.ErrFilestoreSource},
		{name: "filesystem root has no admitted parent-entry split", parent: "parent", child: "child", createChild: true, mutation: parentAcquisitionRootPath, wantErr: core.ErrFilestoreContract, wantExcluded: core.ErrFilestoreSource},
		{name: "missing parent preserves native absence and zero location", parent: "missing-parent", child: "child", wantErr: core.ErrFilestoreSource},
		{name: "regular parent preserves native not-directory refusal", parent: "file-parent", child: "child", wantErr: core.ErrFilestoreSource},
		{name: "looping parent preserves the native link refusal", parent: "loop-parent", child: "child", wantErr: core.ErrFilestoreSource},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			container := t.TempDir()
			outside := t.TempDir()
			payload := []byte{0, 255, 1}
			if err := os.Mkdir(filepath.Join(container, "parent"), 0o700); err != nil {
				t.Fatal(err)
			}
			for _, entry := range []struct {
				name string
				data []byte
			}{{filepath.Join(container, "neighbor"), payload}, {filepath.Join(container, "file-parent"), payload}, {filepath.Join(container, "parent", "referent"), payload}, {filepath.Join(outside, "retained"), payload}} {
				if err := os.WriteFile(entry.name, entry.data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			for _, link := range []struct{ name, target string }{{"alias", "parent"}, {"outside-parent", outside}, {"loop-parent", "loop-parent"}} {
				if err := os.Symlink(link.target, filepath.Join(container, link.name)); err != nil {
					t.Fatal(err)
				}
			}
			childName := filepath.Join(container, tc.parent, tc.child)
			if tc.createChild {
				if err := os.WriteFile(childName, payload, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			full := childName
			if tc.mutation == parentAcquisitionParentItself {
				full = filepath.Join(container, "parent")
			}
			if tc.mutation == parentAcquisitionRootPath {
				full = filepath.VolumeName(container) + string(filepath.Separator)
			}
			path := core.AbsolutePath{}
			var err error
			if tc.mutation != parentAcquisitionZeroPath {
				path, err = core.ParseAbsolutePath(full)
				if err != nil {
					t.Fatal(err)
				}
			}
			ctx := t.Context()
			if tc.mutation == parentAcquisitionNilContext {
				ctx = nil
			}
			if tc.mutation == parentAcquisitionCanceled {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			var parentBefore, childBefore fs.FileInfo
			var childBeforeErr, wantNative error
			if tc.wantErr == nil {
				parentBefore, err = os.Stat(filepath.Dir(full))
				if err != nil {
					t.Fatal(err)
				}
				childBefore, childBeforeErr = os.Lstat(full)
			} else if errors.Is(tc.wantErr, core.ErrFilestoreSource) {
				_, nativeErr := os.Stat(filepath.Dir(full) + string(filepath.Separator) + ".")
				var native *fs.PathError
				if !errors.As(nativeErr, &native) {
					t.Fatalf("native parent refusal = %v, want PathError", nativeErr)
				}
				wantNative = native.Err
			}
			var snapshots [2][]removalFixtureEntry
			var infos [2][]fs.FileInfo
			for i, directory := range [...]string{container, outside} {
				snapshots[i], err = removalFixtureSnapshot(directory)
				if err != nil {
					t.Fatal(err)
				}
				infos[i] = make([]fs.FileInfo, len(snapshots[i]))
				for j, entry := range snapshots[i] {
					infos[i][j], err = os.Lstat(filepath.Join(directory, entry.name))
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			got, gotErr := filestore.OpenParent(ctx, path)
			if got.Root != nil {
				t.Cleanup(func() {
					if err := got.Root.Close(); err != nil {
						t.Error(err)
					}
				})
			}
			if !errors.Is(gotErr, tc.wantErr) || (tc.wantExcluded != nil && errors.Is(gotErr, tc.wantExcluded)) {
				t.Fatalf("OpenParent = (%+v,%v), want %v without %v", got, gotErr, tc.wantErr, tc.wantExcluded)
			}
			if wantNative != nil {
				var native *fs.PathError
				if !errors.Is(gotErr, wantNative) || !errors.As(gotErr, &native) {
					t.Fatalf("native cause = %v, want %v and PathError", gotErr, wantNative)
				}
			}
			if gotErr != nil {
				if got != (filestore.Location{}) {
					t.Fatalf("refused location = %+v, want exact zero", got)
				}
			} else {
				wantPath := mustRelativePath(t, tc.wantBase)
				if err := got.Validate(); err != nil || got.Path != wantPath {
					t.Fatalf("location path/validation = (%v,%v), want %v and nil", got.Path, err, wantPath)
				}
				parentAfter, err := got.Root.Stat(".")
				if err != nil || !os.SameFile(parentBefore, parentAfter) || parentBefore.Mode() != parentAfter.Mode() || parentBefore.ModTime().UnixNano() != parentAfter.ModTime().UnixNano() {
					t.Fatalf("returned parent = (%v,%v), want exact native %v", parentAfter, err, parentBefore)
				}
				childAfter, childErr := got.Root.Lstat(got.Path.String())
				if childBeforeErr != nil {
					var native *fs.PathError
					if !errors.As(childBeforeErr, &native) || !errors.Is(childErr, native.Err) || childAfter != nil {
						t.Fatalf("child refusal = (%v,%v), want native %v", childAfter, childErr, childBeforeErr)
					}
				} else {
					if childErr != nil || !os.SameFile(childBefore, childAfter) || childBefore.Mode() != childAfter.Mode() || childBefore.ModTime().UnixNano() != childAfter.ModTime().UnixNano() {
						t.Fatalf("returned child = (%v,%v), want original %v", childAfter, childErr, childBefore)
					}
					if childBefore.Mode().IsRegular() {
						data, err := got.Root.ReadFile(got.Path.String())
						if err != nil || !bytes.Equal(data, payload) {
							t.Fatalf("returned child bytes = (%v,%v), want %v", data, err, payload)
						}
					}
				}
			}
			for i, directory := range [...]string{container, outside} {
				after, err := removalFixtureSnapshot(directory)
				if err != nil {
					t.Fatal(err)
				}
				if len(after) != len(snapshots[i]) {
					t.Fatalf("observed namespace = %+v, want unchanged %+v", after, snapshots[i])
				}
				for j, entry := range snapshots[i] {
					if after[j].name != entry.name || after[j].mode != entry.mode || after[j].target != entry.target || !bytes.Equal(after[j].data, entry.data) {
						t.Fatalf("entry %d = %+v, want unchanged %+v", j, after[j], entry)
					}
					info, err := os.Lstat(filepath.Join(directory, entry.name))
					if err != nil || !os.SameFile(infos[i][j], info) || infos[i][j].ModTime().UnixNano() != info.ModTime().UnixNano() {
						t.Fatalf("retained metadata %s = (%v,%v), want %v", entry.name, info, err, infos[i][j])
					}
				}
			}
			if tc.mutation == parentAcquisitionReplacedName {
				originalPath := filepath.Join(container, "parent")
				archive := filepath.Join(container, "archive")
				if err := os.Rename(originalPath, archive); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(originalPath, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(originalPath, tc.child), payload, 0o600); err != nil {
					t.Fatal(err)
				}
				foreign, err := os.Stat(originalPath)
				if err != nil || os.SameFile(parentBefore, foreign) {
					t.Fatalf("replacement fixture = (%v,%v), want distinct native inode", foreign, err)
				}
				current, err := core.ParseAbsolutePath(originalPath)
				if err != nil {
					t.Fatal(err)
				}
				if err := filestore.ValidateRootIdentity(got.Root, current); !errors.Is(err, core.ErrFilestoreContract) || errors.Is(err, core.ErrFilestoreSource) {
					t.Fatalf("foreign path identity = %v, want exact contract refusal", err)
				}
				owned, err := core.ParseAbsolutePath(archive)
				if err != nil {
					t.Fatal(err)
				}
				if err := filestore.ValidateRootIdentity(got.Root, owned); err != nil {
					t.Fatalf("retained root identity = %v, want nil", err)
				}
				after, err := got.Root.Lstat(got.Path.String())
				if err != nil || !os.SameFile(childBefore, after) {
					t.Fatalf("retained rooted child = (%v,%v), want original inode", after, err)
				}
				data, err := got.Root.ReadFile(got.Path.String())
				if err != nil || !bytes.Equal(data, payload) {
					t.Fatalf("retained rooted child bytes = (%v,%v), want %v", data, err, payload)
				}
			}
		})
	}
}
