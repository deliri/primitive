package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

type symbolicNativeEntry uint8

const (
	symbolicNativeLink symbolicNativeEntry = iota
	symbolicNativeRegular
	symbolicNativeDirectory
	symbolicNativeAbsent
	symbolicNativeEntryLimit
)

func FuzzSymbolicLinkObservationAndResolution(f *testing.F) {
	seedDirectory := f.TempDir()
	if err := os.WriteFile(filepath.Join(seedDirectory, "file"), []byte{0, 255}, 0o600); err != nil {
		f.Fatal(err)
	}
	if err := os.Symlink("file", filepath.Join(seedDirectory, "link")); err != nil {
		f.Fatal(err)
	}
	root, err := os.OpenRoot(seedDirectory)
	if err != nil {
		f.Fatal(err)
	}
	path, err := core.ParseRelativePath("link")
	if err != nil {
		f.Fatal(err)
	}
	location := filestore.Location{Root: root, Path: path}
	if err := location.Validate(); err != nil {
		f.Fatal(err)
	}
	target, err := filestore.ReadSymbolicLink(f.Context(), location)
	if err != nil {
		f.Fatal(err)
	}
	absolute, err := core.ParseAbsolutePath(filepath.Join(seedDirectory, path.String()))
	if err != nil {
		f.Fatal(err)
	}
	canonical, err := filestore.Canonicalize(f.Context(), absolute)
	if err != nil || canonical.Validate() != nil {
		f.Fatalf("canonical seed = (%v,%v), want admitted existing path", canonical, err)
	}
	if err := root.Close(); err != nil {
		f.Fatal(err)
	}
	f.Add(target.String(), []byte{0, 255}, uint8(symbolicNativeLink), uint8(symbolicObservationAdmitted), false)
	for _, target := range []string{"", "\x00", "\xff", "missing", "subject", "./directory/../file", "directory/entry", "socket:[123]"} {
		f.Add(target, []byte{0, 255}, uint8(symbolicNativeLink), uint8(symbolicObservationAdmitted), false)
	}
	for _, kind := range []symbolicNativeEntry{symbolicNativeRegular, symbolicNativeDirectory, symbolicNativeAbsent} {
		f.Add(target.String(), []byte{}, uint8(kind), uint8(symbolicObservationAdmitted), false)
	}
	for _, ingress := range []symbolicObservationIngress{symbolicObservationNilRoot, symbolicObservationZeroPath, symbolicObservationClosedRoot, symbolicObservationNilContext, symbolicObservationCanceled} {
		f.Add(target.String(), []byte{0, 255}, uint8(symbolicNativeLink), uint8(ingress), false)
	}
	f.Add("entry", []byte{0, 255}, uint8(symbolicNativeLink), uint8(symbolicObservationAdmitted), true)
	f.Fuzz(func(t *testing.T, rawTarget string, payload []byte, rawKind, rawIngress uint8, outside bool) {
		rawTarget = rawTarget[:min(len(rawTarget), 1024)]
		payload = payload[:min(len(payload), 1024)]
		kind := symbolicNativeEntry(rawKind % uint8(symbolicNativeEntryLimit))
		ingress := symbolicObservationIngress(rawIngress % uint8(symbolicObservationCanceled+1))
		container := t.TempDir()
		if err := createSymbolicLinkNativeFixture(container); err != nil {
			t.Fatal(err)
		}
		directory := filepath.Join(container, "root")
		parent := directory
		pathText := "subject"
		if outside {
			parent = filepath.Join(container, "outside")
			pathText = filepath.Join("outside", "subject")
		}
		subject := filepath.Join(parent, "subject")
		switch kind {
		case symbolicNativeLink:
			if err := os.Symlink(rawTarget, subject); err != nil {
				if _, ok := errors.AsType[*os.LinkError](err); !ok {
					t.Fatalf("native fixture refusal = %v, want LinkError", err)
				}
				if info, err := os.Lstat(subject); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("refused native link = (%v,%v), want absent", info, err)
				}
			}
		case symbolicNativeRegular:
			if err := os.WriteFile(subject, payload, 0o600); err != nil {
				t.Fatal(err)
			}
		case symbolicNativeDirectory:
			if err := os.Mkdir(subject, 0o700); err != nil {
				t.Fatal(err)
			}
		case symbolicNativeAbsent:
		default:
			t.Fatalf("kind = %v, want declared native fixture", kind)
		}
		root, err := os.OpenRoot(directory)
		if err != nil {
			t.Fatal(err)
		}
		if ingress == symbolicObservationClosedRoot {
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
		relative, err := core.ParseRelativePath(pathText)
		if err != nil {
			t.Fatal(err)
		}
		absolute, err := core.ParseAbsolutePath(filepath.Join(directory, pathText))
		if err != nil {
			t.Fatal(err)
		}
		location := filestore.Location{Root: root, Path: relative}
		ctx := t.Context()
		var linkIngressErr, canonicalIngressErr error
		switch ingress {
		case symbolicObservationAdmitted, symbolicObservationClosedRoot:
		case symbolicObservationNilRoot:
			location.Root = nil
			linkIngressErr = core.ErrFilestoreContract
		case symbolicObservationZeroPath:
			location.Path = core.RelativePath{}
			absolute = core.AbsolutePath{}
			linkIngressErr = core.ErrFilestoreContract
			canonicalIngressErr = core.ErrFilestoreContract
		case symbolicObservationNilContext:
			ctx = nil
			linkIngressErr = core.ErrNilContext
			canonicalIngressErr = core.ErrNilContext
		case symbolicObservationCanceled:
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(ctx)
			cancel()
			linkIngressErr = context.Canceled
			canonicalIngressErr = context.Canceled
		default:
			t.Fatalf("ingress = %v, want declared fixture", ingress)
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
		nativeTarget, nativeLinkErr := root.Readlink(relative.String())
		gotTarget, linkErr := filestore.ReadSymbolicLink(ctx, location)
		switch {
		case linkIngressErr != nil:
			if !errors.Is(linkErr, linkIngressErr) || errors.Is(linkErr, core.ErrFilestoreSource) {
				t.Fatalf("link ingress = %v, want %v without source identity", linkErr, linkIngressErr)
			}
		case nativeLinkErr != nil:
			var native, got *os.PathError
			if !errors.Is(linkErr, core.ErrFilestoreSource) || !errors.As(nativeLinkErr, &native) || !errors.As(linkErr, &got) || !errors.Is(linkErr, native.Err) || got.Path != native.Path || got.Op != native.Op {
				t.Fatalf("link refusal = (%v,%v), want exact native source cause", linkErr, nativeLinkErr)
			}
		default:
			admissible := len(nativeTarget) > 0 && !strings.ContainsRune(nativeTarget, 0)
			if !admissible {
				if !errors.Is(linkErr, core.ErrFilestoreContract) || errors.Is(linkErr, core.ErrFilestoreSource) {
					t.Fatalf("observed target admission = %v, want exact contract refusal for %q", linkErr, nativeTarget)
				}
			} else if linkErr != nil || gotTarget.Validate() != nil || gotTarget.String() != nativeTarget {
				t.Fatalf("link observation = (%q,%v), want %q", gotTarget.String(), linkErr, nativeTarget)
			}
			if !outside && kind == symbolicNativeLink && nativeTarget != rawTarget {
				t.Fatalf("native target = %q, want mutated bytes %q", nativeTarget, rawTarget)
			}
		}
		if linkErr != nil && gotTarget != (filestore.SymbolicLinkTarget{}) {
			t.Fatalf("refused link target = %+v, want zero", gotTarget)
		}
		nativeCanonical, nativeCanonicalErr := filepath.EvalSymlinks(absolute.String())
		gotCanonical, canonicalErr := filestore.Canonicalize(ctx, absolute)
		switch {
		case canonicalIngressErr != nil:
			if !errors.Is(canonicalErr, canonicalIngressErr) || errors.Is(canonicalErr, core.ErrFilestoreSource) {
				t.Fatalf("canonical ingress = %v, want %v without source identity", canonicalErr, canonicalIngressErr)
			}
		case nativeCanonicalErr != nil:
			if !errors.Is(canonicalErr, core.ErrFilestoreSource) {
				t.Fatalf("canonical refusal = %v, want native source %v", canonicalErr, nativeCanonicalErr)
			}
			// witness:waiver test/errors -- Go EvalSymlinks link-count refusal is a fresh untyped error; Source is the stable wrapper while native typed causes remain required below.
			var native, got *os.PathError
			if errors.As(nativeCanonicalErr, &native) && (!errors.As(canonicalErr, &got) || !errors.Is(canonicalErr, native.Err) || got.Op != native.Op || got.Path != native.Path) {
				t.Fatalf("canonical native cause = (%v,%v), want exact PathError", canonicalErr, nativeCanonicalErr)
			}
			var errno syscall.Errno
			if errors.As(nativeCanonicalErr, &errno) && !errors.Is(canonicalErr, errno) {
				t.Fatalf("canonical native errno = %v, want %v", canonicalErr, errno)
			}
		default:
			want, parseErr := core.ParseAbsolutePath(nativeCanonical)
			if parseErr != nil {
				if !errors.Is(canonicalErr, core.ErrFilestoreContract) || errors.Is(canonicalErr, core.ErrFilestoreSource) {
					t.Fatalf("resolved nominal refusal = %v, want contract from %v", canonicalErr, parseErr)
				}
			} else {
				if canonicalErr != nil || gotCanonical != want || gotCanonical.Validate() != nil {
					t.Fatalf("canonical observation = (%v,%v), want %v", gotCanonical, canonicalErr, want)
				}
				again, err := filestore.Canonicalize(ctx, gotCanonical)
				if err != nil || again != gotCanonical {
					t.Fatalf("canonical fixed point = (%v,%v), want %v", again, err, gotCanonical)
				}
			}
		}
		if canonicalErr != nil && gotCanonical != (core.AbsolutePath{}) {
			t.Fatalf("refused canonical path = %v, want zero", gotCanonical)
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
				t.Fatalf("retained identity/time = (%v,%v), want %v", info, err, metadata[i])
			}
		}
	})
}
