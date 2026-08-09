package process_test

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/temporal"
)

// TestResolveExecutableAdmitsOnlyWhatTheHostWouldRun is the door's whole
// contract on both sides of the runnability boundary. The permission rows are
// the ones that matter: runnability is the host's access answer for this
// process's effective identity, not a mode-bit glance, so a group-only or
// other-only execute bit must refuse for the owning caller while an
// execute-only file with no read bit must resolve, and every special bit is
// probed with and without an execute bit beside it. Link rows walk every
// state a chain can be in. Every fixture lives in its own temporary directory
// and no row consults PATH, the environment, or the working directory, which
// is why this suite runs parallel while Resolve's cannot. Refusals whose
// exact errno the platform owns live in the unix leaf test beside this file.
func TestResolveExecutableAdmitsOnlyWhatTheHostWouldRun(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		build   func(t *testing.T) (path string, wantResolved string)
		name    string
	}{
		{
			name: "an executable regular file resolves to itself",
			build: func(t *testing.T) (string, string) {
				path := resolveExecutablePlant(t, t.TempDir(), 0o700)
				return path, path
			},
		},
		{
			name: "an execute-only file with no read bit still resolves",
			build: func(t *testing.T) (string, string) {
				path := resolveExecutablePlant(t, t.TempDir(), 0o100)
				return path, path
			},
		},
		{
			name: "a fully open mode resolves",
			build: func(t *testing.T) (string, string) {
				path := resolveExecutablePlant(t, t.TempDir(), 0o777)
				return path, path
			},
		},
		{
			name: "a setuid executable resolves",
			build: func(t *testing.T) (string, string) {
				path := resolveExecutablePlant(t, t.TempDir(), 0o755|os.ModeSetuid)
				return path, path
			},
		},
		{
			name: "a setgid executable resolves",
			build: func(t *testing.T) (string, string) {
				path := resolveExecutablePlant(t, t.TempDir(), 0o755|os.ModeSetgid)
				return path, path
			},
		},
		{
			name: "a sticky executable resolves",
			build: func(t *testing.T) (string, string) {
				path := resolveExecutablePlant(t, t.TempDir(), 0o755|os.ModeSticky)
				return path, path
			},
		},
		{
			name: "an empty executable file resolves because runnability is not content",
			build: func(t *testing.T) (string, string) {
				path := filepath.Join(t.TempDir(), "candidate-tool")
				if err := os.WriteFile(path, nil, 0o755); err != nil {
					t.Fatalf("WriteFile(%s) error = %v, want nil", path, err)
				}
				return path, path
			},
		},
		{
			name: "every permission and special bit at once resolves",
			build: func(t *testing.T) (string, string) {
				path := resolveExecutablePlant(t, t.TempDir(), 0o777|os.ModeSetuid|os.ModeSetgid|os.ModeSticky)
				return path, path
			},
		},
		{
			name: "a name at the component byte maximum resolves",
			build: func(t *testing.T) (string, string) {
				path := resolveExecutablePlantNamed(t, t.TempDir(), strings.Repeat("x", 255), 0o700)
				return path, path
			},
		},
		{
			name: "a name containing a space resolves without interpretation",
			build: func(t *testing.T) (string, string) {
				path := resolveExecutablePlantNamed(t, t.TempDir(), "candidate tool", 0o700)
				return path, path
			},
		},
		{
			name: "a name containing a newline resolves without interpretation",
			build: func(t *testing.T) (string, string) {
				path := resolveExecutablePlantNamed(t, t.TempDir(), "candidate\ntool", 0o700)
				return path, path
			},
		},
		{
			name: "a name that looks like a flag resolves without interpretation",
			build: func(t *testing.T) (string, string) {
				path := resolveExecutablePlantNamed(t, t.TempDir(), "-candidate-tool", 0o700)
				return path, path
			},
		},
		{
			name: "a symbolic link to an executable resolves to the link path itself",
			build: func(t *testing.T) (string, string) {
				directory := t.TempDir()
				target := resolveExecutablePlant(t, directory, 0o700)
				link := filepath.Join(directory, "link")
				if err := os.Symlink(target, link); err != nil {
					t.Fatalf("Symlink() error = %v, want nil", err)
				}
				return link, link
			},
		},
		{
			name: "a chain of symbolic links to an executable resolves to the first link path",
			build: func(t *testing.T) (string, string) {
				directory := t.TempDir()
				target := resolveExecutablePlant(t, directory, 0o700)
				middle := filepath.Join(directory, "middle")
				if err := os.Symlink(target, middle); err != nil {
					t.Fatalf("Symlink(middle) error = %v, want nil", err)
				}
				first := filepath.Join(directory, "first")
				if err := os.Symlink(middle, first); err != nil {
					t.Fatalf("Symlink(first) error = %v, want nil", err)
				}
				return first, first
			},
		},
		{
			name: "an executable reached through a symlinked parent directory resolves",
			build: func(t *testing.T) (string, string) {
				directory := t.TempDir()
				real := filepath.Join(directory, "real")
				if err := os.Mkdir(real, 0o700); err != nil {
					t.Fatalf("Mkdir() error = %v, want nil", err)
				}
				resolveExecutablePlant(t, real, 0o700)
				via := filepath.Join(directory, "via")
				if err := os.Symlink(real, via); err != nil {
					t.Fatalf("Symlink(via) error = %v, want nil", err)
				}
				path := filepath.Join(via, "candidate-tool")
				return path, path
			},
		},
		{
			name:    "a regular file with no execute bit refuses with the permission identity",
			wantErr: fs.ErrPermission,
			build: func(t *testing.T) (string, string) {
				return resolveExecutablePlant(t, t.TempDir(), 0o600), ""
			},
		},
		{
			name:    "a group-only execute bit refuses for the owning caller",
			wantErr: fs.ErrPermission,
			build: func(t *testing.T) (string, string) {
				return resolveExecutablePlant(t, t.TempDir(), 0o610), ""
			},
		},
		{
			name:    "an other-only execute bit refuses for the owning caller",
			wantErr: fs.ErrPermission,
			build: func(t *testing.T) (string, string) {
				return resolveExecutablePlant(t, t.TempDir(), 0o601), ""
			},
		},
		{
			name:    "a mode with no permissions at all refuses with the permission identity",
			wantErr: fs.ErrPermission,
			build: func(t *testing.T) (string, string) {
				return resolveExecutablePlant(t, t.TempDir(), 0o000), ""
			},
		},
		{
			name:    "a write-only mode refuses with the permission identity",
			wantErr: fs.ErrPermission,
			build: func(t *testing.T) (string, string) {
				return resolveExecutablePlant(t, t.TempDir(), 0o200), ""
			},
		},
		{
			name:    "a setuid bit without any execute bit refuses with the permission identity",
			wantErr: fs.ErrPermission,
			build: func(t *testing.T) (string, string) {
				return resolveExecutablePlant(t, t.TempDir(), 0o644|os.ModeSetuid), ""
			},
		},
		{
			name:    "an absent path refuses with the nonexistence identity",
			wantErr: fs.ErrNotExist,
			build: func(t *testing.T) (string, string) {
				return filepath.Join(t.TempDir(), "absent-tool"), ""
			},
		},
		{
			name:    "a dangling symbolic link refuses with the nonexistence identity",
			wantErr: fs.ErrNotExist,
			build: func(t *testing.T) (string, string) {
				link := filepath.Join(t.TempDir(), "dangling")
				if err := os.Symlink("nowhere", link); err != nil {
					t.Fatalf("Symlink() error = %v, want nil", err)
				}
				return link, ""
			},
		},
		{
			name:    "a symbolic link chain ending absent refuses with the nonexistence identity",
			wantErr: fs.ErrNotExist,
			build: func(t *testing.T) (string, string) {
				directory := t.TempDir()
				middle := filepath.Join(directory, "middle")
				if err := os.Symlink(filepath.Join(directory, "absent"), middle); err != nil {
					t.Fatalf("Symlink(middle) error = %v, want nil", err)
				}
				first := filepath.Join(directory, "first")
				if err := os.Symlink(middle, first); err != nil {
					t.Fatalf("Symlink(first) error = %v, want nil", err)
				}
				return first, ""
			},
		},
		{
			name:    "an unsearchable parent directory refuses with the permission identity",
			wantErr: fs.ErrPermission,
			build: func(t *testing.T) (string, string) {
				directory := t.TempDir()
				parent := filepath.Join(directory, "sealed")
				if err := os.Mkdir(parent, 0o700); err != nil {
					t.Fatalf("Mkdir() error = %v, want nil", err)
				}
				path := resolveExecutablePlant(t, parent, 0o700)
				if err := os.Chmod(parent, 0o000); err != nil {
					t.Fatalf("Chmod() error = %v, want nil", err)
				}
				t.Cleanup(func() { _ = os.Chmod(parent, 0o700) })
				return path, ""
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			path, wantResolved := tc.build(t)
			got, err := process.ResolveExecutable(t.Context(), absolutePath(t, path))
			if tc.wantErr != nil {
				if !errors.Is(err, core.ErrProcessContract) {
					t.Fatalf("ResolveExecutable(%q) error = %v, want %v", path, err, core.ErrProcessContract)
				}
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("ResolveExecutable(%q) error = %v, want errors.Is(_, %v)", path, err, tc.wantErr)
				}
				if got.String() != "" {
					t.Fatalf("ResolveExecutable(%q) = %q, want the zero path on refusal", path, got.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveExecutable(%q) error = %v, want nil", path, err)
			}
			if got.String() != wantResolved {
				t.Fatalf("ResolveExecutable(%q) = %q, want %q", path, got.String(), wantResolved)
			}
		})
	}
}

// TestResolveExecutableRefusesAnUnsetPathAndACancelledContextBeforeTouchingTheHost
// proves the two gates that must run before the filesystem is consulted at
// all. The unset path is the Go zero value, which no parse produced and which
// would otherwise reach exec.LookPath as the empty string.
func TestResolveExecutableRefusesAnUnsetPathAndACancelledContextBeforeTouchingTheHost(t *testing.T) {
	t.Parallel()

	t.Run("the path is the unset zero value", func(t *testing.T) {
		t.Parallel()

		got, err := process.ResolveExecutable(t.Context(), core.AbsolutePath{})
		if !errors.Is(err, core.ErrProcessContract) {
			t.Fatalf("ResolveExecutable(zero) error = %v, want %v", err, core.ErrProcessContract)
		}
		if got.String() != "" {
			t.Fatalf("ResolveExecutable(zero) = %q, want the zero path on refusal", got.String())
		}
	})

	t.Run("the context is already cancelled", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir()
		path := resolveExecutablePlant(t, directory, 0o700)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		got, err := process.ResolveExecutable(ctx, absolutePath(t, path))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ResolveExecutable() error = %v, want %v", err, context.Canceled)
		}
		if got.String() != "" {
			t.Fatalf("ResolveExecutable() = %q, want the zero path on a cancelled context", got.String())
		}
	})

	t.Run("a cancelled context outranks an unset path", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		got, err := process.ResolveExecutable(ctx, core.AbsolutePath{})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ResolveExecutable() error = %v, want %v before the path gate runs", err, context.Canceled)
		}
		if got.String() != "" {
			t.Fatalf("ResolveExecutable() = %q, want the zero path on a cancelled context", got.String())
		}
	})
}

// TestResolveExecutableProducesACommandRunCanActuallyExecute closes the loop
// the door exists for: the confirmed path goes straight into a real execution
// without the caller reshaping it, exactly as Resolve's answer does.
func TestResolveExecutableProducesACommandRunCanActuallyExecute(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	planted := resolveExecutablePlant(t, directory, 0o700)

	command, err := process.ResolveExecutable(t.Context(), absolutePath(t, planted))
	if err != nil {
		t.Fatalf("ResolveExecutable(%q) error = %v, want nil", planted, err)
	}
	delay, err := temporal.DurationFromNanoseconds(int64(10 * time.Second))
	if err != nil {
		t.Fatalf("DurationFromNanoseconds() error = %v, want nil", err)
	}
	request := process.Request{
		Command:          command,
		WorkingDirectory: absolutePath(t, directory),
		Environment:      process.Environment{Mode: process.EnvironmentModeInherit},
		Streams: process.Streams{
			Stdin:  strings.NewReader(""),
			Stdout: io.Discard,
			Stderr: io.Discard,
		},
		OutputLimit: byteCount(t, 1<<16),
		WaitDelay:   delay,
		Containment: process.Containment{
			Isolation:    process.IsolationDirect,
			CancelSignal: process.CancelSignalKill,
		},
	}
	result, err := process.Run(t.Context(), request)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	code, err := result.ExitCode()
	if err != nil {
		t.Fatalf("ExitCode() error = %v, want nil", err)
	}
	success, err := code.Success()
	if err != nil {
		t.Fatalf("Success() error = %v, want nil", err)
	}
	if !success {
		t.Fatalf("Success() = false, want true for the confirmed %q", command.String())
	}
}

// resolveExecutablePlant writes one regular file wearing the requested mode
// into directory and returns its absolute path. The contents are a real
// script rather than empty bytes so the round-trip proof can execute what the
// door confirmed.
func resolveExecutablePlant(t *testing.T, directory string, mode os.FileMode) string {
	t.Helper()

	return resolveExecutablePlantNamed(t, directory, "candidate-tool", mode)
}

// resolveExecutablePlantNamed plants the candidate under an exact hostile
// name. Special mode bits are applied by a second chmod because file creation
// only carries permission bits.
func resolveExecutablePlantNamed(t *testing.T, directory string, name string, mode os.FileMode) string {
	t.Helper()

	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), mode.Perm()); err != nil {
		t.Fatalf("WriteFile(%s) error = %v, want nil", path, err)
	}
	if mode&os.ModePerm != mode {
		if err := os.Chmod(path, mode); err != nil {
			t.Fatalf("Chmod(%s) error = %v, want nil", path, err)
		}
	}
	return path
}
