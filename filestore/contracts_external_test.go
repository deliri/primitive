package filestore_test

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestPermissionModeHostileBoundaryMatrix(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		mode    fs.FileMode
		wantErr error
	}{}
	for mode := fs.FileMode(0); mode <= fs.ModePerm; mode++ {
		var wantErr error
		if mode == 0 {
			wantErr = core.ErrFilestoreContract
		}
		cases = append(cases, struct {
			name    string
			mode    fs.FileMode
			wantErr error
		}{fmt.Sprintf("permission field %#o retains exact admission", mode), mode, wantErr})
	}
	for bit := fs.ModePerm + 1; bit != 0; bit <<= 1 {
		cases = append(cases, struct {
			name    string
			mode    fs.FileMode
			wantErr error
		}{fmt.Sprintf("high mode bit %#x cannot hide behind valid permissions", bit), bit | fs.ModePerm, core.ErrFilestoreContract})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root := requireTestRoot(t, directory)
			request := filestore.DirectoryRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, "entry")}, Mode: tc.mode}
			gotErr := request.Validate()
			if !errors.Is(gotErr, tc.wantErr) || errors.Is(gotErr, core.ErrFilestoreActivation) || errors.Is(gotErr, core.ErrFilestoreSource) {
				t.Fatalf("mode %#o validation = %v, want pure %v", tc.mode, gotErr, tc.wantErr)
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != 0 {
				t.Fatalf("validation effects = (%v,%v), want no entries", entries, err)
			}
		})
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
		{name: "valid read owns destination and location without a size declaration", wantValid: true, run: func(_ *testing.T) error {
			return (filestore.ReadRequest{
				Destination: io.Discard, Location: location,
			}).Validate()
		}},
		{name: "valid create write owns every activation boundary", wantValid: true, run: func(t *testing.T) error {
			return (filestore.WriteRequest{
				Source: strings.NewReader("x"), Location: location,
				Temporary: mustRelativePath(t, ".target-stage"), Mode: 0o600,
				Install: filestore.InstallCreate,
			}).Validate()
		}},
		{name: "valid replace write admits absent or existing target policy", wantValid: true, run: func(t *testing.T) error {
			return (filestore.WriteRequest{
				Source: strings.NewReader("x"), Location: location,
				Temporary: mustRelativePath(t, ".target-stage"), Mode: 0o600,
				Install: filestore.InstallReplace,
			}).Validate()
		}},
		{name: "valid target-late stage owns source name and mode", wantValid: true, run: func(t *testing.T) error {
			return (filestore.StageRequest{
				Source:    strings.NewReader("x"),
				Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, ".stage")},
				Mode:      0o600,
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
			return (filestore.ReadRequest{Location: location}).Validate()
		}},
		{name: "write without source", run: func(t *testing.T) error {
			return (filestore.WriteRequest{
				Location: location, Temporary: mustRelativePath(t, ".target-stage"), Mode: 0o600,
				Install: filestore.InstallCreate,
			}).Validate()
		}},
		{name: "write without install mode", run: func(t *testing.T) error {
			return (filestore.WriteRequest{
				Source: strings.NewReader("x"), Location: location,
				Temporary: mustRelativePath(t, ".target-stage"), Mode: 0o600,
			}).Validate()
		}},
		{name: "write without temporary path", run: func(_ *testing.T) error {
			return (filestore.WriteRequest{
				Source: strings.NewReader("x"), Location: location,
				Mode: 0o600, Install: filestore.InstallCreate,
			}).Validate()
		}},
		{name: "stage without root", run: func(t *testing.T) error {
			return (filestore.StageRequest{
				Source:    strings.NewReader("x"),
				Temporary: filestore.Location{Path: mustRelativePath(t, filepath.Join(directory.String(), ".stage"))},
				Mode:      0o600,
			}).Validate()
		}},
		{name: "stage without temporary path", run: func(_ *testing.T) error {
			return (filestore.StageRequest{
				Source:    strings.NewReader("x"),
				Temporary: filestore.Location{Root: root},
				Mode:      0o600,
			}).Validate()
		}},
		{name: "stage without source", run: func(t *testing.T) error {
			return (filestore.StageRequest{
				Temporary: filestore.Location{
					Root: root,
					Path: mustRelativePath(t, filepath.Join(directory.String(), ".stage")),
				},
				Mode: 0o600,
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
					Source:    strings.NewReader("x"),
					Location:  filestore.Location{Root: root, Path: target},
					Temporary: target,
					Mode:      0o600,
					Install:   filestore.InstallCreate,
				}).Validate()
			},
		},
		{
			name:    "write temporary is in a different directory",
			wantErr: core.ErrFilestoreContract,
			run: func(t *testing.T) error {
				return (filestore.WriteRequest{
					Source:    strings.NewReader("x"),
					Location:  filestore.Location{Root: root, Path: target},
					Temporary: mustRelativePath(t, filepath.Join("staging", ".target")),
					Mode:      0o600,
					Install:   filestore.InstallCreate,
				}).Validate()
			},
		},
		{
			name:    "write target cannot be the root entry",
			wantErr: core.ErrFilestoreContract,
			run: func(t *testing.T) error {
				return (filestore.WriteRequest{
					Source:    strings.NewReader("x"),
					Location:  filestore.Location{Root: root, Path: mustRelativePath(t, ".")},
					Temporary: mustRelativePath(t, ".stage"),
					Mode:      0o600,
					Install:   filestore.InstallCreate,
				}).Validate()
			},
		},
		{
			name:    "stage temporary cannot be the root entry",
			wantErr: core.ErrFilestoreContract,
			run: func(t *testing.T) error {
				return (filestore.StageRequest{
					Source:    strings.NewReader("x"),
					Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, ".")},
					Mode:      0o600,
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
	recovery, gotErr := filestore.Write(t.Context(), filestore.WriteRequest{
		Source: strings.NewReader("escape"),
		Location: filestore.Location{
			Root: root,
			Path: mustRelativePath(t, filepath.Join("escape", "target")),
		},
		Temporary: mustRelativePath(t, filepath.Join("escape", ".target-stage")),
		Mode:      0o600,
		Install:   filestore.InstallReplace,
	})
	if !errors.Is(gotErr, core.ErrFilestoreActivation) {
		t.Fatalf(
			"Write() through escaping symlink error = %v, want errors.Is %v",
			gotErr,
			core.ErrFilestoreActivation,
		)
	}
	if recovery != (filestore.CommitRequest{}) {
		t.Fatalf("Write() through escaping symlink recovery = %+v, want zero after determinate cleanup", recovery)
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
