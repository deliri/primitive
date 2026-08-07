package process_test

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/temporal"
	"github.com/deliri/primitive/v2026/testserial"
)

// resolvePathEntries installs an exact PATH for one test. Every Resolve case
// mutates the process environment, which is why this file declares the
// process-environment hazard instead of running in parallel: PATH is one
// variable shared by every goroutine in the test binary, and a parallel case
// would answer a question a sibling asked.
func resolvePathEntries(t *testing.T, entries ...string) {
	t.Helper()

	t.Setenv("PATH", strings.Join(entries, string(os.PathListSeparator)))
}

// resolvePlantExecutable writes one executable file into directory and returns
// its absolute path. The contents are a real script rather than empty bytes so
// a case that wants to prove the resolved path is the runnable one can.
func resolvePlantExecutable(t *testing.T, directory, name string) string {
	t.Helper()

	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatalf("WriteFile(%s) error = %v, want nil", name, err)
	}
	return path
}

func resolveComponent(t *testing.T, value string) core.PathComponent {
	t.Helper()

	component, err := core.ParsePathComponent(value)
	if err != nil {
		t.Fatalf("ParsePathComponent(%s) error = %v, want nil", value, err)
	}
	return component
}

// TestResolveFindsTheExecutableOnPathAndAlwaysReturnsItAbsolute is the door's
// whole contract. The relative-PATH-entry case is the one that matters: it is
// the step every consumer wrote by hand and the step the ones that got it
// wrong skipped, because exec.LookPath consults PATH entries as written and
// hands back a relative answer that stops meaning anything the moment the
// working directory moves or a child runs with its own WorkingDirectory.
func TestResolveFindsTheExecutableOnPathAndAlwaysReturnsItAbsolute(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{
		Hazard: core.TestIsolationHazardProcessEnvironment,
		Scope:  core.TestIsolationScopePackageProcess,
	})

	cases := []struct {
		entry func(t *testing.T, directory string) string
		name  string
	}{
		{
			name:  "an absolute PATH entry",
			entry: func(_ *testing.T, directory string) string { return directory },
		},
		{
			name: "a PATH entry with a trailing separator",
			entry: func(_ *testing.T, directory string) string {
				return directory + string(os.PathSeparator)
			},
		},
		{
			name: "an unclean absolute PATH entry",
			entry: func(_ *testing.T, directory string) string {
				return filepath.Join(directory, "unrelated", "..")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			want := resolvePlantExecutable(t, directory, "planted-tool")
			resolvePathEntries(t, tc.entry(t, directory))

			got, err := process.Resolve(t.Context(), resolveComponent(t, "planted-tool"))
			if err != nil {
				t.Fatalf("Resolve() error = %v, want nil", err)
			}
			if !filepath.IsAbs(got.String()) {
				t.Fatalf("Resolve() = %q, want an absolute path", got.String())
			}
			if got.String() != want {
				t.Fatalf("Resolve() = %q, want %q", got.String(), want)
			}
		})
	}
}

// TestResolveRefusesANameFoundThroughARelativePathEntry proves Primitive keeps
// Go's refusal instead of re-enabling what Go removed. A relative PATH entry
// means "whatever is in the directory I happen to be standing in", which for a
// tool that inspects repositories is the repository handing itself the process.
// Go reports exec.ErrDot for it, and that identity must stay reachable so a
// caller can tell a hostile PATH apart from a missing tool.
func TestResolveRefusesANameFoundThroughARelativePathEntry(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{
		Hazard: core.TestIsolationHazardProcessWorkingDirectory,
		Scope:  core.TestIsolationScopePackageProcess,
	})

	cases := []struct {
		name    string
		entries []string
	}{
		{name: "the current directory named as a dot", entries: []string{"."}},
		{name: "the current directory named as an empty entry", entries: []string{"", ""}},
		{name: "a subdirectory named relatively", entries: []string{"bin"}},
		{name: "a relative walk back into the tree", entries: []string{filepath.Join("bin", "..", "bin")}},
		{name: "a relative entry ahead of an absolute one", entries: []string{".", "/usr/bin"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			binDirectory := filepath.Join(directory, "bin")
			if err := os.Mkdir(binDirectory, 0o700); err != nil {
				t.Fatalf("Mkdir() error = %v, want nil", err)
			}
			resolvePlantExecutable(t, binDirectory, "relative-tool")
			resolvePlantExecutable(t, directory, "relative-tool")
			t.Chdir(directory)
			resolvePathEntries(t, tc.entries...)

			got, err := process.Resolve(t.Context(), resolveComponent(t, "relative-tool"))
			if !errors.Is(err, core.ErrProcessContract) {
				t.Fatalf("Resolve() error = %v, want %v", err, core.ErrProcessContract)
			}
			if !errors.Is(err, exec.ErrDot) {
				t.Fatalf("Resolve() error = %v, want it to wrap %v", err, exec.ErrDot)
			}
			if got.String() != "" {
				t.Fatalf("Resolve() = %q, want the zero path on refusal", got.String())
			}
		})
	}
}

// TestResolveTakesTheFirstPathEntryThatHoldsARunnableFile proves the door does
// not invent its own search order and does not accept a name that merely
// exists. A directory or an unreadable regular file wearing the command's name
// must not shadow the real executable further down PATH, because that is
// exactly how an operator's own build tree silently replaces a system tool.
func TestResolveTakesTheFirstPathEntryThatHoldsARunnableFile(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{
		Hazard: core.TestIsolationHazardProcessEnvironment,
		Scope:  core.TestIsolationScopePackageProcess,
	})

	cases := []struct {
		shadow func(t *testing.T, directory string)
		name   string
	}{
		{
			name: "a directory wearing the command name",
			shadow: func(t *testing.T, directory string) {
				if err := os.Mkdir(filepath.Join(directory, "shadowed"), 0o700); err != nil {
					t.Fatalf("Mkdir() error = %v, want nil", err)
				}
			},
		},
		{
			name: "a regular file with no execute bit",
			shadow: func(t *testing.T, directory string) {
				if err := os.WriteFile(filepath.Join(directory, "shadowed"), []byte("x"), 0o600); err != nil {
					t.Fatalf("WriteFile() error = %v, want nil", err)
				}
			},
		},
		{
			name:   "an entry that holds nothing at all",
			shadow: func(*testing.T, string) {},
		},
		{
			name: "an entry that is not a directory",
			shadow: func(t *testing.T, directory string) {
				if err := os.WriteFile(filepath.Join(directory, "notadir"), []byte("x"), 0o600); err != nil {
					t.Fatalf("WriteFile() error = %v, want nil", err)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			first := t.TempDir()
			second := t.TempDir()
			tc.shadow(t, first)
			want := resolvePlantExecutable(t, second, "shadowed")
			resolvePathEntries(t, first, second)

			got, err := process.Resolve(t.Context(), resolveComponent(t, "shadowed"))
			if err != nil {
				t.Fatalf("Resolve() error = %v, want nil", err)
			}
			if got.String() != want {
				t.Fatalf("Resolve() = %q, want %q (the shadow must not win)", got.String(), want)
			}
		})
	}
}

// TestResolveRefusesEveryNameTheHostCannotRunAndKeepsTheNativeIdentity proves
// the failure side carries both contracts a caller needs: the Primitive class
// it can branch on, and exec.ErrNotFound underneath, which is how a product
// probing whether a tool is installed tells "absent" apart from "broken".
func TestResolveRefusesEveryNameTheHostCannotRunAndKeepsTheNativeIdentity(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{
		Hazard: core.TestIsolationHazardProcessEnvironment,
		Scope:  core.TestIsolationScopePackageProcess,
	})

	cases := []struct {
		plant   func(t *testing.T, directory string)
		entries func(directory string) []string
		name    string
		command string
	}{
		{
			name:    "no entry on PATH holds the name",
			command: "absent-tool",
			plant:   func(*testing.T, string) {},
			entries: func(directory string) []string { return []string{directory} },
		},
		{
			name:    "PATH is empty",
			command: "absent-tool",
			plant:   func(*testing.T, string) {},
			entries: func(string) []string { return nil },
		},
		{
			name:    "PATH names a directory that does not exist",
			command: "absent-tool",
			plant:   func(*testing.T, string) {},
			entries: func(directory string) []string {
				return []string{filepath.Join(directory, "missing")}
			},
		},
		{
			name:    "the name is a directory everywhere on PATH",
			command: "dir-tool",
			plant: func(t *testing.T, directory string) {
				if err := os.Mkdir(filepath.Join(directory, "dir-tool"), 0o700); err != nil {
					t.Fatalf("Mkdir() error = %v, want nil", err)
				}
			},
			entries: func(directory string) []string { return []string{directory} },
		},
		{
			name:    "the name has no execute bit anywhere on PATH",
			command: "unrunnable-tool",
			plant: func(t *testing.T, directory string) {
				path := filepath.Join(directory, "unrunnable-tool")
				if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o600); err != nil {
					t.Fatalf("WriteFile() error = %v, want nil", err)
				}
			},
			entries: func(directory string) []string { return []string{directory} },
		},
		{
			name:    "the name is a dangling symbolic link",
			command: "dangling-tool",
			plant: func(t *testing.T, directory string) {
				if err := os.Symlink("nowhere", filepath.Join(directory, "dangling-tool")); err != nil {
					t.Fatalf("Symlink() error = %v, want nil", err)
				}
			},
			entries: func(directory string) []string { return []string{directory} },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			tc.plant(t, directory)
			resolvePathEntries(t, tc.entries(directory)...)

			got, err := process.Resolve(t.Context(), resolveComponent(t, tc.command))
			if !errors.Is(err, core.ErrProcessContract) {
				t.Fatalf("Resolve() error = %v, want %v", err, core.ErrProcessContract)
			}
			if !errors.Is(err, exec.ErrNotFound) && !errors.Is(err, os.ErrPermission) {
				t.Fatalf("Resolve() error = %v, want the native lookup identity to stay reachable", err)
			}
			if got.String() != "" {
				t.Fatalf("Resolve() = %q, want the zero path on refusal", got.String())
			}
		})
	}
}

// TestResolveRefusesAnUnsetNameAndACancelledContextBeforeTouchingPath proves
// the two gates that must run before the host is consulted at all. The unset
// component is the Go zero value, which no parse produced and which would
// otherwise be handed to exec.LookPath as the empty string.
func TestResolveRefusesAnUnsetNameAndACancelledContextBeforeTouchingPath(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{
		Hazard: core.TestIsolationHazardProcessEnvironment,
		Scope:  core.TestIsolationScopePackageProcess,
	})

	directory := t.TempDir()
	resolvePlantExecutable(t, directory, "present-tool")
	resolvePathEntries(t, directory)

	t.Run("the name is the unset zero component", func(t *testing.T) {
		got, err := process.Resolve(t.Context(), core.PathComponent{})
		if !errors.Is(err, core.ErrProcessContract) {
			t.Fatalf("Resolve() error = %v, want %v", err, core.ErrProcessContract)
		}
		if got.String() != "" {
			t.Fatalf("Resolve() = %q, want the zero path on refusal", got.String())
		}
	})

	t.Run("the context is already cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		got, err := process.Resolve(ctx, resolveComponent(t, "present-tool"))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Resolve() error = %v, want %v", err, context.Canceled)
		}
		if got.String() != "" {
			t.Fatalf("Resolve() = %q, want the zero path on a cancelled context", got.String())
		}
	})
}

// TestResolveProducesACommandRunCanActuallyExecute closes the loop the door
// exists for. Resolve's only reason to return core.AbsolutePath is that
// Request.Command demands one, so the proof is that the value goes straight
// into a real execution without a caller reshaping it.
func TestResolveProducesACommandRunCanActuallyExecute(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{
		Hazard: core.TestIsolationHazardProcessEnvironment,
		Scope:  core.TestIsolationScopePackageProcess,
	})

	directory := t.TempDir()
	resolvePathEntries(t, directory, "/bin", "/usr/bin")

	command, err := process.Resolve(t.Context(), resolveComponent(t, "true"))
	if err != nil {
		t.Fatalf("Resolve(true) error = %v, want nil", err)
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
		t.Fatalf("Success() = false, want true for the resolved %q", command.String())
	}
}
