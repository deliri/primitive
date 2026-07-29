package filestore_test

import (
	"errors"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestInstallModeExhaustsUnderlyingDomain(t *testing.T) {
	t.Parallel()

	admitted := []filestore.InstallMode{
		filestore.InstallCreate,
		filestore.InstallReplace,
	}
	for raw := uint16(0); raw <= math.MaxUint8; raw++ {
		got := filestore.InstallMode(raw)
		gotErr := got.Validate()
		if slices.Contains(admitted, got) {
			if gotErr != nil {
				t.Fatalf("InstallMode(%d).Validate() error = %v, want nil", raw, gotErr)
			}
			continue
		}
		if !errors.Is(gotErr, core.ErrFilestoreContract) {
			t.Fatalf("InstallMode(%d).Validate() error = %v, want %v", raw, gotErr, core.ErrFilestoreContract)
		}
	}
}

func TestPermissionModeHostileBoundaryMatrix(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	root, err := os.OpenRoot(rootDirectory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("os.Root.Close() error = %v, want nil", closeErr)
		}
	})
	location := filestore.Location{Root: root, Path: mustRelativePath(t, "target")}
	for raw := uint32(0); raw <= uint32(fs.ModePerm); raw++ {
		mode := fs.FileMode(raw)
		gotErr := (filestore.DirectoryRequest{Location: location, Mode: mode}).Validate()
		if mode == 0 {
			if !errors.Is(gotErr, core.ErrFilestoreContract) {
				t.Fatalf("DirectoryRequest mode %#o error = %v, want %v", mode, gotErr, core.ErrFilestoreContract)
			}
			continue
		}
		if gotErr != nil {
			t.Fatalf("DirectoryRequest permission mode %#o error = %v, want nil", mode, gotErr)
		}
	}
	nonPermissionBits := []fs.FileMode{
		fs.ModeDir,
		fs.ModeAppend,
		fs.ModeExclusive,
		fs.ModeTemporary,
		fs.ModeSymlink,
		fs.ModeDevice,
		fs.ModeNamedPipe,
		fs.ModeSocket,
		fs.ModeSetuid,
		fs.ModeSetgid,
		fs.ModeCharDevice,
		fs.ModeSticky,
		fs.ModeIrregular,
	}
	for _, bit := range nonPermissionBits {
		for _, mode := range []fs.FileMode{bit, bit | 0o600, bit | fs.ModePerm} {
			gotErr := (filestore.DirectoryRequest{Location: location, Mode: mode}).Validate()
			if !errors.Is(gotErr, core.ErrFilestoreContract) {
				t.Fatalf("DirectoryRequest non-permission mode %#o error = %v, want %v", mode, gotErr, core.ErrFilestoreContract)
			}
		}
	}
}

func TestRequestsRejectUnsetOwnershipBoundaries(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	root, err := os.OpenRoot(rootDirectory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("os.Root.Close() error = %v, want nil", closeErr)
		}
	})
	target := mustRelativePath(t, "target")
	directory := mustRelativePath(t, "directory")
	positive := mustByteCount(t, 1)
	location := filestore.Location{Root: root, Path: target}
	staged := mustStage(t, root, ".commit-stage", "x")
	outgoing, err := os.Create(filepath.Join(rootDirectory, "outgoing"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := outgoing.Close(); closeErr != nil {
			t.Errorf("outgoing Close() error = %v, want nil", closeErr)
		}
	})
	cases := []struct {
		run       func(*testing.T) error
		name      string
		wantValid bool
	}{
		{name: "valid location owns root and path", wantValid: true, run: func(_ *testing.T) error {
			return location.Validate()
		}},
		{name: "valid directory owns location and permission mode", wantValid: true, run: func(_ *testing.T) error {
			return (filestore.DirectoryRequest{Location: location, Mode: 0o700}).Validate()
		}},
		{name: "valid read owns destination location and positive maximum", wantValid: true, run: func(_ *testing.T) error {
			return (filestore.ReadRequest{
				Destination: io.Discard, Location: location, MaximumBytes: positive,
			}).Validate()
		}},
		{name: "valid create write owns every activation boundary", wantValid: true, run: func(t *testing.T) error {
			return (filestore.WriteRequest{
				Source: strings.NewReader("x"), Location: location,
				Temporary: mustRelativePath(t, ".target-stage"), Mode: 0o600,
				Install: filestore.InstallCreate, MaximumBytes: positive,
			}).Validate()
		}},
		{name: "valid replace write admits absent or existing target policy", wantValid: true, run: func(t *testing.T) error {
			return (filestore.WriteRequest{
				Source: strings.NewReader("x"), Location: location,
				Temporary: mustRelativePath(t, ".target-stage"), Mode: 0o600,
				Install: filestore.InstallReplace, MaximumBytes: positive,
			}).Validate()
		}},
		{name: "valid target-late stage owns source name mode and maximum", wantValid: true, run: func(t *testing.T) error {
			return (filestore.StageRequest{
				Source:    strings.NewReader("x"),
				Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, ".stage")},
				Mode:      0o600, MaximumBytes: positive,
			}).Validate()
		}},
		{name: "valid staged receipt retains exact file identity and bytes", wantValid: true, run: func(_ *testing.T) error {
			return staged.Validate()
		}},
		{name: "valid create commit owns stage target and install mode", wantValid: true, run: func(_ *testing.T) error {
			return (filestore.CommitRequest{
				Staged: staged, Target: target, Install: filestore.InstallCreate,
			}).Validate()
		}},
		{name: "valid replace commit owns stage target and install mode", wantValid: true, run: func(_ *testing.T) error {
			return (filestore.CommitRequest{
				Staged: staged, Target: target, Install: filestore.InstallReplace,
			}).Validate()
		}},
		{name: "valid append owns location permission and create-or-open intent", wantValid: true, run: func(_ *testing.T) error {
			return (filestore.AppendRequest{
				Location: location,
				Mode:     0o600,
				Append:   filestore.AppendCreateOrOpen,
			}).Validate()
		}},
		{name: "valid rotation transfers a real outgoing handle and incoming request", wantValid: true, run: func(_ *testing.T) error {
			return (filestore.RotationRequest{
				Outgoing: outgoing,
				Incoming: filestore.AppendRequest{
					Location: location,
					Mode:     0o600,
					Append:   filestore.AppendCreate,
				},
			}).Validate()
		}},
		{name: "valid removal owns one mutable rooted name", wantValid: true, run: func(_ *testing.T) error {
			return (filestore.RemovalRequest{Location: location}).Validate()
		}},
		{name: "location without root", run: func(_ *testing.T) error {
			return (filestore.Location{Path: target}).Validate()
		}},
		{name: "location without path", run: func(_ *testing.T) error {
			return (filestore.Location{Root: root}).Validate()
		}},
		{name: "directory without mode", run: func(_ *testing.T) error {
			return (filestore.DirectoryRequest{Location: location}).Validate()
		}},
		{name: "read without destination", run: func(_ *testing.T) error {
			return (filestore.ReadRequest{Location: location, MaximumBytes: positive}).Validate()
		}},
		{name: "read without maximum", run: func(_ *testing.T) error {
			return (filestore.ReadRequest{Location: location, Destination: io.Discard}).Validate()
		}},
		{name: "write without source", run: func(t *testing.T) error {
			return (filestore.WriteRequest{
				Location: location, Temporary: mustRelativePath(t, ".target-stage"), Mode: 0o600,
				Install: filestore.InstallCreate, MaximumBytes: positive,
			}).Validate()
		}},
		{name: "write without install mode", run: func(t *testing.T) error {
			return (filestore.WriteRequest{
				Source: strings.NewReader("x"), Location: location,
				Temporary: mustRelativePath(t, ".target-stage"), Mode: 0o600, MaximumBytes: positive,
			}).Validate()
		}},
		{name: "write without temporary path", run: func(_ *testing.T) error {
			return (filestore.WriteRequest{
				Source: strings.NewReader("x"), Location: location,
				Mode: 0o600, Install: filestore.InstallCreate, MaximumBytes: positive,
			}).Validate()
		}},
		{name: "stage without root", run: func(t *testing.T) error {
			return (filestore.StageRequest{
				Source:    strings.NewReader("x"),
				Temporary: filestore.Location{Path: mustRelativePath(t, filepath.Join(directory.String(), ".stage"))},
				Mode:      0o600, MaximumBytes: positive,
			}).Validate()
		}},
		{name: "stage without temporary path", run: func(_ *testing.T) error {
			return (filestore.StageRequest{
				Source:    strings.NewReader("x"),
				Temporary: filestore.Location{Root: root},
				Mode:      0o600, MaximumBytes: positive,
			}).Validate()
		}},
		{name: "stage without source", run: func(t *testing.T) error {
			return (filestore.StageRequest{
				Temporary: filestore.Location{
					Root: root,
					Path: mustRelativePath(t, filepath.Join(directory.String(), ".stage")),
				},
				Mode: 0o600, MaximumBytes: positive,
			}).Validate()
		}},
		{name: "zero staged file", run: func(_ *testing.T) error {
			return (filestore.StagedFile{}).Validate()
		}},
		{name: "commit without staged file", run: func(_ *testing.T) error {
			return (filestore.CommitRequest{
				Target: target, Install: filestore.InstallCreate,
			}).Validate()
		}},
		{name: "commit without target", run: func(_ *testing.T) error {
			return (filestore.CommitRequest{
				Staged: staged, Install: filestore.InstallCreate,
			}).Validate()
		}},
		{name: "commit without install mode", run: func(_ *testing.T) error {
			return (filestore.CommitRequest{
				Staged: staged, Target: target,
			}).Validate()
		}},
		{name: "append without permission mode", run: func(_ *testing.T) error {
			return (filestore.AppendRequest{Location: location}).Validate()
		}},
		{name: "append without namespace intent", run: func(_ *testing.T) error {
			return (filestore.AppendRequest{Location: location, Mode: 0o600}).Validate()
		}},
		{name: "rotation without outgoing handle", run: func(_ *testing.T) error {
			return (filestore.RotationRequest{
				Incoming: filestore.AppendRequest{
					Location: location,
					Mode:     0o600,
					Append:   filestore.AppendCreate,
				},
			}).Validate()
		}},
		{name: "rotation incoming mode cannot reopen an existing generation", run: func(_ *testing.T) error {
			return (filestore.RotationRequest{
				Outgoing: outgoing,
				Incoming: filestore.AppendRequest{
					Location: location,
					Mode:     0o600,
					Append:   filestore.AppendExisting,
				},
			}).Validate()
		}},
		{name: "removal without location", run: func(_ *testing.T) error {
			return (filestore.RemovalRequest{}).Validate()
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.run(t)
			if tc.wantValid {
				if gotErr != nil {
					t.Fatalf("request.Validate() error = %v, want nil", gotErr)
				}
				return
			}
			if !errors.Is(gotErr, core.ErrFilestoreContract) {
				t.Fatalf("request.Validate() error = %v, want %v", gotErr, core.ErrFilestoreContract)
			}
		})
	}
}

func TestMutationRequestsRejectNonAtomicOrRootEntryPaths(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	root := requireTestRoot(t, rootDirectory)
	positive := mustByteCount(t, 1)
	target := mustRelativePath(t, filepath.Join("objects", "target"))
	cases := []struct {
		wantErr error
		run     func(*testing.T) error
		name    string
	}{
		{
			name:    "write temporary equals target",
			wantErr: core.ErrFilestoreContract,
			run: func(_ *testing.T) error {
				return (filestore.WriteRequest{
					Source:       strings.NewReader("x"),
					Location:     filestore.Location{Root: root, Path: target},
					Temporary:    target,
					Mode:         0o600,
					Install:      filestore.InstallCreate,
					MaximumBytes: positive,
				}).Validate()
			},
		},
		{
			name:    "write temporary is in a different directory",
			wantErr: core.ErrFilestoreContract,
			run: func(t *testing.T) error {
				return (filestore.WriteRequest{
					Source:       strings.NewReader("x"),
					Location:     filestore.Location{Root: root, Path: target},
					Temporary:    mustRelativePath(t, filepath.Join("staging", ".target")),
					Mode:         0o600,
					Install:      filestore.InstallCreate,
					MaximumBytes: positive,
				}).Validate()
			},
		},
		{
			name:    "write target cannot be the root entry",
			wantErr: core.ErrFilestoreContract,
			run: func(t *testing.T) error {
				return (filestore.WriteRequest{
					Source:       strings.NewReader("x"),
					Location:     filestore.Location{Root: root, Path: mustRelativePath(t, ".")},
					Temporary:    mustRelativePath(t, ".stage"),
					Mode:         0o600,
					Install:      filestore.InstallCreate,
					MaximumBytes: positive,
				}).Validate()
			},
		},
		{
			name:    "stage temporary cannot be the root entry",
			wantErr: core.ErrFilestoreContract,
			run: func(t *testing.T) error {
				return (filestore.StageRequest{
					Source:       strings.NewReader("x"),
					Temporary:    filestore.Location{Root: root, Path: mustRelativePath(t, ".")},
					Mode:         0o600,
					MaximumBytes: positive,
				}).Validate()
			},
		},
		{
			name:    "append target cannot be the root entry",
			wantErr: core.ErrFilestoreContract,
			run: func(t *testing.T) error {
				return (filestore.AppendRequest{
					Location: filestore.Location{Root: root, Path: mustRelativePath(t, ".")},
					Mode:     0o600,
					Append:   filestore.AppendCreate,
				}).Validate()
			},
		},
		{
			name:    "removal target cannot be the root entry",
			wantErr: core.ErrFilestoreContract,
			run: func(t *testing.T) error {
				return (filestore.RemovalRequest{
					Location: filestore.Location{Root: root, Path: mustRelativePath(t, ".")},
				}).Validate()
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if gotErr := tc.run(t); !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("request.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
	if err := os.Mkdir(filepath.Join(rootDirectory, "objects"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(rootDirectory, "staging"), 0o700); err != nil {
		t.Fatal(err)
	}
	staged := mustStage(t, root, filepath.Join("staging", ".stage"), "x")
	gotErr := (filestore.CommitRequest{
		Staged:  staged,
		Target:  target,
		Install: filestore.InstallCreate,
	}).Validate()
	if gotErr != nil {
		t.Fatalf("cross-directory CommitRequest.Validate() error = %v, want nil within one rooted capability", gotErr)
	}
}

func TestRelativePathCannotBypassRealRootConfinement(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	rootDirectory := filepath.Join(parent, "root")
	outsideDirectory := filepath.Join(parent, "outside")
	if err := os.Mkdir(rootDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outsideDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideDirectory, filepath.Join(rootDirectory, "escape")); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}
	outsideTarget := filepath.Join(outsideDirectory, "target")
	if err := os.WriteFile(outsideTarget, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(rootDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("os.Root.Close() error = %v, want nil", closeErr)
		}
	}()
	_, gotErr := filestore.Write(t.Context(), filestore.WriteRequest{
		Source: strings.NewReader("escape"),
		Location: filestore.Location{
			Root: root,
			Path: mustRelativePath(t, filepath.Join("escape", "target")),
		},
		Temporary:    mustRelativePath(t, filepath.Join("escape", ".target-stage")),
		Mode:         0o600,
		Install:      filestore.InstallReplace,
		MaximumBytes: mustByteCount(t, 6),
	})
	if gotErr == nil {
		t.Fatal("Write() through escaping symlink error = nil, want confinement failure")
	}
	got, err := os.ReadFile(outsideTarget)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "outside" {
		t.Fatalf("outside target = %q, want %q", got, "outside")
	}
}

func mustRelativePath(t *testing.T, value string) core.RelativePath {
	t.Helper()

	got, err := core.ParseRelativePath(value)
	if err != nil {
		t.Fatalf("ParseRelativePath(%q) error = %v, want nil", value, err)
	}
	return got
}

func mustByteCount(t *testing.T, value uint64) core.ByteCount {
	t.Helper()

	got, err := core.NewByteCount(value)
	if err != nil {
		t.Fatalf("NewByteCount(%d) error = %v, want nil", value, err)
	}
	return got
}
