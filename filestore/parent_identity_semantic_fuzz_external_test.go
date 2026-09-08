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

type nativeParentFixture uint8

const (
	nativeParentReal nativeParentFixture = iota
	nativeParentAlias
	nativeParentOutsideAlias
	nativeParentMissing
	nativeParentFile
	nativeParentLoop
	nativeParentFixtureLimit
)

func FuzzOpenParentAndRootIdentityNativeCustody(f *testing.F) {
	seedDirectory := f.TempDir()
	seedPath, err := core.ParseAbsolutePath(filepath.Join(seedDirectory, "retained"))
	if err != nil {
		f.Fatal(err)
	}
	seed, err := filestore.OpenParent(f.Context(), seedPath)
	if err != nil {
		f.Fatal(err)
	}
	if err := seed.Validate(); err != nil {
		f.Fatal(err)
	}
	base := seed.Path.String()
	if err := seed.Root.Close(); err != nil {
		f.Fatal(err)
	}
	for parent := range nativeParentFixtureLimit {
		f.Add(uint8(parent), uint8(rootIdentityOriginal), base, []byte{0, 255}, false)
	}
	for identity := rootIdentityAlias; identity <= rootIdentityLoop; identity++ {
		f.Add(uint8(nativeParentReal), uint8(identity), base, []byte{0, 255}, false)
	}
	f.Add(uint8(nativeParentReal), uint8(rootIdentityOriginal), " ", []byte{}, false)
	f.Add(uint8(nativeParentReal), uint8(rootIdentityOriginal), "missing-child", []byte{0, 255}, false)
	f.Add(uint8(nativeParentReal), uint8(rootIdentityOriginal), base, []byte{0, 255}, true)
	f.Add(uint8(nativeParentReal), uint8(rootIdentityOriginal), "", []byte{0, 255}, false)
	f.Fuzz(func(t *testing.T, rawParent, rawIdentity uint8, rawChild string, payload []byte, canceled bool) {
		var err error
		payload = payload[:min(len(payload), 1024)]
		rawChild = rawChild[:min(len(rawChild), 512)]
		parentKind := nativeParentFixture(rawParent % uint8(nativeParentFixtureLimit))
		identityKind := rootIdentityFixture(rawIdentity % uint8(rootIdentityLoop+1))
		container := t.TempDir()
		real := filepath.Join(container, "real")
		outside := filepath.Join(container, "outside")
		for _, directory := range []string{real, outside} {
			if err := os.Mkdir(directory, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, base), payload, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		for _, name := range []string{"neighbor", "file-parent"} {
			if err := os.WriteFile(filepath.Join(container, name), payload, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		for _, link := range []struct{ name, target string }{{"alias", "real"}, {"outside-alias", "outside"}, {"loop", "loop"}} {
			if err := os.Symlink(link.target, filepath.Join(container, link.name)); err != nil {
				t.Fatal(err)
			}
		}
		parentPath := real
		switch parentKind {
		case nativeParentReal:
		case nativeParentAlias:
			parentPath = filepath.Join(container, "alias")
		case nativeParentOutsideAlias:
			parentPath = filepath.Join(container, "outside-alias")
		case nativeParentMissing:
			parentPath = filepath.Join(container, "missing")
		case nativeParentFile:
			parentPath = filepath.Join(container, "file-parent")
		case nativeParentLoop:
			parentPath = filepath.Join(container, "loop")
		}
		component, componentErr := core.ParsePathComponent(rawChild)
		path := core.AbsolutePath{}
		if componentErr == nil {
			path, err = core.ParseAbsolutePath(filepath.Join(parentPath, component.String()))
			if err != nil {
				t.Fatal(err)
			}
		}
		ctx := t.Context()
		if canceled {
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(ctx)
			cancel()
		}
		var wantBoundary, wantNative error
		var parentBefore fs.FileInfo
		switch {
		case canceled:
			wantBoundary = context.Canceled
		case componentErr != nil:
			wantBoundary = core.ErrFilestoreContract
		default:
			parentBefore, err = os.Stat(parentPath + string(filepath.Separator) + ".")
			if err != nil {
				var native *fs.PathError
				if !errors.As(err, &native) {
					t.Fatal(err)
				}
				wantBoundary = core.ErrFilestoreSource
				wantNative = native.Err
			}
		}
		before, err := removalFixtureSnapshot(container)
		if err != nil {
			t.Fatal(err)
		}
		got, gotErr := filestore.OpenParent(ctx, path)
		rootClosed := false
		if got.Root != nil {
			t.Cleanup(func() {
				if !rootClosed {
					if err := got.Root.Close(); err != nil {
						t.Error(err)
					}
				}
			})
		}
		if !errors.Is(gotErr, wantBoundary) || (wantBoundary == core.ErrFilestoreContract && errors.Is(gotErr, core.ErrFilestoreSource)) {
			t.Fatalf("OpenParent = (%+v,%v), want exact %v", got, gotErr, wantBoundary)
		}
		if wantNative != nil {
			var native *fs.PathError
			if !errors.Is(gotErr, wantNative) || !errors.As(gotErr, &native) {
				t.Fatalf("parent native cause = %v, want %v and PathError", gotErr, wantNative)
			}
		}
		after, err := removalFixtureSnapshot(container)
		if err != nil {
			t.Fatal(err)
		}
		if len(after) != len(before) {
			t.Fatalf("parent acquisition namespace = %+v, want unchanged %+v", after, before)
		}
		for i, entry := range before {
			if after[i].name != entry.name || after[i].mode != entry.mode || after[i].target != entry.target || !bytes.Equal(after[i].data, entry.data) {
				t.Fatalf("parent acquisition entry %d = %+v, want %+v", i, after[i], entry)
			}
		}
		if gotErr != nil {
			if got != (filestore.Location{}) {
				t.Fatalf("refused parent = %+v, want exact zero", got)
			}
			return
		}
		wantPath := mustRelativePath(t, component.String())
		if err := got.Validate(); err != nil || got.Path != wantPath {
			t.Fatalf("location path/validation = (%v,%v), want %v", got.Path, err, wantPath)
		}
		heldBefore, err := got.Root.Stat(".")
		if err != nil || !os.SameFile(parentBefore, heldBefore) {
			t.Fatalf("returned parent = (%v,%v), want native %v", heldBefore, err, parentBefore)
		}
		parentAbsolute, err := core.ParseAbsolutePath(parentPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := filestore.ValidateRootIdentity(got.Root, parentAbsolute); err != nil {
			t.Fatalf("acquired parent identity = %v, want nil", err)
		}
		nativeChild, nativeChildErr := os.Lstat(path.String())
		ownedChild, ownedChildErr := got.Root.Lstat(got.Path.String())
		if nativeChildErr != nil {
			var native *fs.PathError
			if !errors.As(nativeChildErr, &native) || !errors.Is(ownedChildErr, native.Err) || ownedChild != nil {
				t.Fatalf("rooted child = (%v,%v), want native %v", ownedChild, ownedChildErr, nativeChildErr)
			}
		} else if ownedChildErr != nil || !os.SameFile(nativeChild, ownedChild) {
			t.Fatalf("rooted child = (%v,%v), want native %v", ownedChild, ownedChildErr, nativeChild)
		}
		data, err := got.Root.ReadFile(base)
		if err != nil || !bytes.Equal(data, payload) {
			t.Fatalf("rooted payload = (%v,%v), want %v", data, err, payload)
		}
		physical, err := filepath.EvalSymlinks(parentPath)
		if err != nil {
			t.Fatal(err)
		}
		identityPath := parentPath
		candidate := got.Root
		archive := filepath.Join(container, "archive")
		switch identityKind {
		case rootIdentityOriginal:
		case rootIdentityAlias:
			identityPath = filepath.Join(container, "second-alias")
			if err := os.Symlink(physical, identityPath); err != nil {
				t.Fatal(err)
			}
		case rootIdentityRenamedMissing, rootIdentityRenamedOwned, rootIdentityReplacedSameBytes, rootIdentityReplacedFile, rootIdentityReplacedLinkToOwned:
			if err := os.Rename(physical, archive); err != nil {
				t.Fatal(err)
			}
			switch identityKind {
			case rootIdentityRenamedOwned:
				identityPath = archive
			case rootIdentityReplacedSameBytes:
				if err := os.Mkdir(physical, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(physical, base), payload, 0o600); err != nil {
					t.Fatal(err)
				}
				foreign, err := os.Stat(physical)
				if err != nil || os.SameFile(heldBefore, foreign) {
					t.Fatalf("replacement fixture = (%v,%v), want distinct inode", foreign, err)
				}
			case rootIdentityReplacedFile:
				if err := os.WriteFile(physical, payload, 0o600); err != nil {
					t.Fatal(err)
				}
			case rootIdentityReplacedLinkToOwned:
				if err := os.Symlink(archive, physical); err != nil {
					t.Fatal(err)
				}
			}
		case rootIdentityNil:
			candidate = nil
		case rootIdentityClosed:
			if err := got.Root.Close(); err != nil {
				t.Fatal(err)
			}
			rootClosed = true
		case rootIdentityZeroPath:
		case rootIdentityMissingParent:
			identityPath = filepath.Join(container, "absent", "root")
		case rootIdentityBelowFile:
			identityPath = filepath.Join(container, "file-parent", "root")
		case rootIdentityLoop:
			identityPath = filepath.Join(container, "loop")
		}
		identityAbsolute := core.AbsolutePath{}
		if identityKind != rootIdentityZeroPath {
			identityAbsolute, err = core.ParseAbsolutePath(identityPath)
			if err != nil {
				t.Fatal(err)
			}
		}
		wantBoundary = nil
		wantNative = nil
		switch {
		case identityKind == rootIdentityNil || identityKind == rootIdentityZeroPath:
			wantBoundary = core.ErrFilestoreContract
		case rootClosed:
			wantBoundary = core.ErrFilestoreSource
			wantNative = fs.ErrClosed
		default:
			observed, nativeErr := os.Stat(identityPath)
			if nativeErr != nil {
				var native *fs.PathError
				if !errors.As(nativeErr, &native) {
					t.Fatal(nativeErr)
				}
				wantBoundary = core.ErrFilestoreSource
				wantNative = native.Err
			} else if !os.SameFile(heldBefore, observed) {
				wantBoundary = core.ErrFilestoreContract
			}
		}
		before, err = removalFixtureSnapshot(container)
		if err != nil {
			t.Fatal(err)
		}
		infos := make([]fs.FileInfo, len(before))
		for i, entry := range before {
			infos[i], err = os.Lstat(filepath.Join(container, entry.name))
			if err != nil {
				t.Fatal(err)
			}
		}
		gotErr = filestore.ValidateRootIdentity(candidate, identityAbsolute)
		if !errors.Is(gotErr, wantBoundary) || (wantBoundary == core.ErrFilestoreContract && errors.Is(gotErr, core.ErrFilestoreSource)) {
			t.Fatalf("root identity = %v, want exact %v", gotErr, wantBoundary)
		}
		if wantNative != nil {
			var native *fs.PathError
			if !errors.Is(gotErr, wantNative) || !errors.As(gotErr, &native) {
				t.Fatalf("identity native cause = %v, want %v and PathError", gotErr, wantNative)
			}
		}
		if !rootClosed {
			owned, err := got.Root.Stat(".")
			if err != nil || !os.SameFile(heldBefore, owned) || heldBefore.Mode() != owned.Mode() || heldBefore.ModTime().UnixNano() != owned.ModTime().UnixNano() {
				t.Fatalf("retained root = (%v,%v), want original %v", owned, err, heldBefore)
			}
			data, err := got.Root.ReadFile(base)
			if err != nil || !bytes.Equal(data, payload) {
				t.Fatalf("retained root bytes = (%v,%v), want %v", data, err, payload)
			}
		} else if _, err := got.Root.Stat("."); !errors.Is(err, fs.ErrClosed) {
			t.Fatalf("caller-closed root = %v, want closed", err)
		}
		after, err = removalFixtureSnapshot(container)
		if err != nil {
			t.Fatal(err)
		}
		if len(after) != len(before) {
			t.Fatalf("identity namespace = %+v, want unchanged %+v", after, before)
		}
		for i, entry := range before {
			if after[i].name != entry.name || after[i].mode != entry.mode || after[i].target != entry.target || !bytes.Equal(after[i].data, entry.data) {
				t.Fatalf("identity entry %d = %+v, want unchanged %+v", i, after[i], entry)
			}
			info, err := os.Lstat(filepath.Join(container, entry.name))
			if err != nil || !os.SameFile(infos[i], info) || infos[i].ModTime().UnixNano() != info.ModTime().UnixNano() {
				t.Fatalf("identity metadata %s = (%v,%v), want unchanged %v", entry.name, info, err, infos[i])
			}
		}
	})
}
